package web

import (
	"os"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// The AI answers a send with a good version, then writes one that fails
// check: the tab open all along, a new tab and the board list all say
// AI の番.
func TestTurnAnsweredThenBrokenWeb(t *testing.T) {
	f := setup(t)
	t.Setenv("PITBOARD_SEND", "true")
	at := time.Now().Add(-time.Hour)
	write := func(s string) {
		t.Helper()
		at = at.Add(time.Second)
		if s == "" {
			at = time.Now().Add(time.Second)
		}
		if err := os.WriteFile(f.st.ReportPath(), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(f.st.ReportPath(), at, at)
	}
	write(src)
	f.page("open")
	b := page.Open(f.st)
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	at = time.Now()
	check := func(name, want string) {
		t.Helper()
		for _, tab := range []string{"open", "new-" + name} {
			if p := f.page(tab); len(p.Summary) == 0 || p.Summary[0].Text != want {
				t.Errorf("%s, tab %s: %+v", name, tab, p.Summary)
			}
		}
		ix := f.s.index()
		if len(ix.Boards) != 1 || len(ix.Boards[0].Summary) == 0 || ix.Boards[0].Summary[0].Text != want {
			t.Errorf("%s, list: %+v", name, ix.Boards)
		}
	}
	write(src)
	check("answered", page.WordYourTurn)
	write(src + "\n<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n")
	check("broken", page.WordAITurn)
	os.Remove(f.st.ReportPath())
	check("missing", page.WordAITurn)
}
