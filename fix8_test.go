package main

import (
	"os"
	"strings"
	"testing"
)

const fix8Report = `<Item id="a" look="y">一</Item>
<Item id="b" look="y">二</Item>
<Item id="c" look="y">三</Item>
<Item id="d" look="y">四</Item>
<Fold title="束" look="y">
<Item id="e" look="y">五</Item>
</Fold>
`

func fix8App(t *testing.T) (*Store, *app) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(fix8Report), 0o644)
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

func unreadOf(a *app, id string) bool { return a.isNew(byID(a.top().nodes, id)) }

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
		if n := pv.nodeAt(pv.cur); n != nil && n.kind == kDetails {
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

func TestRIsAViewerKey(t *testing.T) {
	_, probs, _ := parseReport("<Ask id=\"q\">q\n<Option key=\"r\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>")
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
		t.Errorf("r as an Option key: %v", probs)
	}
}
