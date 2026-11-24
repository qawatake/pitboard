package viewer

import (
	"os"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/page/guardtable"
)

// The terminal does what page's guard table says, key by key.
func TestGuardTable(t *testing.T) {
	for _, c := range guardtable.Cases {
		t.Run(c.Name, func(t *testing.T) {
			st := testStore(t)
			os.WriteFile(st.ReportPath(), []byte(guardtable.Report), 0o644)
			a := testApp(t, st)
			pv := a.top()
			a.render()
			pv.cur, pv.follow = c.Line, true
			pv.open[c.Line] = c.Open
			if c.How == guardtable.Settling || c.How == guardtable.Settled || c.How == guardtable.Hid {
				pv.auto = c.Line
			}
			if c.How == guardtable.Hid {
				pv.hidTo = c.Line
			}
			a.render()
			if !pv.curShown {
				t.Fatalf("line %q not on screen", c.Line)
			}
			pv.placed, pv.lock = c.How == guardtable.Placed, c.How == guardtable.Locked
			switch c.How {
			case guardtable.Settling, guardtable.Hid:
				pv.autoAt = clock()
			case guardtable.Settled:
				pv.autoAt = clock().Add(-page.SettleDelay)
			}
			unread := a.b.MarkOf(pv.nodeAt(c.Line)) == page.MarkNew
			r := []rune(c.Key)[0]
			a.handle(key{r: r})
			if c.Twice {
				a.handle(key{r: r})
			}
			op := ""
			if evs := st.Events(); len(evs) > 0 {
				op = evs[len(evs)-1].Op
			}
			if op != c.Op {
				t.Errorf("recorded %q, want %q", op, c.Op)
			}
			switch c.Do {
			case page.DoNothing:
				if a.msg != c.Msg {
					t.Errorf("msg %q, want %q", a.msg, c.Msg)
				}
			case page.DoOpen:
				if !pv.open[c.Line] {
					t.Error("not opened")
				}
			case page.DoComment:
				if a.in == nil {
					t.Error("no editor")
				}
			case page.DoRead:
				if !unread || a.b.MarkOf(pv.nodeAt(c.Line)) != page.MarkNone {
					t.Error("not read")
				}
			}
		})
	}
}
