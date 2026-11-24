package board

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/report"
)

func writeReport(t *testing.T, st *Store, src string) {
	t.Helper()
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A row the person has not answered (look, an Ask), removed, is
// warned of until said once by another command; one they answered, hid
// or read is not.
func TestRemovedUnanswered(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, `<Item id="a-flat" look="y">> storybook がいいとは言っていない、フラットな一覧はメンテナブルか</Item>
<Ask id="ok">答えた問い
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
<Item id="hid" look="y">隠した行</Item>
<Item id="plain">ただの項目</Item>
`)
	if w := st.Removed(); len(w) != 0 {
		t.Fatalf("nothing removed yet: %q", w)
	}
	st.Append(Event{Op: "react", ID: "ok", Value: "a", Option: "A"})
	nodes, _, _ := report.Parse("<Item id=\"hid\" look=\"y\">隠した行</Item>\n")
	st.Append(Event{Op: "hide", Key: report.HideKey(nodes[0])})
	writeReport(t, st, "<Item id=\"other\">別の行</Item>\n")
	w := st.Removed()
	want := `warning: row "a-flat" (> storybook がいいとは言っていない…) was removed but the person has not hidden or answered it`
	if len(w) < 1 || w[0] != want {
		t.Fatalf("warnings %q", w)
	}
	for _, x := range w {
		if strings.Contains(x, `"ok"`) || strings.Contains(x, `"plain"`) {
			t.Errorf("warned of an answered or plain row: %q", x)
		}
	}
	// 'check' says it until another command has said it once.
	if w2 := st.Removed(); len(w2) != len(w) {
		t.Errorf("second time %q", w2)
	}
	if note := st.ProblemNote(); note != strings.Join(w, "\n") {
		t.Errorf("note %q", note)
	}
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("after it was said: %q", w)
	}
	if note := st.ProblemNote(); note != "" {
		t.Errorf("note again %q", note)
	}
}

// A removed row with a comment not settled done is warned of until the
// comment is settled done, or the row comes back.
func TestRemovedOpenComment(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"x\">行</Item>\n")
	st.Removed()
	e, _ := st.Append(Event{Op: "thought", ID: "x", Text: "これは？"})
	st.Append(Event{Op: "send"})
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	want := `warning: row "x" (行) was removed but it has an open comment (` + strconv.Itoa(e.N) + `)`
	if w := st.Removed(); len(w) != 1 || w[0] != want {
		t.Fatalf("warnings %q", w)
	}
	// Back: no warning.
	writeReport(t, st, "<Item id=\"x\">行</Item>\n")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("back: %q", w)
	}
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 1 {
		t.Errorf("gone again: %q", w)
	}
	st.SettleAll([]Settlement{{e.N, "doing", "見ています"}})
	if w := st.Removed(); len(w) != 1 {
		t.Errorf("doing is still open: %q", w)
	}
	st.Settle(e.N, "答えました")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("settled done: %q", w)
	}
}

// What counts as answered is what is in effect now: a reaction or hiding
// taken back (u) does not count, and reading (A, or hiding then u) answers
// only a look="y" row, not one that asks.
func TestRemovedAnsweredInEffect(t *testing.T) {
	st := testStore(t)
	src := `<Ask id="undone">答えを戻した問い
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
<Ask id="q">選ぶ
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
<Ask id="unhid">隠して戻した問い
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
<Item id="lk" look="y">読めばいい行</Item>
<Item id="w" wait="you">手の要る行</Item>
<Item id="w2" wait="you">コメントした行</Item>
`
	writeReport(t, st, src)
	st.Removed()
	nodes, _, _ := report.Parse(src)
	byID := map[string]*report.Node{}
	report.WalkAll(nodes, func(n *report.Node) { byID[n.ID] = n })
	r, _ := st.Append(Event{Op: "react", ID: "undone", Value: "a", Option: "A"})
	st.Append(Event{Op: "undo", Undo: r.N})
	h, _ := st.Append(Event{Op: "hide", Key: report.HideKey(byID["unhid"])})
	st.Append(Event{Op: "undo", Undo: h.N})
	// Read everything (A): only the look row is handled by it.
	var hs []string
	for _, n := range byID {
		hs = append(hs, viewHash(n))
	}
	st.AddViewed(hs)
	c, _ := st.Append(Event{Op: "thought", ID: "w2", Text: "やりました"})
	st.Append(Event{Op: "send"})
	st.Settle(c.N, "了解")
	writeReport(t, st, "<Item id=\"other\">別の行</Item>\n")
	got := map[string]bool{}
	for _, w := range st.Removed() {
		got[strings.Split(w, `"`)[1]] = true
	}
	for _, id := range []string{"undone", "q", "unhid", "w"} {
		if !got[id] {
			t.Errorf("%s: not warned of (%v)", id, got)
		}
	}
	for _, id := range []string{"lk", "w2"} {
		if got[id] {
			t.Errorf("%s: warned of though handled", id)
		}
	}
}

// A comment the person has not sent is not the AI's business: no warning.
func TestRemovedUnsentComment(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"x\">行</Item>\n")
	st.Removed()
	st.Append(Event{Op: "thought", ID: "x", Text: "まだ送っていない"})
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("unsent: %q", w)
	}
}

