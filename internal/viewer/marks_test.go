package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

const marksReport = `<Title>t</Title>
<Section title="あなたへ">
<Item id="plan" ask="y">この方針で進めてよいか</Item>
<Item id="other" look="y">ただの項目</Item>
</Section>
<Fold title="済んだこと">
<Ask id="q">どちら
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
</Fold>
`

func marksApp(t *testing.T) (*board.Store, *app) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(marksReport), 0o644)
	return st, testApp(t, st)
}

func TestWaitingAndNewMarks(t *testing.T) {
	st, a := marksApp(t)
	pv := a.top()
	a.render()
	if w, f := a.counts(pv); w != 2 || f != 1 {
		t.Fatalf("counts %d %d, want 2 waiting (plan, q) and 1 new (other)", w, f)
	}
	var frame strings.Builder
	a.t.out = &frame
	a.t.prev = nil
	a.render()
	s := reEsc.ReplaceAllString(frame.String(), "")
	if !strings.Contains(s, "反応待ち 2  未読 1") {
		t.Errorf("top line:\n%s", s)
	}
	// The marks have a column of their own, beside the cursor's, the
	// Fold's too.
	rows := shown(a)
	for _, want := range []string{"> ● · この方針", "  ● · ただの項目", "  ● ▸ 済んだこと"} {
		if !strings.Contains(rows, want) {
			t.Errorf("want %q in\n%s", want, rows)
		}
	}
	// y on plan: no longer waiting.
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.handle(key{r: 'y'})
	if w, _ := a.counts(pv); w != 1 {
		t.Errorf("waiting after y: %d", w)
	}
	// On "other": still unread while the cursor is on it, however long.
	a.handle(key{r: 'j'})
	a.render()
	wait(t, settleDelay)
	a.render()
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("unread while on it: %d", f)
	}
	if rows := shown(a); !strings.Contains(rows, "> ● · ただの項目") {
		t.Errorf("the mark goes under the cursor:\n%s", rows)
	}
	// Leaving it, coming back, opening: still unread.
	a.handle(key{r: 'k'})
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	wait(t, settleDelay)
	a.render()
	a.handle(key{name: "enter"})
	a.render()
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("unread after leaving and opening: %d", f)
	}
	// r: read. The hint says so only while there is a green mark.
	if s := hintLine(a); !strings.Contains(s, "r 読んだ") {
		t.Errorf("hints %q", s)
	}
	a.handle(key{r: 'r'})
	a.render()
	if _, f := a.counts(pv); f != 0 {
		t.Errorf("unread after r: %d", f)
	}
	if s := hintLine(a); strings.Contains(s, "r 読んだ") {
		t.Errorf("hints after r %q", s)
	}
	// u takes r back; r again.
	a.handle(key{r: 'u'})
	if _, f := a.counts(pv); f != 1 || a.msg != "r (読んだ) を取り消しました: ただの項目" {
		t.Errorf("after u: %d %q", f, a.msg)
	}
	a.handle(key{r: 'r'})
	// It is remembered in the file, not in events.jsonl.
	a2 := testApp(t, st)
	if _, f := a2.counts(a2.top()); f != 0 {
		t.Errorf("a new viewer sees %d unread", f)
	}
	for _, e := range st.Events() {
		if e.Op != "react" {
			t.Errorf("unexpected event %+v", e)
		}
	}
	// What the line says changes: unread again.
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(marksReport, "ただの項目", "ただの項目 (改)", 1)), 0o644)
	a.reload(pv)
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("changed line not unread: %d", f)
	}
	// A: everything now in the report is read; u takes that back.
	a.handle(key{r: 'A'})
	if w, f := a.counts(pv); w != 1 || f != 0 {
		t.Errorf("after A: %d %d", w, f)
	}
	a.handle(key{r: 'u'})
	if _, f := a.counts(pv); f != 1 || !strings.Contains(a.msg, "A") {
		t.Errorf("after u: %d %q", f, a.msg)
	}
	// Lines without look="y" are never unread.
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(marksReport, ` look="y"`, "", 1)), 0o644)
	a.reload(pv)
	if _, f := a.counts(pv); f != 0 {
		t.Errorf("unread without look: %d", f)
	}
}

