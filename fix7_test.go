package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// 2: what a key in the editor did is said there.
func TestEditorShowsMessage(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	t.Setenv("PATH", t.TempDir()) // no osascript, no clipboard tools
	a.handle(key{r: 'm'})
	a.handle(key{r: 'x'})
	a.handle(key{name: "ctrl+v"})
	if a.msg == "" {
		t.Fatal("ctrl+v said nothing")
	}
	if s := plainLines(a.bottom()); !strings.Contains(s, a.msg) {
		t.Errorf("not on screen: %q\n%s", a.msg, s)
	}
	a.handle(key{r: 'y'})
	if s := plainLines(a.bottom()); strings.Contains(s, "クリップボード") || a.in.ed.String() != "xy" {
		t.Errorf("after the next key: %q", s)
	}
}

// 3, 7: no line runs past the width, however narrow.
func TestNothingPastTheWidth(t *testing.T) {
	st := testStore(t)
	report := "<Title>テストのとても長い題</Title>\n<Item id=\"a\" ask=\"y\">一</Item>\n"
	for i := range 5 {
		report += fmt.Sprintf("<Item id=\"n%d\" look=\"y\">新しい %d</Item>\n", i, i)
	}
	os.WriteFile(st.reportPath(), []byte(report), 0o644)
	st.Append(Event{Op: "thought", Text: "a"})
	for _, w := range []int{1, 5, 10, 16, 20, 21, 30} {
		for _, help := range []bool{false, true} {
			a := testApp(t, st)
			a.t.w, a.t.h = w, 24
			a.help = help
			ls, _ := screen(t, a)
			for i, l := range ls {
				if visibleWidth(l) > w {
					t.Errorf("width %d help %v: line %d is %d wide: %q", w, help, i, visibleWidth(l), l)
				}
			}
		}
	}
	// The counts at the top right go from the right when they do not fit.
	a := testApp(t, st)
	a.t.w = 30
	if m := a.sendMark().plain(); m != "反応待ち 1  未読 5" {
		t.Errorf("mark at 30: %q", m)
	}
	a.t.w = 20
	if m := a.sendMark().plain(); m != "反応待ち 1" {
		t.Errorf("mark at 20: %q", m)
	}
}

// 5: taking back a rewritten comment tells the text the AI last got.
func TestRetractTellsTheLatestText(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "ひとこと"})
	changes(t, st)
	st.Append(Event{Op: "edit", Thought: th.N, Text: "直した"})
	changes(t, st)
	st.Append(Event{Op: "retract", Thought: th.N})
	if got := changes(t, st); got != fmt.Sprintf("retract thought %d: 直した", th.N) {
		t.Errorf("changes %q", got)
	}
	// Rewritten but not sent, then taken back: the AI last got the first.
	th2, _ := st.Append(Event{Op: "thought", Text: "最初"})
	changes(t, st)
	st.Append(Event{Op: "edit", Thought: th2.N, Text: "送っていない"})
	st.Append(Event{Op: "retract", Thought: th2.N})
	if got := changes(t, st); got != fmt.Sprintf("retract thought %d: 最初", th2.N) {
		t.Errorf("changes %q", got)
	}
}

// 11, 12: a bold head word on a list line keeps a line of its own, and
// lines under "1. " may be indented to its text.
func TestListHeadsAndNumberedIndent(t *testing.T) {
	src := "<Item id=\"a\">t\n- **XDG に置く**\n  続きの行がここに来ます。\n1. 文\n   - 入れ子\n   - 入れ子 2\n2. 次\n</Item>"
	nodes, probs, _ := parseReport(src)
	if len(probs) != 0 {
		t.Fatalf("problems %v", probs)
	}
	body := nodes[0].body
	if body[0].text != "**XDG に置く**\n続きの行がここに来ます。" {
		t.Errorf("head: %q", body[0].text)
	}
	if len(body) != 3 || len(body[1].subs) != 2 || body[2].num != "2." {
		t.Errorf("numbered: %d, subs %d", len(body), len(body[1].subs))
	}
	// Problems that leave nothing out do not say "what could be read".
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="a" wait="me">x</Item>`), 0o644)
	a := testApp(t, st)
	if s := shown(a); !strings.Contains(s, "誤りがあります") || strings.Contains(s, "読めた所まで") {
		t.Errorf("complete report:\n%s", s)
	}
	os.WriteFile(st.reportPath(), []byte("<Item id=\"a\">x</Item>\n<Bogus>y</Bogus>"), 0o644)
	a.reload(a.top())
	if s := shown(a); !strings.Contains(s, "読めた所まで") {
		t.Errorf("partial report:\n%s", s)
	}
}

// 16: the end of the report under the comments is said.
func TestMoreUnderTheComments(t *testing.T) {
	st := testStore(t)
	report := ""
	for i := range 40 {
		report += fmt.Sprintf("<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.reportPath(), []byte(report), 0o644)
	st.Append(Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	ls, _ := screen(t, a)
	if !strings.Contains(strings.Join(ls, "\n"), "↓ あと") {
		t.Errorf("no sign of more:\n%s", strings.Join(ls, "\n"))
	}
}

// 16: a waiting file whose pid now belongs to another process.
func TestWaiterStartTime(t *testing.T) {
	st := testStore(t)
	me := os.Getpid()
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d %s\n", me, startOf(me))), 0o644)
	if st.Waiter() != me {
		t.Error("own pid with its start not taken")
	}
	if startOf(me) == "" {
		t.Skip("ps gives no start time here")
	}
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d Mon Jan  1 00:00:00 2001\n", me)), 0o644)
	if st.Waiter() != 0 {
		t.Error("a reused pid counts as waiting")
	}
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d\n", me)), 0o644)
	if st.Waiter() != me {
		t.Error("an old file (pid only) is not read")
	}
}
