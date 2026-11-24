package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

func TestNestedListHidesWithItsLine(t *testing.T) {
	src := "<Section title=\"s\">\n- 板はセッションごと\n  - 理由 A\n- ほか\n</Section>"
	nodes, _, _ := report.Parse(src)
	top := nodes[0].Kids[0]
	if k := report.HideKey(top); k != "-|板はセッションごと\n  -理由 A" {
		t.Errorf("key %q", k)
	}
	if k := report.HideKey(nodes[0].Kids[1]); k != "-|ほか" {
		t.Errorf("key without nesting changed: %q", k)
	}
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(src), 0o644)
	a := testApp(t, st)
	pv := a.top()
	if keys := pv.navKeys(); len(keys) != 2 {
		t.Errorf("stops %q: nested lines must not be stops", keys)
	}
	a.render()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	if s := shown(a); strings.Contains(s, "理由 A") || strings.Contains(s, "板はセッションごと") {
		t.Errorf("hidden with its nested lines:\n%s", s)
	}
	// A numbered line is read only.
	nodes, _, _ = report.Parse("<Section title=\"s\">\n1. a\n</Section>")
	if pv.navigable(nodes[0].Kids[0]) {
		t.Error("a numbered line is a stop")
	}
}

const hideReport = `<Title>t</Title>
<Section title="決まったこと">
- 板はセッションごと
<Item id="a" status="作業中">題 A</Item>
</Section>
<Section title="ほか">
<Item id="b">題 B</Item>
</Section>
`

// shown is the report as drawn, without styles.
func shown(a *app) string {
	var out []string
	for _, r := range a.rows(a.top()) {
		out = append(out, reEsc.ReplaceAllString(r.line, ""))
	}
	return strings.Join(out, "\n")
}

func TestHideAndUnhide(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.draw()
	if pv.cur != "-1:板はセッションごと" {
		t.Fatalf("the cursor must stop at the plain line first: %q", pv.cur)
	}
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.draw()
	// y n (no keys any more) and c do nothing on a plain line.
	for _, r := range "ync" {
		a.handle(key{r: r})
	}
	if len(st.Events()) != 0 || a.in != nil {
		t.Fatal("y n c acted on a plain line")
	}
	a.handle(key{r: 'x'})
	a.draw()
	if s := shown(a); strings.Contains(s, "板はセッションごと") {
		t.Errorf("the hidden line is still drawn:\n%s", s)
	}
	if pv.cur != "a" {
		t.Errorf("after hiding, the cursor goes to the next line: %q", pv.cur)
	}
	// x moved the cursor there: x again hides it at once (y is no key).
	a.handle(key{r: 'y'})
	a.handle(key{r: 'x'})
	if evs := st.Events(); len(evs) != 2 {
		t.Fatalf("x right after x: %+v", evs)
	}
	a.draw()
	if s := shown(a); strings.Contains(s, "決まったこと") {
		t.Errorf("a section whose lines are all hidden is still drawn:\n%s", s)
	}
	if pv.hiddenCount() != 2 {
		t.Errorf("hidden count = %d", pv.hiddenCount())
	}

	// H shows them faintly, and x on one brings it back.
	a.handle(key{r: 'H'})
	a.draw()
	if s := shown(a); !strings.Contains(s, "⌀ 板はセッションごと") || !strings.Contains(s, "決まったこと") {
		t.Errorf("H does not show hidden lines:\n%s", s)
	}
	pv.cur = "a"
	a.draw()
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	a.handle(key{r: 'H'})
	a.draw()
	if s := shown(a); !strings.Contains(s, "題 A") || strings.Contains(s, "板はセッションごと") {
		t.Errorf("after unhiding a:\n%s", s)
	}

	// The AI changes the hidden line's status: it is a new line and shows.
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(hideReport, "- 板はセッションごと", "- 板はセッションごと (確定)", 1)), 0o644)
	a.reload(pv)
	a.draw()
	if s := shown(a); !strings.Contains(s, "板はセッションごと (確定)") {
		t.Errorf("a changed line must show again:\n%s", s)
	}

	sendAll(st)
	got, _ := st.Changes()
	want := []string{"hide (2): 板はセッションごと / 題 A", "unhide (1): 題 A"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s", strings.Join(got, "\n"))
	}
}

func TestChangedItemShowsAgain(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	n := report.ByID(a.rep.Nodes, "a")
	st.Append(board.Event{Op: "hide", Key: report.HideKey(n), Title: n.Title()})
	a.loadEvents()
	if s := shown(a); strings.Contains(s, "題 A") {
		t.Fatalf("hidden:\n%s", s)
	}
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(hideReport, `status="作業中"`, `state="done"`, 1)), 0o644)
	a.reload(a.top())
	if s := shown(a); !strings.Contains(s, "題 A") {
		t.Errorf("the item's state changed, so it shows again:\n%s", s)
	}
}

// x records only if the line is still what the screen shows.
func TestHideChecksTheFile(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.draw()
	a.handle(key{r: 'j'})
	a.draw() // cursor on a
	if pv.cur != "a" {
		t.Fatalf("cursor %q", pv.cur)
	}
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(hideReport, `status="作業中"`, `status="needs you"`, 1)), 0o644)
	a.handle(key{r: 'x'})
	if evs := st.Events(); len(evs) != 0 {
		t.Errorf("x recorded the old line: %+v", evs)
	}
	if a.msg != msgChanged {
		t.Errorf("msg = %q", a.msg)
	}
}

// H takes the cursor's line away: the line the cursor goes to takes no
// keys until it has been on screen for a while.
func TestHMovesCursorAndWaits(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.draw()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.draw()
	a.handle(key{r: 'H'})
	a.handle(key{r: 'x'}) // hide the plain line while shown: the cursor stays
	a.draw()
	if pv.cur != "-1:板はセッションごと" {
		t.Fatalf("cursor %q", pv.cur)
	}
	a.handle(key{r: 'H'}) // the line goes away; the cursor moves to a
	a.draw()
	a.handle(key{r: 'x'})
	if pv.cur != "a" || len(st.Events()) != 1 || a.msg != msgSettlingHide {
		t.Fatalf("x acted on a line the viewer moved to: cur %q %q %+v", pv.cur, a.msg, st.Events())
	}
	// The same key again works (another key is dropped too: page's
	// TestGuardAgainIsPerKey).
	a.handle(key{r: 'x'})
	if evs := st.Events(); len(evs) != 2 || evs[1].Op != "hide" || !strings.Contains(evs[1].Key, "題 A") {
		t.Fatalf("x pressed again works: %+v", evs)
	}
	a.handle(key{r: 'u'})
	a.handle(key{r: 'H'})
	a.handle(key{r: 'H'}) // shown and left out again: the cursor stays on a
	a.draw()
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	if evs := st.Events(); evs[len(evs)-1].Op != "hide" {
		t.Errorf("after the delay x works without j/k: %+v", evs)
	}
}
