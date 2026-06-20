package syslogmon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/zeromicro/go-zero/core/logx"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/pkg/core/response"
	"zephyr-go/pkg/core/xerr"
)

// sseRateLimit caps how fast we forward lines on a single SSE connection.
// Anything above this is dropped with a single warn log per second so we
// don't spam the gateway log. The doc explicitly asks for 100/s.
const sseRateLimit = 100

// TailHandler — GET /services/:svc/files/:file/tail. Streams new lines
// over Server-Sent Events. Uses fsnotify on the parent dir so rotations
// (rename / remove / create) are visible.
func TailHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcName := pathVar(r, "svc")
		fileName := pathVar(r, "file")
		full, halted := resolveOrError(w, svcName, fileName)
		if halted {
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			response.Error(w, xerr.NewErrCodeMsg(http.StatusInternalServerError,
				"streaming not supported by upstream"))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		streamTail(r, w, flusher, full, svcName, fileName)
	}
}

// streamTail does the actual fsnotify/loop work. Split out for readability;
// the handler above is only HTTP plumbing.
func streamTail(r *http.Request, w io.Writer, flusher http.Flusher, full, svcName, fileName string) {
	from := r.URL.Query().Get("from")

	f, err := os.Open(full)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
		return
	}
	defer f.Close()

	// Initial seek according to `from`.
	switch from {
	case "head":
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
			return
		}
	case "", "now":
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
			return
		}
	default:
		off, perr := strconv.ParseInt(from, 10, 64)
		if perr != nil {
			writeSSE(w, flusher, "error", map[string]string{"reason": "bad from"})
			return
		}
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
			return
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
		return
	}
	defer watcher.Close()
	if err := watcher.Add(full); err != nil {
		writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// fsnotify is unreliable on docker bind mounts (inotify events from a
	// different mount namespace are sometimes silently dropped). Combine it
	// with a 500ms polling tick so we always catch new bytes appended to
	// the file even when no fs event arrives.
	poll := time.NewTicker(500 * time.Millisecond)
	defer poll.Stop()

	limiter := newRateLimiter(sseRateLimit)
	defer limiter.close()

	reader := bufio.NewReaderSize(f, 64<<10)

	// Emit a hello frame immediately so the client knows the stream is live
	// even when no log line has arrived yet. This also smoke-tests that the
	// flusher chain (handler → go-zero rest → http server → nginx) is not
	// silently buffering us.
	writeSSE(w, flusher, "hello", map[string]string{"svc": svcName, "file": fileName, "from": from})

	// Drain whatever is already past the seek point on first wake-up.
	drainAndPush(w, flusher, reader, f, svcName, fileName, limiter)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			// Poll-based fallback for environments where inotify is muted.
			drainAndPush(w, flusher, reader, f, svcName, fileName, limiter)
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			switch {
			case ev.Op&fsnotify.Write != 0:
				drainAndPush(w, flusher, reader, f, svcName, fileName, limiter)
			case ev.Op&(fsnotify.Rename|fsnotify.Remove) != 0:
				// File rotated — try to reopen.
				newF, err := os.Open(full)
				if err != nil {
					writeSSE(w, flusher, "error", map[string]string{"reason": "file removed"})
					return
				}
				f.Close()
				f = newF
				reader = bufio.NewReaderSize(f, 64<<10)
				_ = watcher.Remove(full)
				_ = watcher.Add(full)
				writeSSE(w, flusher, "rotate", map[string]string{
					"reason": "file rotated, reopened",
				})
				drainAndPush(w, flusher, reader, f, svcName, fileName, limiter)
			case ev.Op&fsnotify.Create != 0:
				// New file under same name (e.g. logrotate's create).
				newF, err := os.Open(full)
				if err == nil {
					f.Close()
					f = newF
					reader = bufio.NewReaderSize(f, 64<<10)
					writeSSE(w, flusher, "rotate", map[string]string{
						"reason": "file recreated",
					})
				}
			}
		case err := <-watcher.Errors:
			if err != nil {
				writeSSE(w, flusher, "error", map[string]string{"reason": err.Error()})
				return
			}
		}
	}
}

// drainAndPush reads every newly-available complete line from f and forwards
// it as `event: line` SSE frames, subject to rate-limiting.
//
// We read directly from the file with a fresh bufio.Scanner each call. This
// is more robust than holding a long-lived bufio.Reader because:
//   - bufio.Reader caches EOF state — once it returns EOF, subsequent reads
//     keep returning EOF even after new bytes are appended.
//   - On bind mounts inside docker, fsnotify events from another mount
//     namespace may be silently dropped, so we cannot rely on Read returning
//     fresh data only when an event arrives.
//
// A short-lived Scanner reads from the current file offset to the live EOF,
// emits each newly-completed line, then returns. Anything past the last
// newline (a partial line written but not yet flushed by the producer) is
// rolled back via Seek so the next call picks it up whole.
func drainAndPush(w io.Writer, flusher http.Flusher, _ *bufio.Reader, f *os.File, svcName, fileName string, limiter *rateLimiter) {
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	st, err := f.Stat()
	if err != nil {
		return
	}
	if st.Size() == pos {
		return // no new bytes
	}
	if st.Size() < pos {
		// Truncation/rotation — reset to start of new file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return
		}
		pos = 0
	}

	// Read everything new in one shot. Lines are usually small; if the producer
	// dumps megabytes between polls we still cope, just in one buffer.
	delta := st.Size() - pos
	buf := make([]byte, delta)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return
	}
	buf = buf[:n]

	// Find last newline; bytes after it are a partial line — rewind so we
	// pick it up on the next call.
	lastNL := -1
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			lastNL = i
			break
		}
	}
	if lastNL < 0 {
		// No complete line yet; rewind fully.
		_, _ = f.Seek(pos, io.SeekStart)
		return
	}
	if lastNL+1 < len(buf) {
		// Rewind past the partial trailing line.
		_, _ = f.Seek(pos+int64(lastNL)+1, io.SeekStart)
		buf = buf[:lastNL+1]
	}

	// Emit each complete line.
	start := 0
	for i := 0; i < len(buf); i++ {
		if buf[i] != '\n' {
			continue
		}
		line := string(buf[start : i+1])
		start = i + 1
		if !limiter.allow() {
			continue
		}
		ll := ParseLine(svcName, fileName, line)
		writeSSE(w, flusher, "line", ll)
	}
}

// writeSSE serialises payload as JSON and emits one SSE frame.
func writeSSE(w io.Writer, flusher http.Flusher, event string, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	flusher.Flush()
}

// rateLimiter is a tiny token-bucket sized for `perSec` events per second.
// We refill once a second on a goroutine; allow() returns false (and warns
// at most once per second) when the bucket is empty.
type rateLimiter struct {
	mu        sync.Mutex
	tokens    int
	perSec    int
	stop      chan struct{}
	wasWarned bool
}

func newRateLimiter(perSec int) *rateLimiter {
	rl := &rateLimiter{tokens: perSec, perSec: perSec, stop: make(chan struct{})}
	go rl.refill()
	return rl
}

func (r *rateLimiter) refill() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			r.mu.Lock()
			r.tokens = r.perSec
			if r.wasWarned {
				logx.Errorf("syslogmon: SSE rate limit hit; some lines dropped")
				r.wasWarned = false
			}
			r.mu.Unlock()
		}
	}
}

func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens <= 0 {
		r.wasWarned = true
		return false
	}
	r.tokens--
	return true
}

func (r *rateLimiter) close() { close(r.stop) }
