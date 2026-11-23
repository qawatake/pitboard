package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

// onHook puts the cursor on the Item "hook", pointed at by the person.
func onHook(t *testing.T, a *app) {
	t.Helper()
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	wait(t, settleDelay)
	a.render()
	if a.top().cur != "hook" {
		t.Fatalf("cursor on %q", a.top().cur)
	}
}

func changes(t *testing.T, st *Store) string {
	t.Helper()
	sendAll(st)
	lines, err := st.Changes()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

func TestUndoReactionNotSent(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	onHook(t, a)
	a.handle(key{r: 'n'})
	a.handle(key{r: 'y'})
	a.handle(key{r: 'u'})
	if a.msg != "y を取り消しました: hook を入れるか決める" {
		t.Errorf("msg %q", a.msg)
	}
	if e := a.state.react["hook"]; e.Value != "n" {
		t.Errorf("after u: %q, want n back", e.Value)
	}
	if a.top().cur != "hook" {
		t.Error("the cursor moved")
	}
	a.handle(key{r: 'u'})
	if _, ok := a.state.react["hook"]; ok || a.unsent != 0 {
		t.Errorf("after u u: %+v unsent %d", a.state.react, a.unsent)
	}
	a.handle(key{r: 'u'})
	if a.msg != "取り消せる操作がありません" {
		t.Errorf("msg %q", a.msg)
	}
	if got := changes(t, st); got != "" {
		t.Errorf("the AI hears of what never was sent: %q", got)
	}
	// Only appended: n, y and the two undos.
	if evs := st.Events(); len(evs) != 5 || evs[2].Op != "undo" || evs[2].Undo != evs[1].N {
		t.Errorf("events %+v", evs)
	}
}

func TestUndoReactionSent(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	onHook(t, a)
	a.handle(key{r: 'y'})
	if got := changes(t, st); got != "y hook (hook を入れるか決める)" {
		t.Fatalf("changes %q", got)
	}
	a.loadEvents()
	a.handle(key{r: 'u'})
	if a.unsent != 1 {
		t.Errorf("unsent %d", a.unsent)
	}
	if r := rightOf(a, "hook"); r != "取消 未送信" {
		t.Errorf("right %q", r)
	}
	if got := changes(t, st); got != "retract y hook (hook を入れるか決める)" {
		t.Errorf("changes %q", got)
	}
	a.loadEvents()
	if r := rightOf(a, "hook"); r != "" {
		t.Errorf("right after sending %q", r)
	}
	// Sent but not read yet, then taken back and sent: the AI hears neither.
	a.handle(key{r: 'n'})
	sendAll(st)
	a.loadEvents()
	a.handle(key{r: 'u'})
	if got := changes(t, st); got != "" {
		t.Errorf("changes %q", got)
	}
}

func TestUndoAnswerAndHide(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.render()
	a.handle(key{name: "enter"})
	a.render()
	a.handle(key{r: 'a'})
	if e := a.state.react["unit"]; e.Value != "a" {
		t.Fatalf("answer %+v", e)
	}
	a.handle(key{r: 'u'})
	if _, ok := a.state.react["unit"]; ok || a.msg != "答え a を取り消しました: ページの単位" {
		t.Errorf("after u: %q", a.msg)
	}
	// Hide, sent and read; u shows it again and tells the AI.
	a.handle(key{r: 'x'})
	if !pv.isHidden(byID(pv.nodes, "unit")) {
		t.Fatal("not hidden")
	}
	if got := changes(t, st); got != "hide (1): ページの単位" {
		t.Fatalf("changes %q", got)
	}
	a.loadEvents()
	a.handle(key{r: 'u'})
	if pv.isHidden(byID(pv.nodes, "unit")) || a.msg != "隠したのを取り消しました: ページの単位" {
		t.Errorf("after u: hidden %v msg %q", pv.isHidden(byID(pv.nodes, "unit")), a.msg)
	}
	if got := changes(t, st); got != "retract hide: ページの単位" {
		t.Errorf("changes %q", got)
	}
}

func TestUndoComments(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.render()
	// Put, not sent: u and it never was.
	a.in = newInput("", "", "コメントを書く")
	a.write("one")
	a.handle(key{r: 'u'})
	if len(a.state.thoughts) != 0 || a.msg != "コメントを取り消しました: one" {
		t.Fatalf("thoughts %d msg %q", len(a.state.thoughts), a.msg)
	}
	// Put and read by the AI, then rewritten and read: u on the rewrite
	// tells the text before; u on the comment takes it back.
	a.in = newInput("", "", "コメントを書く")
	a.write("two")
	n := a.state.thoughts[0].N
	if got := changes(t, st); got != "thought "+itoa(n)+": two" {
		t.Fatalf("changes %q", got)
	}
	a.loadEvents()
	a.rewrite(n)
	a.write("two, rewritten")
	if got := changes(t, st); got != "edit thought "+itoa(n)+": two, rewritten" {
		t.Fatalf("changes %q", got)
	}
	a.loadEvents()
	a.handle(key{r: 'u'})
	if a.thoughtN(n).Text != "two" || a.msg != "コメントの直しを取り消しました: two, rewritten" {
		t.Errorf("after u: %q %q", a.thoughtN(n).Text, a.msg)
	}
	if w, _ := a.state.statusOf(a.thoughtN(n)); w != "未送信" {
		t.Errorf("status %q", w)
	}
	if got := changes(t, st); got != "edit thought "+itoa(n)+": two" {
		t.Errorf("changes %q", got)
	}
	a.loadEvents()
	a.handle(key{r: 'u'})
	if len(a.state.thoughts) != 0 {
		t.Error("the comment is still there")
	}
	if got := changes(t, st); got != "retract thought "+itoa(n)+": two" {
		t.Errorf("changes %q", got)
	}
	// u in the editor is a letter.
	a.loadEvents()
	a.in = newInput("", "", "コメントを書く")
	a.handle(key{r: 'u'})
	if a.in == nil || a.in.ed.String() != "u" {
		t.Error("u in the editor must be typed")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestUndoHint(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	onHook(t, a)
	if strings.Contains(hintLine(a), "u 取り消す") {
		t.Error("u shown with nothing to take back")
	}
	a.handle(key{r: 'y'})
	// Narrow: u goes first.
	if s := hintLine(a); strings.Contains(s, "u 取り消す") || !strings.Contains(s, "x 隠す") {
		t.Errorf("narrow hints %q", s)
	}
	a.handle(key{name: "tab"})
	if s := hintLine(a); !strings.Contains(s, "u 取り消す") {
		t.Errorf("hints %q", s)
	}
	a.handle(key{r: 'u'}) // works in the list too
	if _, ok := a.state.react["hook"]; ok {
		t.Error("u in the list did nothing")
	}
}

func TestOnlyChangedRowsAreWritten(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="a" ask="y">一</Item>
<Item id="b" look="y">二</Item>
<Item id="c">三</Item>
<Item id="d">四</Item>
`), 0o644)
	a := testApp(t, st)
	var out bytes.Buffer
	a.t.out = &out
	a.render()
	if !strings.Contains(out.String(), "\x1b[H") {
		t.Fatal("the first frame is not whole")
	}
	for _, k := range []rune{'j', 'j', 'k'} {
		out.Reset()
		a.handle(key{r: k})
		a.render()
		f := out.String()
		if strings.Contains(f, "\x1b[H") || strings.Contains(f, "\x1b[2J") {
			t.Errorf("%c: the whole screen was written", k)
		}
		// The two lines the cursor moved between, and perhaps the hints.
		if n := strings.Count(f, "\x1b[2K"); n > 3 {
			t.Errorf("%c: %d rows written:\n%q", k, n, f)
		}
	}
	// The marks stay in their column under the cursor and beside it, and
	// moving the cursor does not read a line.
	rows := shown(a)
	for _, want := range []string{"  ● ○ 一", "> ● ○ 二", "    ○ 三"} {
		if !strings.Contains(rows, want) {
			t.Errorf("want %q in\n%s", want, rows)
		}
	}
	// A resize writes everything again.
	a.t.resize()
	out.Reset()
	a.render()
	if !strings.Contains(out.String(), "\x1b[H") {
		t.Error("not whole after a resize")
	}
}