// A Fold without an id is remembered by its target (Fold|title=…): its
// look="y" unread, or an open comment on it, is warned of when it goes.
func TestRemovedIDlessFold(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Fold title=\"読んでほしい\" look=\"y\">\n- 中身\n</Fold>\n<Fold title=\"コメントした\">\n- 中身\n</Fold>\n<Fold title=\"ただの\">\n- 中身\n</Fold>\n")
	st.Removed()
	e, _ := st.Append(Event{Op: "thought", ID: "Fold|title=コメントした", Text: "これは？"})
	st.Append(Event{Op: "send"})
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	want := []string{
		`warning: Fold "コメントした" was removed but it has an open comment (` + strconv.Itoa(e.N) + `)`,
		`warning: Fold "読んでほしい" was removed but the person has not hidden or answered it`,
	}
	if w := st.Removed(); strings.Join(w, "\n") != strings.Join(want, "\n") {
		t.Errorf("warnings %q", w)
	}
	if got := st.TakeRemoved(); len(got) != 2 {
		t.Errorf("taken %q", got)
	}
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("after taken: %q", w)
	}
}

// 'check' (Removed) says every warning until the line other commands end
// with has said it once; then it is forgotten. A row back, then gone
// again, is said again.
func TestRemovedSaidOnce(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"a\" look=\"y\">読む行</Item>\n")
	st.ProblemNote()
	writeReport(t, st, "<Item id=\"b\">別の行</Item>\n")
	want := `warning: row "a" (読む行) was removed but the person has not hidden or answered it`
	for range 3 {
		if w := st.Removed(); len(w) != 1 || w[0] != want {
			t.Errorf("check: %q", w)
		}
	}
	if note := st.ProblemNote(); note != want {
		t.Errorf("first: %q", note)
	}
	if note := st.ProblemNote(); note != "" {
		t.Errorf("second: %q", note)
	}
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("check after: %q", w)
	}
	if _, err := os.Stat(st.rowsPath()); err != nil {
		t.Fatal(err)
	}
	if rows := st.readRows(); len(rows) != 1 {
		t.Errorf("rows %v", rows)
	}
	writeReport(t, st, "<Item id=\"a\" look=\"y\">読む行</Item>\n")
	st.ProblemNote()
	writeReport(t, st, "<Item id=\"b\">別の行</Item>\n")
	if note := st.ProblemNote(); note != want {
		t.Errorf("gone again: %q", note)
	}
}

// A report with problems: the removal and the problem line both.
func TestRemovedWithProblems(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"a\" look=\"y\">読む行</Item>\n")
	st.ProblemNote()
	writeReport(t, st, "<Item id=\"b\" state=\"bogus\">別の行</Item>\n")
	note := st.ProblemNote()
	if !strings.HasPrefix(note, `warning: row "a"`) || !strings.HasSuffix(note, "report.md has 1 problem(s). Run: pitboard -s s1 check") {
		t.Errorf("note %q", note)
	}
}

// A version with problems does not overwrite what was remembered: the
// warning names the row as the last good version had it.
func TestRemovedTitleFromValidParse(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"a\" look=\"y\">正しい題</Item>\n")
	st.Removed()
	writeReport(t, st, "<Item id=\"a\" look=\"y\" state=\"bogus\">broken</Item>\n")
	if _, probs, err := report.Parse("<Item id=\"a\" look=\"y\" state=\"bogus\">broken</Item>\n"); err != nil || len(probs) == 0 {
		t.Fatalf("the broken version must parse with problems: %v %v", probs, err)
	}
	st.Removed()
	writeReport(t, st, "<Item id=\"b\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 1 || !strings.Contains(w[0], "(正しい題)") {
		t.Errorf("warnings %q", w)
	}
}

// FIX21: a row the person's screen showed is remembered (RememberShown),
// so removing it before the AI runs any command is still warned of; and
// an answer given to what an Ask said before does not answer it.
func TestRemovedShownAndStaleAnswer(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Ask id=\"q\">どちら\n<Option key=\"a\">A</Option>\n<Option key=\"b\">B</Option>\n</Ask>\n")
	st.RememberShown() // the viewer showed it; no AI command yet
	writeReport(t, st, "<Item id=\"other\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 1 || !strings.Contains(w[0], `row "q"`) {
		t.Fatalf("shown, then removed: %q", w)
	}

	st2 := testStore(t)
	v1 := "<Ask id=\"q\">どちら\n<Option key=\"a\">A</Option>\n<Option key=\"b\">B</Option>\n</Ask>\n"
	writeReport(t, st2, v1)
	nodes, _, _ := report.Parse(v1)
	st2.Append(Event{Op: "react", ID: "q", Value: "a", Option: "A", Line: report.ByID(nodes, "q").Sig()})
	st2.Removed()
	// Rewritten: the board shows q waiting again; then removed.
	writeReport(t, st2, "<Ask id=\"q\">どちらにする (書き直した)\n<Option key=\"a\">A</Option>\n<Option key=\"b\">B</Option>\n</Ask>\n")
	st2.Removed()
	writeReport(t, st2, "<Item id=\"other\">別の行</Item>\n")
	if w := st2.Removed(); len(w) != 1 || !strings.Contains(w[0], "has not hidden or answered") {
		t.Errorf("answered an older version, then removed: %q", w)
	}
}

// An Ask the person answered with a sent comment instead of an option is
// answered: removing it is not warned of (once the comment is settled).
func TestRemovedAskCommented(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, `<Ask id="q">選ぶ
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
`)
	st.Removed()
	rec, _ := st.Append(Event{Op: "thought", ID: "q", Text: "どちらでもない", Title: "選ぶ"})
	st.Append(Event{Op: "send"})
	st.Append(Event{Op: "settle", Thought: rec.N, Text: "answered", By: "ai"})
	writeReport(t, st, "<Item id=\"other\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("warned of an Ask answered by a comment: %q", w)
	}
}
