package syslogmon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// withFakeLogRoot points LogRoot at a tmp dir for the duration of the test
// by symlinking /app/logs to a tmp tree if running as root, otherwise it
// builds the layout under a tmp dir and returns adjusted resolveAt.
//
// Because LogRoot is a const, we instead test ResolveLogPath via the public
// surface where the on-disk side is mocked by populating /app/logs IF the
// environment allows it; otherwise the test asserts only the validation
// failures (which do not need the FS) and skips the FS-dependent cases.

func tmpRoot(t *testing.T) string {
	t.Helper()
	// Try to create the real root, falling back to skip if denied.
	if err := os.MkdirAll(filepath.Join(LogRoot, "gateway"), 0o755); err != nil {
		t.Skipf("cannot prepare LogRoot (%v); FS-dependent assertions skipped", err)
	}
	return LogRoot
}

func TestResolveLogPath_ServiceWhitelist(t *testing.T) {
	if _, err := ResolveLogPath("notaservice", "access.log"); !errors.Is(err, ErrBadService) {
		t.Fatalf("expected ErrBadService, got %v", err)
	}
}

func TestResolveLogPath_FileRegex(t *testing.T) {
	bad := []string{
		"../etc/passwd",
		"hello.txt",
		"my log.log",
		"",
	}
	for _, name := range bad {
		if _, err := ResolveLogPath("gateway", name); !errors.Is(err, ErrBadFile) {
			t.Fatalf("name=%q expected ErrBadFile, got %v", name, err)
		}
	}
}

func TestResolveLogPath_PathTraversal(t *testing.T) {
	// Cleaned path of `gateway` + `..\\..\\etc\\passwd` is rejected by the
	// regex first, but we also confirm that a name that *would* pass the
	// regex but contained a separator could not exist — the regex itself
	// forbids `/`. Add a sanity check for a name that escapes through
	// the regex: there shouldn't be any. This test is therefore a
	// guard-rail against a future regex relaxation.
	for _, name := range []string{"a/b.log", "../a.log", "..\\a.log"} {
		if _, err := ResolveLogPath("gateway", name); !errors.Is(err, ErrBadFile) {
			t.Fatalf("name=%q must not pass regex; got %v", name, err)
		}
	}
}

func TestResolveLogPath_Symlink(t *testing.T) {
	root := tmpRoot(t)
	dir := filepath.Join(root, "gateway")
	target := filepath.Join(dir, "real.log")
	link := filepath.Join(dir, "linked.log")
	t.Cleanup(func() { os.Remove(link); os.Remove(target) })

	if err := os.WriteFile(target, []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := ResolveLogPath("gateway", "linked.log"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("expected ErrSymlink, got %v", err)
	}
}

func TestResolveLogPath_OK(t *testing.T) {
	root := tmpRoot(t)
	target := filepath.Join(root, "gateway", "ok.log")
	t.Cleanup(func() { os.Remove(target) })

	if err := os.WriteFile(target, []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ResolveLogPath("gateway", "ok.log")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if got != target {
		t.Fatalf("got=%q want=%q", got, target)
	}
}

func TestResolveLogPath_NotFound(t *testing.T) {
	tmpRoot(t)
	if _, err := ResolveLogPath("gateway", "nosuchfile.log"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
