package syslogmon

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/pkg/core/response"
	"zephyr-go/pkg/core/xerr"
)

// pathVar reads `name` out of go-zero's path-variable context map. Returns
// "" if missing. Centralised so the handlers stay readable.
func pathVar(r *http.Request, name string) string {
	if vars := pathvar.Vars(r); vars != nil {
		return vars[name]
	}
	return ""
}

// Limits are intentionally const — see design doc §5 for the rationale.
const (
	maxReadLines     = 5000
	defaultReadLines = 200
	maxFileBytes     = 200 << 20 // 200 MiB hard cap for search / download
	lineCountSkipAt  = 50 << 20  // > 50 MiB → don't bother counting newlines
)

// fileInfoOut is the per-file response of /services/:svc/files.
type fileInfoOut struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	MTime     string `json:"mtime"`
	LineCount int64  `json:"line_count"`
}

// resolveOrError centralises the (svc, file) → abs-path validation and
// translates the typed errors into HTTP responses. Returns "" + true if
// the response has already been written.
func resolveOrError(w http.ResponseWriter, svcName, file string) (string, bool) {
	full, err := ResolveLogPath(svcName, file)
	if err == nil {
		return full, false
	}
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotRegular):
		response.Error(w, xerr.NewErrCodeMsg(http.StatusNotFound, err.Error()))
	default: // bad service / bad file / escape / symlink
		response.Error(w, xerr.NewErrCodeMsg(http.StatusBadRequest, err.Error()))
	}
	return "", true
}

// ListServicesHandler — GET /services. The white-list is a const so the
// implementation is a plain marshal.
func ListServicesHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, Services)
	}
}

// ListFilesHandler — GET /services/:svc/files.
func ListFilesHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcName := pathVar(r, "svc")
		if !IsValidService(svcName) {
			response.Error(w, xerr.NewErrCodeMsg(http.StatusBadRequest, ErrBadService.Error()))
			return
		}
		dir := filepath.Join(LogRoot, svcName)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				response.Success(w, []fileInfoOut{})
				return
			}
			response.Error(w, err)
			return
		}

		out := make([]fileInfoOut, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !IsValidFileName(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			// Refuse symlinks — handler must never follow them.
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			out = append(out, fileInfoOut{
				Name:      e.Name(),
				Size:      info.Size(),
				MTime:     info.ModTime().Format(time.RFC3339),
				LineCount: countLines(filepath.Join(dir, e.Name()), info.Size()),
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		response.Success(w, out)
	}
}

// countLines returns -1 when the file is bigger than lineCountSkipAt; for
// smaller files it reads the whole content and counts '\n'. The doc
// explicitly asks for `bytes.Count`.
func countLines(path string, size int64) int64 {
	if size > lineCountSkipAt {
		return -1
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	return int64(bytes.Count(b, []byte{'\n'}))
}

// ReadFileHandler — GET /services/:svc/files/:file. Either tail or
// from_line+limit. Default = last 200 lines.
func ReadFileHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcName := pathVar(r, "svc")
		fileName := pathVar(r, "file")
		full, halted := resolveOrError(w, svcName, fileName)
		if halted {
			return
		}

		q := r.URL.Query()
		fromLine, _ := strconv.Atoi(q.Get("from_line"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		tail, _ := strconv.Atoi(q.Get("tail"))

		if fromLine > 0 {
			if limit <= 0 || limit > maxReadLines {
				limit = defaultReadLines
			}
			lines, err := readFromLine(full, fromLine, limit)
			if err != nil {
				response.Error(w, err)
				return
			}
			response.Success(w, parseAll(svcName, fileName, fromLine, lines))
			return
		}

		// tail mode (default 200, max 5000).
		if tail <= 0 {
			tail = defaultReadLines
		}
		if tail > maxReadLines {
			tail = maxReadLines
		}
		lines, startLineNo, err := readTail(full, tail)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, parseAll(svcName, fileName, startLineNo, lines))
	}
}

func parseAll(svcName, fileName string, startLineNo int, lines []string) []LogLine {
	out := make([]LogLine, 0, len(lines))
	for i, raw := range lines {
		ll := ParseLine(svcName, fileName, raw)
		ll.LineNo = startLineNo + i
		out = append(out, ll)
	}
	return out
}

// SearchHandler — GET /services/:svc/files/:file/search.
//
// A streaming line scanner with substring matching. Regex was rejected to
// avoid pathological input from a UI text box. Callers pass a level filter
// and optional RFC3339 since/until window; lines whose parsed ts falls
// outside the window are dropped.
type searchOut struct {
	Lines     []LogLine `json:"lines"`
	Truncated bool      `json:"truncated"`
}

func SearchHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcName := pathVar(r, "svc")
		fileName := pathVar(r, "file")
		full, halted := resolveOrError(w, svcName, fileName)
		if halted {
			return
		}

		q := r.URL.Query()
		needle := q.Get("q")
		if needle == "" {
			response.Error(w, xerr.NewErrCodeMsg(http.StatusBadRequest, "q is required"))
			return
		}
		level := q.Get("level")
		since := parseRFC(q.Get("since"))
		until := parseRFC(q.Get("until"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 || limit > maxReadLines {
			limit = defaultReadLines
		}

		st, err := os.Stat(full)
		if err != nil {
			response.Error(w, err)
			return
		}
		if st.Size() > maxFileBytes {
			response.Error(w, xerr.NewErrCodeMsg(http.StatusRequestEntityTooLarge,
				"file too large to search; please download instead"))
			return
		}

		hits, truncated, err := scanSearch(full, svcName, fileName, needle, level, since, until, limit)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, searchOut{Lines: hits, Truncated: truncated})
	}
}

func parseRFC(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// DownloadHandler — GET /services/:svc/files/:file/download.
//
// Streams the file through logmask.Mask line-by-line so we never buffer the
// whole file in memory.
func DownloadHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcName := pathVar(r, "svc")
		fileName := pathVar(r, "file")
		full, halted := resolveOrError(w, svcName, fileName)
		if halted {
			return
		}
		st, err := os.Stat(full)
		if err != nil {
			response.Error(w, err)
			return
		}
		if st.Size() > maxFileBytes {
			response.Error(w, xerr.NewErrCodeMsg(http.StatusRequestEntityTooLarge,
				"file too large to download"))
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition",
			`attachment; filename="`+svcName+`-`+fileName+`"`)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		streamMasked(w, full)
	}
}
