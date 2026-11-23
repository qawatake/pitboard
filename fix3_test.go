package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A board made at the top of a repository is found from below it, by the
// hook and by the commands; a new board goes where the boards are.
func TestBoardFoundFromBelow(t *testing.T) {
	st := testStore(t)
	sub := filepath.Join(st.base, "a", "b")
	os.MkdirAll(sub, 0o755)
	st.Append(Event{Op: "thought", Text: "下から"})
	sendAll(st)

	if got := runHook(t, "post-tool", hookInput(sub, "s1", "")); !strings.Contains(got, "thought 1: 下から") {
		t.Errorf("hook from a subdirectory: %q", got)
	}
	st.Append(Event{Op: "thought", Text: "もう一つ"})
	sendAll(st)
	t.Chdir(sub)
	var out strings.Builder
	if err := run([]string{"-s", "s1", "changes"}, &out); err != nil || !strings.Contains(out.String(), "thought 3: もう一つ") {
		t.Errorf("changes from a subdirectory: %v %q", err, out.String())
	}
	out.Reset()
	if err := run([]string{"-s", "s", "settle", "3", "見送った"}, &out); err != nil {
		t.Errorf("settle by prefix from a subdirectory: %v", err)
	}
	out.Reset()
	if err := run([]string{"-s", "new1", "path"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(st.base, ".pitboard", "new1", "report.md"); strings.TrimSpace(out.String()) != want {
		t.Errorf("new board at %q, want %q", out.String(), want)
	}
	if _, err := os.Stat(filepath.Join(sub, ".pitboard")); err == nil {
		t.Error("path made a second .pitboard/ in the subdirectory")
	}
}

func TestNewBoardWithoutAnyHere(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out strings.Builder
	if err := run([]string{"-s", "x1", "path"}, &out); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(filepath.Dir(strings.TrimSpace(out.String())))
	if got != filepath.Join(real, ".pitboard", "x1") {
		t.Errorf("path = %q", out.String())
	}
}

func TestAncestorsStopAtHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	as := ancestors(filepath.Join(home, "x", "y"))
	if as[len(as)-1] != filepath.Clean(home) {
		t.Errorf("ancestors go above home: %q", as)
	}
}

func TestHookInputLimit(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "x"})
	sendAll(st)
	in := hookInput(st.base, "s1", `"tool_response":"`+strings.Repeat("a", hookInputLimit)+`"`)
	if got := runHook(t, "post-tool", in); got != "" {
		t.Errorf("input over the limit was read: %q", got[:min(len(got), 80)])
	}
	if got := runHook(t, "post-tool", hookInput(st.base, "s1", "")); got == "" {
		t.Error("the thought must still be there")
	}
}

func TestHookOnUnwritableBoard(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	st := testStore(t)
	os.Chmod(st.dir, 0o555)
	defer os.Chmod(st.dir, 0o755)
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
		if got := runHook(t, ev, hookInput(st.base, "s1", "")); got != "" {
			t.Errorf("%s: %q", ev, got)
		}
	}
	if _, err := os.Stat(st.eventsPath()); err == nil {
		t.Error("events.jsonl was created")
	}
	// Written but unreadable: still nothing, and no error.
	os.Chmod(st.dir, 0o755)
	st.Append(Event{Op: "thought", Text: "x"})
	os.Chmod(st.eventsPath(), 0o000)
	defer os.Chmod(st.eventsPath(), 0o644)
	if got := runHook(t, "post-tool", hookInput(st.base, "s1", "")); got != "" {
		t.Errorf("unreadable events: %q", got)
	}
}

func TestStopDoesNotBlockForHides(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.base, "s1", "")
	st.Append(Event{Op: "hide", Key: "-|x", Title: "x"})
	sendAll(st)
	if got := runHook(t, "stop", in); got != "" {
		t.Errorf("stop blocked for a hide: %q", got)
	}
	if got := runHook(t, "prompt-submit", in); !strings.Contains(got, "hide: x") {
		t.Errorf("the hide must wait for the next delivery: %q", got)
	}
	st.Append(Event{Op: "hide", Key: "-|y", Title: "y"})
	st.Append(Event{Op: "react", ID: "hook", Value: "y"})
	sendAll(st)
	if got := runHook(t, "stop", in); !strings.Contains(got, "block") || !strings.Contains(got, "hide: y") {
		t.Errorf("stop with a reaction: %q", got)
	}
}

// x records only if the line is still what the screen shows.
func TestHideChecksTheFile(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.draw()
	a.handle(key{r: 'j'})
	a.draw() // cursor on a
	if pv.cur != "a" {
		t.Fatalf("cursor %q", pv.cur)
	}
	os.WriteFile(st.reportPath(), []byte(strings.Replace(hideReport, `status="作業中"`, `status="needs you"`, 1)), 0o644)
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
	os.WriteFile(st.reportPath(), []byte(hideReport), 0o644)
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
	a.handle(key{r: 'n'})
	a.draw()
	a.handle(key{r: 'n'})
	if pv.cur != "a" || len(st.Events()) != 1 {
		t.Fatalf("n acted on a line the viewer moved to: cur %q %+v", pv.cur, st.Events())
	}
	wait(t, settleDelay)
	a.handle(key{r: 'n'})
	if evs := st.Events(); len(evs) != 2 || evs[1].Value != "n" || evs[1].ID != "a" {
		t.Errorf("after the delay n works without j/k: %+v", evs)
	}
}

func TestSmallDisplayFixes(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.t.w = 40
	// 9c: a placed cursor on an Ask says how to answer.
	a.render()
	if pv.cur != "unit" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k ↑↓ → x") && !strings.Contains(s, "動かすと x が効く") {
		t.Errorf("placed on a closed Ask: %q", s)
	}
	a.handle(key{name: "enter"})
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k ↑↓ → 答える") && !strings.Contains(s, "動かすと答えられる") {
		t.Errorf("placed on an open Ask: %q", s)
	}
	// 9a: at width 40 the pane says there is more, and where.
	for i := 0; i < 7; i++ {
		st.Append(Event{Op: "thought", Text: "t"})
	}
	sendAll(st)
	st.Changes()
	st.Settle(1, "x")
	st.Append(Event{Op: "thought", Text: "u"})
	a.loadEvents()
	p := reEsc.ReplaceAllString(strings.Join(a.pane(30), "\n"), "")
	if !strings.Contains(p, "s で送る") {
		t.Errorf("pane at width 40:\n%s", p)
	}
	// 9d: a long destination does not squeeze the thought in the list.
	a.t.w = 80
	th, _ := st.Append(Event{Op: "thought", Text: "the thought itself"})
	st.Settle(th.N, strings.Repeat("とても長い行き先 ", 20))
	a.loadEvents()
	l := reEsc.ReplaceAllString(strings.Join(func() []string {
		var out []string
		for _, r := range a.listRows() {
			out = append(out, r.line)
		}
		return out
	}(), "\n"), "")
	if !strings.Contains(l, "the thought itself") || strings.Contains(l, "th…") {
		t.Errorf("list:\n%s", l)
	}
}
