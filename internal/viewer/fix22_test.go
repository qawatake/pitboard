package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

const fix22Report = `<Title>t</Title>
<Section title="s">
- one
- two
- three
- four
</Section>
`

// FIX22, FIX23: hiding line after line is x, then x x on each line the
// last x moved the cursor to (the first x there is dropped); u brings back
// the last only.
func TestHideRowsInARow(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(fix22Report), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	for i, want := range []string{"one", "two", "three"} {
		if n := pv.nodeAt(pv.cur); n == nil || !strings.Contains(n.Title(), want) {
			t.Fatalf("press %d: cursor on %q, want %q", i+1, pv.cur, want)
		}
		if i > 0 {
			a.handle(key{r: 'x'})
			if evs := st.Events(); len(evs) != i || a.msg != page.MsgSettlingHide {
				t.Fatalf("press %d: first x on a moved-to line: %q %+v", i+1, a.msg, evs)
			}
		}
		a.handle(key{r: 'x'})
		a.render() // no time passes: the presses come one right after another
		if evs := st.Events(); len(evs) != i+1 || evs[i].Op != "hide" {
			t.Fatalf("press %d: %q %+v", i+1, a.msg, evs)
		}
	}
	s := shown(a)
	if strings.Contains(s, "one") || strings.Contains(s, "two") || strings.Contains(s, "three") || !strings.Contains(s, "four") {
		t.Errorf("after x x x:\n%s", s)
	}
	a.handle(key{r: 'u'})
	a.render()
	if s := shown(a); !strings.Contains(s, "three") || strings.Contains(s, "two") || strings.Contains(s, "one") {
		t.Errorf("u must bring back the last line only:\n%s", s)
	}
	if pv.hiddenCount() != 2 {
		t.Errorf("hidden count %d", pv.hiddenCount())
	}
}
