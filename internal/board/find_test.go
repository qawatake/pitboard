package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAncestorsStopAtHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	as := ancestors(filepath.Join(home, "x", "y"))
	if as[len(as)-1] != filepath.Clean(home) {
		t.Errorf("ancestors go above home: %q", as)
	}
}

func TestSessions(t *testing.T) {
	base := t.TempDir()
	for _, s := range []string{"abc123", "abd456"} {
		os.MkdirAll(filepath.Join(base, ".pitboard", s), 0o755)
	}
	if s, err := resolveSession(base, "abc"); err != nil || s != "abc123" {
		t.Errorf("prefix: %q %v", s, err)
	}
	if _, err := resolveSession(base, "ab"); err == nil {
		t.Error("an ambiguous prefix must fail")
	}
	if s, err := resolveSession(base, "new-one"); err != nil || s != "new-one" {
		t.Errorf("new: %q %v", s, err)
	}
	if _, err := resolveSession(base, "../x"); err == nil {
		t.Error("a bad name must fail")
	}
}

// The session id in the wake message is as short as it can be and still
// resolves to the session.
func TestShortSession(t *testing.T) {
	base := t.TempDir()
	for _, s := range []string{"38d12f95-0b22-4d64-88fb-17ed69d428cb", "38d12f95-9999", "abc"} {
		os.MkdirAll(Open(base, s).Dir, 0o755)
	}
	st := Open(base, "38d12f95-0b22-4d64-88fb-17ed69d428cb")
	if got := ShortSession(st); got != "38d12f95-0" {
		t.Errorf("short id %q", got)
	}
	if got := ShortSession(Open(base, "abc")); got != "abc" {
		t.Errorf("short id %q", got)
	}
	os.RemoveAll(Open(base, "38d12f95-9999").Dir)
	if got := ShortSession(st); got != "38d12f95" {
		t.Errorf("alone: %q", got)
	}
	// The short id resolves to the session.
	if s, err := resolveSession(base, ShortSession(st)); err != nil || s != st.Session {
		t.Errorf("resolves to %q %v", s, err)
	}
}
