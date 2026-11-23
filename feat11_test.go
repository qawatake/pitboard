package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWaitAttribute(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<Item id="a" wait="you">x</Item>`, ""},
		{`<Item id="a" wait="me">x</Item>`, `wait "me"`},
		{`<Item id="a" state="done" wait="you">x</Item>`, "waits for nobody"},
		{`<Link id="a" path="p" wait="you">x</Link>`, "no attribute"},
	} {
		_, probs, _ := parseReport(c.src)
		got := ""
		for _, p := range probs {
			got += p.Msg
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q (want %q)", c.src, got, c.want)
		}
	}
}

const colorReport = `<Item id="you" wait="you" status="あなた待ち">決めてほしい</Item>
<Item id="run" state="doing" status="作業中">動いている</Item>
<Item id="queue" status="順番待ち">待っている</Item>
<Item id="old" status="あなた待ち">語だけ</Item>
<Item id="plan" ask="y">聞いている</Item>
`

func TestStatusColors(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(colorReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	for id, color := range map[string]string{"you": yellow, "run": green, "queue": dim, "old": dim} {
		n := byID(pv.nodes, id)
		r := a.right(pv, n)
		if len(r) == 0 || r[0].st != color {
			t.Errorf("%s: status color %q, want %q", id, r, color)
		}
	}
	// wait="you" is not "waiting", but space goes to it after the waiting.
	if w, _ := a.counts(pv); w != 1 {
		t.Errorf("waiting %d (only ask=\"y\")", w)
	}
	a.render()
	a.handle(key{r: ' '})
	if pv.cur != "plan" {
		t.Fatalf("first space: %q", pv.cur)
	}
	st.Append(Event{Op: "react", ID: "plan", Value: "y", Line: byID(pv.nodes, "plan").sig()})
	a.loadEvents()
	a.viewAll(pv)
	a.handle(key{r: ' '})
	if pv.cur != "you" {
		t.Errorf("second space: %q, want the wait=\"you\" Item before anything new", pv.cur)
	}
}

// rightOf is what a line shows at its right end, without styles.
func rightOf(a *app, id string) string {
	pv := a.top()
	return a.right(pv, byID(pv.nodes, id)).plain()
}

func TestInputStateOnTheLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="hook" status="作業中" state="doing">hook を入れるか決める</Item>`), 0o644)
	a := testApp(t, st)
	a.render()
	steps := []struct {
		do   func()
		want string
	}{
		{func() { st.Append(Event{Op: "react", ID: "hook", Value: "y", Line: byID(a.top().nodes, "hook").sig()}) }, "y 未送信  作業中"},
		{func() { sendAll(st) }, "y 送信済  作業中"},
		{func() { st.Changes() }, "y 既読  作業中"},
		{func() { st.Append(Event{Op: "thought", ID: "hook", Text: "一つ目"}) }, "y 未送信  作業中"},
		{func() { st.Append(Event{Op: "thought", ID: "hook", Text: "二つ目"}) }, "y 2 未送信  作業中"},
		{func() { sendAll(st) }, "y 2 送信済  作業中"},
		{func() { st.Changes() }, "y 2 既読  作業中"},
		{func() { st.SettleAll([]settlement{{4, "doing", "やっています"}}) }, "y 2 対応中  作業中"},
		{func() { st.SettleAll([]settlement{{4, "hold", ""}}) }, "y 2 保留  作業中"},
		{func() { st.SettleAll([]settlement{{4, "", "やりました"}}) }, "y 2 対応済  作業中"},
	}
	for i, s := range steps {
		s.do()
		a.loadEvents()
		if got := rightOf(a, "hook"); got != s.want {
			t.Errorf("step %d: %q, want %q", i, got, s.want)
		}
	}
	// The comment's state in the list too, and what the AI wrote.
	a.handle(key{name: "tab"})
	a.lcur = 4
	a.lopen[4] = true
	rows := ""
	for _, r := range a.listRows() {
		rows += reEsc.ReplaceAllString(r.line, "") + "\n"
	}
	if !strings.Contains(rows, "対応済") || !strings.Contains(rows, "対応: やりました") {
		t.Errorf("list:\n%s", rows)
	}
	// Nothing on a line without input; no ✎.
	if strings.Contains(rows, "✎") {
		t.Errorf("✎ is gone:\n%s", rows)
	}
}

func TestSettleStates(t *testing.T) {
	st := testStore(t)
	for range 3 {
		st.Append(Event{Op: "thought", Text: "t"})
	}
	t.Chdir(st.base)
	var out bytes.Buffer
	for _, args := range [][]string{
		{"settle", "1", "--doing", "調べています"},
		{"settle", "--hold", "2"},
		{"settle", "--doing", "3", "a", "--done", "1", "済んだ"},
	} {
		if err := run(append([]string{"-s", "s1"}, args...), &out); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}
	s := replayEvents(st.Events(), 0)
	var got []string
	for _, th := range s.thoughts {
		w, _ := s.statusOf(th)
		got = append(got, w+":"+th.settled)
	}
	if strings.Join(got, "|") != "対応済:済んだ|保留:|対応中:a" {
		t.Errorf("states %q", got)
	}
	if err := run([]string{"-s", "s1", "settle", "2", "--done"}, &out); err == nil {
		t.Error("done without a text was accepted")
	}
}

func TestChangesInEnglish(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "gone", Value: "y", Title: "消えた行"})
	st.Append(Event{Op: "react", ID: "hook", Value: "n", Title: "前の題"})
	sendAll(st)
	lines, _ := st.Changes()
	want := "y gone (消えた行 → now gone)\nn hook (前の題 → now: hook を入れるか決める)"
	if strings.Join(lines, "\n") != want {
		t.Errorf("got\n%s", strings.Join(lines, "\n"))
	}
}

// Every word on screen follows the word list.
func TestNoOldWords(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", ID: "hook", Text: "x"})
	a := testApp(t, st)
	a.render()
	var all []string
	for _, r := range a.helpRows() {
		all = append(all, r.line)
	}
	a.in = newInput("", "", "コメントを書く")
	all = append(all, a.editorLines()...)
	a.in = nil
	a.handle(key{name: "tab"})
	a.lopen[1] = true
	for _, r := range a.listRows() {
		all = append(all, r.line)
	}
	// The screens without a report, and the editor's file.
	all = append(all, msgChanged, msgMoved, msgNoReport, msgNoBoard, msgNoSession)
	for _, setup := range []func(b *app){
		func(b *app) { b.setStore(nil) },
		func(b *app) { os.Mkdir(filepath.Join(b.base, ".pitboard"), 0o755); b.setStore(nil) },
		func(b *app) { b.setStore(newStore(b.base, "nosuch")) },
	} {
		b := &app{t: &term{w: 120, h: 30, out: io.Discard}, base: t.TempDir(), lopen: map[int]bool{}}
		setup(b)
		all = append(all, shown(b))
	}
	s := reEsc.ReplaceAllString(strings.Join(all, "\n"), "")
	if !strings.Contains(s, "セッション nosuch") {
		t.Errorf("the missing session's screen was not drawn:\n%s", s)
	}
	for _, old := range []string{"板", "思いつき", "送信済み", "新着", "受け取った", "行き先", "ページ", "j k ↑↓  送る", "半画面ずつ送る", "ターミナル"} {
		if strings.Contains(s, old) {
			t.Errorf("old word %q on screen:\n%s", old, s)
		}
	}
}
