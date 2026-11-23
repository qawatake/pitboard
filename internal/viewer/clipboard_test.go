package viewer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

// ---- images ----

func pngFile(t *testing.T, dir, name string, size int) string {
	p := filepath.Join(dir, name)
	os.WriteFile(p, append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, size)...), 0o644)
	return p
}

func TestPasteImage(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	prev := clipboardImage
	t.Cleanup(func() { clipboardImage = prev })
	clipboardImage = func(context.Context) ([]byte, error) { return nil, board.ErrNoImage }
	a.handle(key{r: 'm'})
	a.handle(key{r: 'x'})
	a.handle(key{name: "ctrl+v"})
	if a.in.ed.String() != "x" || a.msg != "クリップボードに画像がありません" {
		t.Errorf("no image: %q %q", a.in.ed.String(), a.msg)
	}
	clipboardImage = func(context.Context) ([]byte, error) { return []byte("png"), nil }
	a.handle(key{name: "left"})
	a.handle(key{name: "ctrl+v"})
	if a.in.ed.String() != "[Image #1]x" {
		t.Errorf("pasted: %q", a.in.ed.String())
	}
	clipboardImage = func(context.Context) ([]byte, error) { return make([]byte, board.ImageMax+1), nil }
	a.handle(key{name: "ctrl+v"})
	if !strings.Contains(a.msg, "大きすぎる") {
		t.Errorf("too large: %q", a.msg)
	}

	// A pasted path of an image file (as dropping a file pastes it).
	dir := filepath.Join(t.TempDir(), "my shots")
	os.MkdirAll(dir, 0o755)
	p := pngFile(t, dir, "shot 1.PNG", 10)
	a.in = newInput("", "", "思いつきを書く")
	paste := func(s string) {
		a.handle(key{name: "paste-start"})
		for _, r := range s {
			a.handle(key{r: r})
		}
		a.handle(key{name: "paste-end"})
	}
	paste(strings.ReplaceAll(p, " ", `\ `))
	if got := a.in.ed.String(); got != "[Image #2]" {
		t.Errorf("pasted path: %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(filepath.Join(st.Dir, "images"), "2.png")); len(b) != 18 {
		t.Errorf("copied %d bytes", len(b))
	}
	// Other text, a missing file, or two paths: kept as text.
	for _, s := range []string{" see " + p, "/no/such.png", p + "\n" + p, "'" + filepath.Join(dir, "x.txt") + "'"} {
		a.in = newInput("", "", "")
		paste(s)
		if a.in.ed.String() != s {
			t.Errorf("%q became %q", s, a.in.ed.String())
		}
	}
	a.in = newInput("", "", "")
	paste("'" + p + "'")
	if a.in.ed.String() != "[Image #3]" {
		t.Errorf("quoted path: %q", a.in.ed.String())
	}
	big := pngFile(t, dir, "big.png", board.ImageMax)
	a.in = newInput("", "", "")
	paste(big)
	if a.in.ed.String() != big || !strings.Contains(a.msg, "大きすぎる") {
		t.Errorf("big file: %q %q", a.in.ed.String(), a.msg)
	}
	// Drawn faint.
	if s := withImages("a [Image #3] b", "").String(); !strings.Contains(s, dim+"[") {
		t.Errorf("not faint: %q", s)
	}
}
