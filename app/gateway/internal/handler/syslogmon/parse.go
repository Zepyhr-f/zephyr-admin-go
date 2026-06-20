package syslogmon

import (
	"encoding/json"
	"regexp"
	"strings"
)

// LogLine is the unified shape returned per-line for read / search responses.
// Fields not derivable from a given format stay at their zero value so the
// frontend can render them conditionally.
type LogLine struct {
	TS     string `json:"ts,omitempty"`
	Level  string `json:"level,omitempty"`
	Logger string `json:"logger,omitempty"`
	Svc    string `json:"service,omitempty"`
	File   string `json:"file,omitempty"`
	LineNo int    `json:"line_no"`
	Trace  string `json:"trace,omitempty"`
	Span   string `json:"span,omitempty"`
	Msg    string `json:"msg,omitempty"`
	Raw    string `json:"raw"`
}

// goZeroLine matches the in-the-wild go-zero JSON log envelope. We only
// pull the few fields the frontend renders; everything else falls into
// `Raw` for the user to expand.
type goZeroLine struct {
	Timestamp string `json:"@timestamp"`
	Level     string `json:"level"`
	Logger    string `json:"logger"`
	Trace     string `json:"trace"`
	Span      string `json:"span"`
	Content   string `json:"content"`
}

var (
	// fastapiLineRe matches loguru / standard Python "ts | LEVEL | logger | msg".
	// Both sides of every separator can have any amount of whitespace.
	fastapiLineRe = regexp.MustCompile(
		`^(?P<ts>\S+ \S+)\s*\|\s*(?P<level>\S+)\s*\|\s*(?P<logger>\S+)\s*\|\s*(?P<msg>.*)$`,
	)

	// nginxCombinedRe matches the canonical nginx combined access log line:
	//   ip - user [ts] "method path proto" status size "ref" "ua"
	nginxCombinedRe = regexp.MustCompile(
		`^(?P<ip>\S+) \S+ \S+ \[(?P<ts>[^\]]+)\] "(?P<req>[^"]*)" (?P<status>\d+) (?P<size>\S+) "(?P<ref>[^"]*)" "(?P<ua>[^"]*)"`,
	)
)

// ParseLine inspects `raw` and returns the best-effort LogLine. svc and file
// are echoed back verbatim — handlers know the context so the parser doesn't.
// Line numbering is the caller's responsibility (LineNo is set by the read
// handler that's tracking position).
func ParseLine(svc, file, raw string) LogLine {
	out := LogLine{Svc: svc, File: file, Raw: raw}

	trimmed := strings.TrimRight(raw, "\r\n")

	// 1. JSON path — go-zero emits a top-level object on every line.
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var gz goZeroLine
		if err := json.Unmarshal([]byte(trimmed), &gz); err == nil && (gz.Timestamp != "" || gz.Content != "") {
			out.TS = gz.Timestamp
			out.Level = strings.ToLower(gz.Level)
			out.Logger = gz.Logger
			out.Trace = gz.Trace
			out.Span = gz.Span
			out.Msg = gz.Content
			if out.Level == "" {
				out.Level = "info"
			}
			return out
		}
	}

	// 2. FastAPI / loguru text path.
	if m := fastapiLineRe.FindStringSubmatch(trimmed); m != nil {
		out.TS = m[1]
		out.Level = strings.ToLower(m[2])
		out.Logger = m[3]
		out.Msg = m[4]
		return out
	}

	// 3. Nginx combined path. Level is inferred from the file name —
	//    `access.log` is info, anything containing "error" is error.
	if m := nginxCombinedRe.FindStringSubmatch(trimmed); m != nil {
		out.TS = m[2]
		out.Level = "info"
		if strings.Contains(file, "error") {
			out.Level = "error"
		}
		out.Logger = "nginx"
		out.Msg = m[1] + ` "` + m[3] + `" ` + m[4]
		return out
	}

	// 4. Fallback — keep the raw and label as unknown so the UI can still
	//    render and color it.
	out.Level = "unknown"
	out.Msg = trimmed
	return out
}
