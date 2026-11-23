package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func plainLines(ls []string) string {
	var out []string
	for _, l := range ls {
		out = append(out, reEsc.ReplaceAllString(l, ""))
	}
	return strings.Join(out, "\n")
}

func hintLine(a *app) string { return plainLines(a.bottom()) }

// ---- j / k into the thoughts pane ----

func TestPaneByJK(t *testing.T) {
	st := testStore(t)
	for i := 1; i <= 4; i++ {
		st.Append(Event{Op: "thought", Text: fmt.Sprintf("思いつき%d\n二行目", i)})
	}
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'}) // the Item, the last stop
	a.render()
	if a.inPane || pv.cur != "hook" {
		t.Fatalf("cursor %q inPane %v", pv.cur, a.inPane)
	}
	a.handle(key{r: 'j'})
	a.render()
	if !a.inPane || a.pcur != 4 {
		t.Fatalf("j past the end must go into the pane: inPane %v pcur %d", a.inPane, a.pcur)
	}
	if s := hintLine(a); s != "    enter 開く  m 書く  ? キー" {
		t.Errorf("pane hints %q", s)
	}
	p := plainLines(a.pane(30))
	for _, w := range []string{"> ▸ 思いつき4", "思いつき3", "思いつき2", "思いつき1"} {
		if !strings.Contains(p, w) {
			t.Errorf("open pane lacks %q:\n%s", w, p)
		}
	}
	if s := shown(a); strings.Contains(s, ">") {
		t.Errorf("the report must not show its cursor while in the pane:\n%s", s)
	}
	a.handle(key{r: 'j'})
	a.handle(key{name: "enter"})
	a.render()
	p = plainLines(a.pane(30))
	if a.pcur != 3 || !strings.Contains(p, "▾ 思いつき3 二行目") || !strings.Contains(p, "まだ AI に届いていません") {
		t.Errorf("enter opens the thought:\n%s", p)
	}
	if s := hintLine(a); s != "    enter 閉じる  m 書く  ? キー" {
		t.Errorf("open thought hints %q", s)
	}
	// Nothing is recorded on a thought.
	for _, r := range "ynxcHa" {
		a.handle(key{r: r})
	}
	if n := len(st.Events()); n != 4 {
		t.Errorf("keys recorded something in the pane: %d events", n)
	}
	// k back to the first, then out to the report's last stop.
	a.handle(key{r: 'k'})
	a.handle(key{r: 'k'})
	a.render()
	if a.inPane || pv.cur != "hook" || !pv.curShown {
		t.Fatalf("k from the first thought: inPane %v cur %q", a.inPane, pv.cur)
	}
	a.handle(key{r: 'y'}) // the person pressed k to get here: it works
	if evs := st.Events(); len(evs) != 5 || evs[4].Value != "y" {
		t.Errorf("y after coming back: %+v", evs[len(evs)-1])
	}
	// Back to three.
	if p := plainLines(a.pane(30)); strings.Contains(p, "思いつき1") {
		t.Errorf("the pane must shrink back:\n%s", p)
	}
}

func TestPaneScrollsAndLowScreen(t *testing.T) {
	st := testStore(t)
	for i := 1; i <= 20; i++ {
		st.Append(Event{Op: "thought", Text: fmt.Sprintf("t%02d", i)})
	}
	a := testApp(t, st)
	a.t.h = 14
	a.render()
	if p := plainLines(a.pane(14)); strings.Contains(p, "t20") {
		t.Errorf("a low screen shows the heading only:\n%s", p)
	}
	a.handle(key{r: 'j'})
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	if !a.inPane {
		t.Fatal("j must enter the pane on a low screen too")
	}
	p := a.pane(14)
	if len(p) > a.t.h/2 || !strings.Contains(plainLines(p), "> ▸ t20") {
		t.Errorf("open pane on a low screen (%d lines):\n%s", len(p), plainLines(p))
	}
	for range 15 {
		a.handle(key{r: 'j'})
	}
	a.render()
	p = a.pane(14)
	if !strings.Contains(plainLines(p), "> ▸ t05") || strings.Contains(plainLines(p), "t20") || len(p) > a.t.h/2 {
		t.Errorf("scrolled pane:\n%s", plainLines(p))
	}
	ls, _ := screen(t, a)
	if !strings.Contains(strings.Join(ls, "\n"), "> ▸ t05") {
		t.Errorf("the cursor's thought must be on screen:\n%s", strings.Join(ls, "\n"))
	}
}

func TestPaneAfterTextWithoutStops(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	for i := range 40 {
		fmt.Fprintf(&b, "段落 %d\n\n", i)
	}
	os.WriteFile(st.reportPath(), []byte(b.String()), 0o644)
	st.Append(Event{Op: "thought", Text: "x"})
	a := testApp(t, st)
	a.render()
	presses := 0
	for !a.inPane && presses < 200 {
		a.handle(key{r: 'j'})
		a.render()
		presses++
	}
	if !a.inPane || presses < 10 {
		t.Errorf("j must scroll the text to its end, then enter the pane (%d presses)", presses)
	}
	if ls, _ := screen(t, a); !strings.Contains(strings.Join(ls, "\n"), "段落 39") {
		t.Error("the end of the text must have been shown")
	}
}

// ---- one-line messages and hints ----

func TestMessagesAreOneLine(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.t.w = 50
	a.msg = "送れませんでした: PITBOARD_SEND: exit status 2: " + strings.Repeat("とても長いエラー ", 10)
	b := a.bottom()
	if len(b) != 1 || !strings.Contains(plainLines(b), "…") || visibleWidth(b[0]) > margin+a.textWidth() {
		t.Errorf("message: %q", plainLines(b))
	}
}

func TestEditorHintsKeepEsc(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.in = &input{label: "x"}
	for _, c := range []struct {
		w    int
		want string
	}{
		{80, "    enter 書く  ctrl+s 送る  ctrl+j 改行  ctrl+e エディタ  esc やめる"},
		{60, "    enter 書く  ctrl+s 送る  ctrl+j 改行  esc やめる"},
		{50, "    enter 書く  ctrl+s 送る  esc やめる"},
	} {
		a.t.w = c.w
		if s := plainLines(a.editorHints()); s != c.want {
			t.Errorf("width %d: %q, want %q", c.w, s, c.want)
		}
	}
}
