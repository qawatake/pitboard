package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

// Under an open line, the thoughts the AI is done with fold into one faint
// line after the others, and stay folded: enter only closes the line, and
// opening it again shows them folded. The folded line is no stop.
func TestDoneFoldUnderRow(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a">項目</Item>
<Item id="b">次</Item>`), 0o644)
	d1, _ := st.Append(board.Event{Op: "thought", ID: "a", Text: "済んだ一つ目"})
	st.Append(board.Event{Op: "thought", ID: "a", Text: "まだのもの"})
	st.Settle(d1.N, "直しました")
	a := testApp(t, st)
	pv := a.top()
	pv.cur = "a"
	text := func() string {
		var out []string
		for _, r := range a.rows(pv) {
			out = append(out, reEsc.ReplaceAllString(r.line, ""))
		}
		return strings.Join(out, "\n")
	}
	a.enter(pv)
	s := text()
	if !strings.Contains(s, "✎ 対応済 1 件") || strings.Contains(s, "済んだ一つ目") || !strings.Contains(s, "まだのもの") {
		t.Fatalf("opened, folded:\n%s", s)
	}
	if strings.Index(s, "まだのもの") > strings.Index(s, "対応済 1 件") {
		t.Errorf("the folded line comes last:\n%s", s)
	}
	for _, r := range a.rows(pv) {
		if r.key != "" && strings.Contains(r.line, "対応済 1 件") {
			t.Errorf("the folded line is a stop")
		}
	}

	// A new done thought keeps it folded, with the new count.
	d2, _ := st.Append(board.Event{Op: "thought", ID: "a", Text: "済んだ二つ目"})
	st.Settle(d2.N, "これも")
	a.b.Load()
	if s := text(); !strings.Contains(s, "✎ 対応済 2 件") || strings.Contains(s, "済んだ二つ目") {
		t.Fatalf("a new done one:\n%s", s)
	}

	a.enter(pv)
	if s := text(); strings.Contains(s, "まだのもの") || !pv.isClosed("a") {
		t.Fatalf("closed:\n%s", s)
	}
	a.enter(pv)
	if s := text(); !strings.Contains(s, "✎ 対応済 2 件") {
		t.Fatalf("opened again, folded:\n%s", s)
	}
}

func (pv *view) isClosed(k string) bool { return !pv.open[k] }
