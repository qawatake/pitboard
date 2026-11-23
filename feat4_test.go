package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- taking a thought back ----

func TestRetractBeforeDelivery(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "keep"})
	th, _ := st.Append(Event{Op: "thought", Text: "mistake"})
	st.Append(Event{Op: "retract", Thought: th.N})
	if n := unsent(st.Events(), st.Seen()); n != 1 {
		t.Errorf("unsent = %d, want 1 (the taken-back thought is not counted)", n)
	}
	lines, _ := st.Changes()
	if strings.Join(lines, "\n") != "thought 1: keep" {
		t.Errorf("changes: %q", lines)
	}
	if s := replayEvents(st.Events(), st.Seen()); len(s.thoughts) != 1 || s.thoughts[0].Text != "keep" {
		t.Errorf("the board must drop it: %+v", s.thoughts)
	}
}

func TestRetractAfterDelivery(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", ID: "hook", Text: "とり下げる思いつき\n二行目"})
	st.Changes()
	st.Settle(th.N, "todo にした") // given a destination, then taken back
	st.Append(Event{Op: "retract", Thought: th.N})
	if n := unsent(st.Events(), st.Seen()); n != 1 {
		t.Errorf("unsent = %d", n)
	}
	lines, _ := st.Changes()
	if strings.Join(lines, "\n") != "retract thought 1: とり下げる思いつき 二行目" {
		t.Errorf("changes: %q", lines)
	}
	// settle on a taken-back thought is accepted silently.
	if err := st.Settle(th.N, "見送った"); err != nil {
		t.Errorf("settle after retract: %v", err)
	}
	// Taken back and undone before changes: nothing to tell.
	th2, _ := st.Append(Event{Op: "thought", Text: "two"})
	st.Changes()
	st.Append(Event{Op: "retract", Thought: th2.N})
	st.Append(Event{Op: "unretract", Thought: th2.N})
	if n := unsent(st.Events(), st.Seen()); n != 0 {
		t.Errorf("unsent after undo = %d", n)
	}
	if lines, _ := st.Changes(); len(lines) != 0 {
		t.Errorf("changes after undo: %q", lines)
	}
	// A retract delivered, then undone: the AI is told it stands again.
	st.Append(Event{Op: "retract", Thought: th2.N})
	st.Changes()
	st.Append(Event{Op: "unretract", Thought: th2.N})
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != fmt.Sprintf("unretract thought %d: two", th2.N) {
		t.Errorf("changes after a delivered retract was undone: %q", lines)
	}
	// Undone before delivery: the thought goes as usual.
	th3, _ := st.Append(Event{Op: "thought", Text: "three"})
	st.Append(Event{Op: "retract", Thought: th3.N})
	st.Append(Event{Op: "unretract", Thought: th3.N})
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != fmt.Sprintf("thought %d: three", th3.N) {
		t.Errorf("changes: %q", lines)
	}
}

func TestStopHookIgnoresTakenBack(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "x"})
	st.Append(Event{Op: "retract", Thought: th.N})
	if got := runHook(t, "stop", hookInput(st.base, "s1", "")); got != "" {
		t.Errorf("stop blocked for nothing: %q", got)
	}
}

func TestRetractInPaneAndUndo(t *testing.T) {
	st := testStore(t)
	for _, s := range []string{"one", "two", "three"} {
		st.Append(Event{Op: "thought", Text: s})
	}
	a := testApp(t, st)
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	a.handle(key{r: 'j'}) // into the pane, on "three"
	a.render()
	if !a.inPane || a.pcur != 3 {
		t.Fatalf("pane %v pcur %d", a.inPane, a.pcur)
	}
	if s := hintLine(a); s != "    enter 開く  x 取り消す  m 書く  ? キー" {
		t.Errorf("hints %q", s)
	}
	a.handle(key{r: 'x'})
	if a.msg != "取り消しました (u で戻す)" || a.pcur != 2 {
		t.Fatalf("msg %q pcur %d", a.msg, a.pcur)
	}
	a.render()
	if p := plainLines(a.pane(30)); strings.Contains(p, "three") {
		t.Errorf("taken back, still shown:\n%s", p)
	}
	// x again at once: the viewer moved the cursor, so it waits.
	a.handle(key{r: 'x'})
	if len(a.state.thoughts) != 2 {
		t.Fatal("x pressed twice took back the next thought")
	}
	// Back: u right after.
	a.handle(key{r: 'x'}) // dropped again, and it ends the chance to undo
	a.handle(key{r: 'u'})
	if len(a.state.thoughts) != 2 {
		t.Error("u after another key must not undo")
	}
	wait(t, settleDelay)
	a.render()
	a.handle(key{r: 'x'}) // "two" now
	a.handle(key{r: 'u'})
	if len(a.state.thoughts) != 2 || a.pcur != 2 || a.msg != "戻しました" {
		t.Errorf("u must undo: thoughts %d pcur %d msg %q", len(a.state.thoughts), a.pcur, a.msg)
	}
	evs := st.Events()
	if e := evs[len(evs)-1]; e.Op != "unretract" || e.Thought != 2 {
		t.Errorf("last event %+v", e)
	}
	// ✎ on the line goes too.
	th, _ := st.Append(Event{Op: "thought", ID: "hook", Text: "on hook"})
	a.loadEvents()
	if c := len(a.state.on("hook")); c != 1 {
		t.Fatalf("on hook %d", c)
	}
	st.Append(Event{Op: "retract", Thought: th.N})
	a.loadEvents()
	if c := len(a.state.on("hook")); c != 0 {
		t.Errorf("✎ still counts a taken-back thought")
	}
}

func TestRetractInList(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "one"})
	a := testApp(t, st)
	a.handle(key{name: "tab"})
	a.render()
	if s := hintLine(a); s != "    enter 開く  x 取り消す  m 書く  ? キー" {
		t.Errorf("list hints %q", s)
	}
	a.handle(key{r: 'x'})
	a.render()
	if len(a.state.thoughts) != 0 {
		t.Fatal("x in the list must take the thought back")
	}
	msg := a.msg
	a.msg = ""
	if s := hintLine(a); !strings.Contains(s, "tab 報告へ") {
		t.Errorf("empty list hints %q", s)
	}
	a.msg = msg
	a.handle(key{r: 'u'})
	if len(a.state.thoughts) != 1 || a.lcur != 1 {
		t.Errorf("u in the list: %d lcur %d", len(a.state.thoughts), a.lcur)
	}
}

// ---- no board ----

func TestNoBoardMessages(t *testing.T) {
	dir := t.TempDir()
	a := &app{t: &term{w: 120, h: 30}, base: dir, lopen: map[int]bool{}}
	a.setStore(nil)
	if s := shown(a); !strings.Contains(s, "このディレクトリと親に板 (.pitboard/) がありません") {
		t.Errorf("no .pitboard:\n%s", s)
	}
	if s := hintLine(a); !strings.Contains(s, filepath.Base(dir)) {
		t.Errorf("the place is not shown: %q", s)
	}
	os.Mkdir(filepath.Join(dir, ".pitboard"), 0o755)
	if s := shown(a); !strings.Contains(s, "板はまだありません。AI が報告を書くと、ここに出ます") {
		t.Errorf("empty .pitboard:\n%s", s)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if got := homeTilde(filepath.Join(home, "x", "y")); got != filepath.Join("~", "x", "y") {
			t.Errorf("homeTilde = %q", got)
		}
		if got := homeTilde(home + "z"); got != home+"z" {
			t.Errorf("homeTilde on a sibling = %q", got)
		}
	}
}
