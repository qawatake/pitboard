package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// FIX21: esc keeps the draft for its target; opening the editor for the
// same target brings it back. Placing it forgets it.
func TestDraftKeptOnEsc(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.handle(key{r: 'm'})
	for _, r := range "下書き" {
		a.handle(key{r: r})
	}
	a.handle(key{name: "esc"})
	if a.in != nil {
		t.Fatal("esc did not close the editor")
	}
	a.handle(key{r: 'm'})
	if a.in == nil || a.in.ed.String() != "下書き" {
		t.Fatalf("draft not back: %+v", a.in)
	}
	a.handle(key{name: "enter"})
	a.handle(key{r: 'm'})
	if a.in.ed.String() != "" {
		t.Errorf("a placed comment came back as a draft: %q", a.in.ed.String())
	}
}

// Scrolled away while writing: the last line still says the editor is
// open; a key brings the editor back.
func TestWritingAwayLine(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString("<Item id=\"top\">上</Item>\n")
	for i := range 80 {
		b.WriteString("- 行 " + strings.Repeat("x", i%7) + "\n")
	}
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	a.reload(a.top())
	a.t.h = 20
	a.handle(key{r: 'm'})
	if a.in == nil {
		t.Fatal("no editor")
	}
	for range 6 {
		a.handle(key{name: "pgdn"})
	}
	lines, _ := screen(t, a)
	last := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			last = lines[i]
			break
		}
	}
	if !strings.Contains(last, page.MsgWritingAway) || !strings.Contains(last, "esc やめる") {
		t.Errorf("bottom line while scrolled away: %q", last)
	}
	a.handle(key{r: 'j'})
	if a.in.ed.String() != "j" || a.in.scroll != 0 {
		t.Errorf("j did not type and come back: %q %d", a.in.ed.String(), a.in.scroll)
	}
}

// At 40 columns the editor's hints keep the newline key of the keyboard
// protocol in use.
func TestEditorNewlineAt40(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.in = &input{label: "x"}
	a.t.w = 40
	if s := plainLines(a.editorHints()); !strings.Contains(s, "ctrl+j") {
		t.Errorf("ctrl+j terminal at 40: %q", s)
	}
	a.kbd = true
	if s := plainLines(a.editorHints()); !strings.Contains(s, "enter") || !(strings.Contains(s, "shift+enter") || strings.Contains(s, "S-enter")) {
		t.Errorf("kitty keyboard at 40: %q", s)
	}
}
