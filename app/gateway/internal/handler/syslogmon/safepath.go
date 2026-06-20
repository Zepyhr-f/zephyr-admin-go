// Package syslogmon serves the sysadmin "服务日志监控" feature: list / read /
// search / tail (SSE) / download a fixed white-listed set of log files under
// /app/logs/<svc>/. All handlers are read-only and must be mounted behind
// gateway JWT (perms gating is deferred to P3).
package syslogmon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// LogRoot is the on-disk root that holds every service's log dir. It is a
// constant on purpose — the mount point in docker-compose is fixed and
// changing it requires a deployment change anyway.
const LogRoot = "/app/logs"

// Service is one entry in the fixed service white-list. Order is the order
// the frontend tree should display.
type Service struct {
	Name    string `json:"service"`
	Display string `json:"display"`
}

// Services is the immutable white-list. Any svc parameter that is not a key
// of this map is rejected with 400.
var Services = []Service{
	{Name: "gateway", Display: "Gateway"},
	{Name: "auth", Display: "Auth RPC"},
	{Name: "identity", Display: "Identity RPC"},
	{Name: "ai", Display: "Zephyr AI"},
	{Name: "nginx", Display: "Nginx"},
}

// fileNameRe matches `*.log`, `*.log.<n>` and `*.log.<n>.gz`. Mirrors the
// design doc verbatim.
var fileNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+\.log(\.\d+(\.gz)?)?$`)

// ErrBadService / ErrBadFile / ErrNotFound let handlers map to HTTP codes.
var (
	ErrBadService = errors.New("syslogmon: service not in white-list")
	ErrBadFile    = errors.New("syslogmon: file name does not match policy")
	ErrEscape     = errors.New("syslogmon: resolved path escapes log root")
	ErrSymlink    = errors.New("syslogmon: target is a symlink")
	ErrNotFound   = errors.New("syslogmon: log file not found")
	ErrNotRegular = errors.New("syslogmon: target is not a regular file")
)

// IsValidService reports whether svc is allowed.
func IsValidService(svc string) bool {
	for _, s := range Services {
		if s.Name == svc {
			return true
		}
	}
	return false
}

// IsValidFileName reports whether file matches the allowed regex.
func IsValidFileName(file string) bool {
	return fileNameRe.MatchString(file)
}

// ResolveLogPath validates `svc` against the white-list and `file` against
// the regex, then computes the absolute path under LogRoot and verifies
// it points at a regular file (not a symlink, not a directory). The caller
// should map ErrBadService / ErrBadFile / ErrEscape / ErrSymlink to HTTP
// 400, and ErrNotFound / ErrNotRegular to HTTP 404.
func ResolveLogPath(svc, file string) (string, error) {
	if !IsValidService(svc) {
		return "", ErrBadService
	}
	if !IsValidFileName(file) {
		return "", ErrBadFile
	}

	want := filepath.Join(LogRoot, svc) + string(filepath.Separator)
	full := filepath.Clean(filepath.Join(LogRoot, svc, file))
	// Defence in depth: even with the regex above forbidding `/` and `..`, we
	// still confirm the cleaned path is rooted at /app/logs/<svc>/.
	if len(full) <= len(want) || full[:len(want)] != want {
		return "", fmt.Errorf("%w: %s", ErrEscape, full)
	}

	st, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return "", ErrSymlink
	}
	if !st.Mode().IsRegular() {
		return "", ErrNotRegular
	}
	return full, nil
}
