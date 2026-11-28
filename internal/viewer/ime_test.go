package viewer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// imeCount sets $PITBOARD_IME_OFF (to a command that adds a line to a file,
// or unset) and returns how many times it has run, waiting a little for
// runs in flight.
func imeCount(t *testing.T, set bool) func() int {
	t.Helper()
	out := filepath.Join(t.TempDir(), "ime")
	ran := make(chan struct{}, 16)
	prev := imeRan
	imeRan = func() { ran <- struct{}{} }
	t.Cleanup(func() { imeRan = prev })
	if set {
		t.Setenv("PITBOARD_IME_OFF", "echo off >> "+out)
	} else {
		t.Setenv("PITBOARD_IME_OFF", "")
	}
	runs := 0
	return func() int {
		t.Helper()
	drain:
		for {
			select {
			case <-ran:
				runs++
			case <-time.After(300 * time.Millisecond):
				break drain
			}
		}
		b, _ := os.ReadFile(out)
		if n := strings.Count(string(b), "off\n"); n != runs {
			t.Fatalf("the command ended %d times, the file says %d", runs, n)
		}
		return runs
	}
}

// writeComment opens the editor with m and types s.
func writeComment(a *app, s string) {
	a.draw()
	a.handle(key{r: 'm'})
	for _, r := range s {
		a.handle(key{r: r})
	}
}

func TestIMEOffWhenEditorCloses(t *testing.T) {
	count := imeCount(t, true)
	a := testApp(t, testStore(t))
	writeComment(a, "あいう")
	if n := count(); n != 0 {
		t.Fatalf("while writing: ran %d times", n)
	}
	a.handle(key{name: "enter"}) // placed
	if a.in != nil {
		t.Fatal("enter did not close the editor")
	}
	if n := count(); n != 1 {
		t.Fatalf("enter: ran %d times, want 1", n)
	}
	writeComment(a, "えお")
	a.handle(key{name: "esc"}) // given up (kept as a draft)
	if n := count(); n != 2 {
		t.Fatalf("esc: ran %d times, want 2", n)
	}
	a.handle(key{r: 'j'}) // a key of the viewer: nothing more
	if n := count(); n != 2 {
		t.Fatalf("a key of the viewer: ran %d times, want 2", n)
	}
}

func TestIMEOffBackFromEditor(t *testing.T) {
	prev := runExternal
	runExternal = func(_ *term, c *exec.Cmd) error { return c.Run() }
	t.Cleanup(func() { runExternal = prev })
	for _, tc := range []struct{ name, script string }{
		{"written", `printf '書いた\n' > "$1"`},
		{"left empty", `: > "$1"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := imeCount(t, true)
			ed := filepath.Join(t.TempDir(), "editor.sh")
			if err := os.WriteFile(ed, []byte("#!/bin/sh\n"+tc.script+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("VISUAL", ed)
			a := testApp(t, testStore(t))
			writeComment(a, "あ")
			a.handle(key{name: "ctrl+g"})
			if a.in != nil {
				t.Fatalf("back from the editor: still open (%q)", a.msg)
			}
			if n := count(); n != 1 {
				t.Fatalf("ran %d times, want 1", n)
			}
		})
	}
}

func TestIMEOffUnset(t *testing.T) {
	count := imeCount(t, false)
	a := testApp(t, testStore(t))
	writeComment(a, "あ")
	a.handle(key{name: "enter"})
	writeComment(a, "い")
	a.handle(key{name: "esc"})
	if n := count(); n != 0 {
		t.Fatalf("unset: ran %d times", n)
	}
}

func TestIMEOffFailureIsQuiet(t *testing.T) {
	ran := make(chan struct{}, 1)
	prev := imeRan
	imeRan = func() { ran <- struct{}{} }
	t.Cleanup(func() { imeRan = prev })
	t.Setenv("PITBOARD_IME_OFF", "echo nope >&2; exit 3")
	a := testApp(t, testStore(t))
	writeComment(a, "あ")
	a.handle(key{name: "enter"})
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("the command did not end")
	}
	if a.msg != "" {
		t.Errorf("a failing command: %q", a.msg)
	}
}
