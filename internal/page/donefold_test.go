package page

import (
	"os"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

// Under a line, the comments the AI is done with come after the others
// and fold into one line: DoneFold says how many.
func TestDoneFold(t *testing.T) {
	st := board.Open(t.TempDir(), "s1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, st, "<Item id=\"a\">項目</Item>\n<Item id=\"b\">他</Item>\n")
	var ns []int
	for _, text := range []string{"一つ目", "二つ目", "三つ目", "四つ目"} {
		e, err := st.Append(board.Event{Op: "thought", ID: "a", Text: text})
		if err != nil {
			t.Fatal(err)
		}
		ns = append(ns, e.N)
	}
	e, _ := st.Append(board.Event{Op: "thought", ID: "b", Text: "まだ"})
	if err := st.SettleAll([]board.Settlement{{N: ns[0], State: "done", Text: "済"}, {N: ns[2], State: "done", Text: "済"}, {N: ns[3], State: "hold", Text: "後で"}}); err != nil {
		t.Fatal(err)
	}
	p := Open(st).Page(false)
	a := find(p.Things, "a")
	var got []string
	for _, c := range a.Comments {
		got = append(got, c.Text)
	}
	if want := []string{"二つ目", "四つ目", "一つ目", "三つ目"}; len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("order %v, want %v", got, want)
	}
	if a.DoneFold != "対応済 2 件" {
		t.Errorf("DoneFold %q", a.DoneFold)
	}
	if b := find(p.Things, "b"); b.DoneFold != "" || len(b.Comments) != 1 {
		t.Errorf("b: %q %d (%d)", b.DoneFold, len(b.Comments), e.N)
	}
}

// Enter: closed opens (folded), folded shows them, shown closes (and
// folds again); a line with nothing done toggles.
func TestEnterStep(t *testing.T) {
	for _, c := range []struct{ open, shown, done, wantOpen, wantShown bool }{
		{false, false, true, true, false},
		{true, false, true, true, true},
		{true, true, true, false, false},
		{false, false, false, true, false},
		{true, false, false, false, false},
	} {
		if o, s := EnterStep(c.open, c.shown, c.done); o != c.wantOpen || s != c.wantShown {
			t.Errorf("%+v: got %v %v", c, o, s)
		}
	}
}

// Open with its done comments folded, enter's hint is 開く (it shows
// them); shown, 閉じる.
func TestDoneFoldHint(t *testing.T) {
	b := open(t, "<Item id=\"a\">項目\n  中身\n</Item>\n")
	n := b.Report.Nodes[0]
	enter := func(tc Touch) string {
		for _, h := range b.Offer(n, tc).Hints {
			if h.Key == "enter" {
				return h.Word
			}
		}
		return ""
	}
	if got := enter(Touch{Open: true, Folded: true}); got != "開く" {
		t.Errorf("folded: %q", got)
	}
	if got := enter(Touch{Open: true}); got != "閉じる" {
		t.Errorf("shown: %q", got)
	}
}
