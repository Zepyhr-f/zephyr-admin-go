package syslogmon

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"zephyr-go/pkg/logmask"
)

// readBlockSize is the chunk we Seek-back when reading the tail of a file.
// 64 KiB is large enough that average log lines (~200B) all fit into a few
// rounds, and small enough to avoid pinning a lot of memory per request.
const readBlockSize = 64 << 10

// readTail returns the last `n` lines of the file at `path` and the line
// number at which the slice starts (1-indexed). It Seeks back from EOF in
// blocks, so very large files are bounded by O(n) bytes plus one final pass
// to count total lines for the start index.
func readTail(path string, n int) (lines []string, startLineNo int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := st.Size()
	if size == 0 {
		return nil, 1, nil
	}

	// Read backwards in chunks until we have n+1 newlines or hit BOF.
	var buf []byte
	pos := size
	for pos > 0 && bytesNewlines(buf) <= n {
		readLen := int64(readBlockSize)
		if pos < readLen {
			readLen = pos
		}
		pos -= readLen
		chunk := make([]byte, readLen)
		if _, err := f.ReadAt(chunk, pos); err != nil && err != io.EOF {
			return nil, 0, err
		}
		buf = append(chunk, buf...)
	}

	// Split the trailing buffer into lines. The very last line may be
	// missing a newline, which is fine.
	all := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	lines = all

	// Compute starting line number = total_lines - len(lines) + 1.
	total, err := totalLineCount(path)
	if err != nil {
		// Best effort; we still return the lines and an approximate index.
		return lines, 1, nil
	}
	if total < len(lines) {
		startLineNo = 1
	} else {
		startLineNo = total - len(lines) + 1
	}
	return lines, startLineNo, nil
}

// bytesNewlines counts '\n' in b. Local helper to keep readTail readable.
func bytesNewlines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}

// totalLineCount streams the file once and counts '\n'. Used only as the
// last step to map a tail slice back to a 1-indexed line range.
func totalLineCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64<<10)
	total := 0
	buf := make([]byte, 64<<10)
	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				total++
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// readFromLine reads `limit` lines starting at the 1-indexed `from` line.
// If `from` is past EOF the result is empty.
func readFromLine(path string, from, limit int) ([]string, error) {
	if from < 1 {
		from = 1
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 4<<20) // up to 4 MiB / line
	out := make([]string, 0, limit)
	cur := 0
	for scanner.Scan() {
		cur++
		if cur < from {
			continue
		}
		out = append(out, scanner.Text())
		if len(out) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scanSearch streams the file, returning at most `limit` lines whose raw
// content contains `needle` and whose parsed ts (when present) falls within
// [since, until]. truncated indicates we stopped before EOF because the
// limit was reached.
func scanSearch(path, svcName, fileName, needle, level string, since, until time.Time, limit int) ([]LogLine, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	out := make([]LogLine, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	lineNo := 0
	truncated := false
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		if !strings.Contains(raw, needle) {
			continue
		}
		ll := ParseLine(svcName, fileName, raw)
		ll.LineNo = lineNo

		if level != "" && !strings.EqualFold(ll.Level, level) {
			continue
		}
		if !since.IsZero() || !until.IsZero() {
			ts := parseAnyTS(ll.TS)
			if !ts.IsZero() {
				if !since.IsZero() && ts.Before(since) {
					continue
				}
				if !until.IsZero() && ts.After(until) {
					continue
				}
			}
		}
		out = append(out, ll)
		if len(out) >= limit {
			// Peek one more byte to know whether we actually truncated.
			truncated = scanner.Scan()
			break
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		return nil, truncated, err
	}
	return out, truncated, nil
}

// parseAnyTS tolerates the various timestamp shapes our log emitters use:
// RFC3339, RFC3339Nano, "2006-01-02 15:04:05" / "...05.000".
func parseAnyTS(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// streamMasked copies path's contents to w line-by-line, applying logmask.Mask
// to each line. Errors mid-stream are swallowed because headers have already
// been written; the operator can spot truncation in the access log.
func streamMasked(w io.Writer, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 64<<10)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			_, _ = io.WriteString(w, logmask.Mask(line))
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}
