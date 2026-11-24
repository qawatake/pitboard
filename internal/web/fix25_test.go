package web

import (
	"os"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// The turn is in the Summary of the page and of the board list, also when
// the report is missing or empty.
func TestTurnWithoutReportWeb(t *testing.T) {
	f := setup(t)
	for _, c := range []struct {
		name string
		do   func()
		turn string
	}{
		{"empty", func() { os.WriteFile(f.st.ReportPath(), nil, 0o644) }, page.WordYourTurn},
		{"missing", func() { os.Remove(f.st.ReportPath()) }, page.WordAITurn},
	} {
		c.do()
		p := f.page("tab1")
		if p.Message == "" || len(p.Summary) == 0 || p.Summary[0].Text != c.turn {
			t.Errorf("%s page: message %q summary %+v", c.name, p.Message, p.Summary)
		}
		ix := f.s.index()
		if len(ix.Boards) != 1 || len(ix.Boards[0].Summary) == 0 || ix.Boards[0].Summary[0].Text != c.turn {
			t.Errorf("%s list: %+v", c.name, ix.Boards)
		}
	}
}

// A late x on a line already hidden says so, by the person or the page,
// and pressing again says the same.
func TestLateHideOnHiddenWeb(t *testing.T) {
	f := setupWith(t, fix22Src)
	f.page("tab1")
	tok := f.thing("a").Token
	x := func(cur Cur) Result {
		return f.act("tab1", Act{Action: "key", Key: "x", Target: "a", Seen: tok, Cur: cur})
	}
	if r := x(Cur{How: "person", Sel: tok}); !r.OK || !r.Hidden {
		t.Fatalf("x: %+v", r)
	}
	for _, cur := range []Cur{{How: "person", Sel: tok}, {How: "app", Sel: tok}, {How: "app", Sel: tok}} {
		if r := x(cur); r.OK || r.Changed || r.Message != page.MsgAlreadyHidden {
			t.Errorf("late x (%s): %+v", cur.How, r)
		}
	}
	if got := f.ops(); len(got) != 1 {
		t.Errorf("ops %v", got)
	}
}