func TestSpaceJumps(t *testing.T) {
	_, a := marksApp(t)
	pv := a.top()
	a.render()
	// From plan (waiting), the nearest marked line below: other (unread),
	// then q inside the closed Fold (opened), then round to plan.
	for _, want := range []string{"other", "q", "plan"} {
		a.handle(key{r: ' '})
		a.render()
		if pv.cur != want || pv.placed {
			t.Fatalf("space: cur %q, want %q", pv.cur, want)
		}
	}
	if !pv.open["fold:1"] {
		t.Error("the Fold was not opened")
	}
	// The line just jumped to waits settleDelay.
	a.handle(key{r: 'y'})
	if len(a.st.Events()) != 0 {
		t.Error("y right after a jump was recorded")
	}
	wait(t, settleDelay)
	a.render()
	a.handle(key{r: 'y'})
	// space is still of use (on a narrow line it goes before g).
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !a.anyMarked(a.top()) || !strings.Contains(s, "g 飛ぶ") {
		t.Errorf("hints: %q", s)
	}
	// An Item waiting for the person (wait="you") does not hold space:
	// with two of them, space still goes on to what is below.
	os.WriteFile(a.st.ReportPath(), []byte(`<Item id="w1" wait="you" status="あなた待ち">一</Item>
<Item id="w2" wait="you" status="あなた待ち">二</Item>
<Item id="n1" look="y">三</Item>
`), 0o644)
	a.reload(pv)
	pv.cur = "w1"
	for _, want := range []string{"w2", "n1", "w1"} {
		a.handle(key{r: ' '})
		if pv.cur != want {
			t.Fatalf("space with wait=you: %q, want %q", pv.cur, want)
		}
	}
	a.handle(key{r: 'A'})
	os.WriteFile(a.st.ReportPath(), []byte(`<Item id="n1">三</Item>`), 0o644)
	a.reload(pv)
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); strings.Contains(s, "space") {
		t.Errorf("hints with nothing to go to: %q", s)
	}
	a.handle(key{r: ' '})
	if a.msg != "印のある行はありません" {
		t.Errorf("msg %q", a.msg)
	}
}

func TestBrokenViewedFile(t *testing.T) {
	st, _ := marksApp(t)
	os.WriteFile(st.ViewedPath(), []byte("garbage\n\x00\x01\n"), 0o644)
	a := testApp(t, st)
	if _, f := a.counts(a.top()); f != 1 {
		t.Errorf("broken file: %d new", f)
	}
	os.Remove(st.ViewedPath())
	os.Mkdir(st.ViewedPath(), 0o755) // cannot be read as a file
	a = testApp(t, st)
	a.render()
	a.handle(key{r: 'A'}) // cannot be written either: no crash
}

const fix8Report = `<Item id="a" look="y">一</Item>
<Item id="b" look="y">二</Item>
<Item id="c" look="y">三</Item>
<Item id="d" look="y">四</Item>
<Fold title="束" look="y">
<Item id="e" look="y">五</Item>
</Fold>
`

func fix8App(t *testing.T) (*board.Store, *app) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(fix8Report), 0o644)
	a := testApp(t, st)
	a.render()
	return st, a
}

// to puts the cursor on the line with id (pointed at by the person).
func to(t *testing.T, a *app, id string) {
	t.Helper()
	for i := 0; i < 10 && a.top().cur != id; i++ {
		a.handle(key{r: 'j'})
		a.render()
	}
	wait(t, settleDelay)
	a.render()
	if a.top().cur != id {
		t.Fatalf("cursor on %q", a.top().cur)
	}
}

func unreadOf(a *app, id string) bool { return a.isNew(report.ByID(a.top().nodes, id)) }

func TestReadOnlyByKeys(t *testing.T) {
	_, a := fix8App(t)
	// Moving over every line and opening the Fold reads nothing.
	for i := 0; i < 8; i++ {
		a.handle(key{r: 'j'})
		a.render()
		a.handle(key{name: "enter"})
		a.render()
	}
	for i := 0; i < 8; i++ {
		a.handle(key{r: 'k'})
		a.render()
	}
	if _, f := a.counts(a.top()); f != 6 {
		t.Fatalf("unread after moving: %d", f)
	}
	// y reads its line; a comment on a line too; x too.
	to(t, a, "b")
	a.handle(key{r: 'y'})
	if unreadOf(a, "b") {
		t.Error("y did not read the line")
	}
	to(t, a, "c")
	a.handle(key{r: 'c'})
	if !unreadOf(a, "c") {
		t.Error("only starting to write read the line")
	}
	if !a.write("コメント") || unreadOf(a, "c") {
		t.Error("the comment did not read the line")
	}
	a.render()
	to(t, a, "d")
	a.handle(key{r: 'x'})
	if unreadOf(a, "d") {
		t.Error("x did not read the line")
	}
	if !unreadOf(a, "a") {
		t.Error("a was read without a key")
	}
}

func TestReadFold(t *testing.T) {
	_, a := fix8App(t)
	to(t, a, "a")
	a.handle(key{r: 'r'})
	// r on a line without a green mark does nothing (nothing to take back).
	n := len(a.hist)
	a.handle(key{r: 'r'})
	if len(a.hist) != n {
		t.Error("r on a read line was recorded")
	}
	// r on the Fold: its mark goes, what it holds included.
	pv := a.top()
	for i := 0; i < 10; i++ {
		if n := pv.nodeAt(pv.cur); n != nil && n.Kind == report.KindDetails {
			break
		}
		a.handle(key{r: 'j'})
		a.render()
	}
	wait(t, settleDelay)
	a.render()
	a.handle(key{r: 'r'})
	if unreadOf(a, "e") || a.markOfLine(pv, pv.nodeAt(pv.cur)) != markNone {
		t.Error("r on the Fold left a green mark")
	}
	a.handle(key{r: 'u'})
	if !unreadOf(a, "e") || !strings.HasPrefix(a.msg, "r (読んだ) を取り消しました") {
		t.Errorf("u after r: %q", a.msg)
	}
}
