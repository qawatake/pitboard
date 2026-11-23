package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

// ---- j / k into the thoughts pane ----

func TestPaneByJK(t *testing.T) {
	st := testStore(t)
	for i := 1; i <= 4; i++ {
		st.Append(board.Event{Op: "thought", Text: fmt.Sprintf("思いつき%d\n二行目", i)})
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
	if s := hintLine(a); s != "      enter 開く  e 直す  x 取り消す  s 送る  m 書く  ? キー" {
		t.Errorf("pane hints %q", s)
	}
	p := plainLines(a.pane(30))
	for _, w := range []string{">   ▸ 思いつき4", "思いつき3", "思いつき2", "思いつき1"} {
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
	if a.pcur != 3 || !strings.Contains(p, "▾ 思いつき3 二行目") || !strings.Contains(p, "まだ送っていません (s で送る)") {
		t.Errorf("enter opens the thought:\n%s", p)
	}
	if s := hintLine(a); s != "      enter 閉じる  e 直す  x 取り消す  s 送る  m 書く  ? キー" {
		t.Errorf("open thought hints %q", s)
	}
	// Nothing is recorded on a thought (x takes it back: tested apart).
	for _, r := range "yncHa" {
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
		st.Append(board.Event{Op: "thought", Text: fmt.Sprintf("t%02d", i)})
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
	if len(p) > a.t.h/2 || !strings.Contains(plainLines(p), ">   ▸ t20") {
		t.Errorf("open pane on a low screen (%d lines):\n%s", len(p), plainLines(p))
	}
	for range 15 {
		a.handle(key{r: 'j'})
	}
	a.render()
	p = a.pane(14)
	if !strings.Contains(plainLines(p), ">   ▸ t05") || strings.Contains(plainLines(p), "t20") || len(p) > a.t.h/2 {
		t.Errorf("scrolled pane:\n%s", plainLines(p))
	}
	ls, _ := screen(t, a)
	if !strings.Contains(strings.Join(ls, "\n"), ">   ▸ t05") {
		t.Errorf("the cursor's thought must be on screen:\n%s", strings.Join(ls, "\n"))
	}
}

func TestPaneAfterTextWithoutStops(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	for i := range 40 {
		fmt.Fprintf(&b, "段落 %d\n\n", i)
	}
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	st.Append(board.Event{Op: "thought", Text: "x"})
	a := testApp(t, st)
	a.render()
	presses := 0
	for !a.inPane && presses < 200 {
		a.handle(key{r: 'j'})
		a.render()
		presses++
	}
	if !a.inPane || presses < 3 {
		t.Errorf("j must scroll the text to its end, then enter the pane (%d presses)", presses)
	}
	if ls, _ := screen(t, a); !strings.Contains(strings.Join(ls, "\n"), "段落 39") {
		t.Error("the end of the text must have been shown")
	}
}

func TestRetractInPaneAndUndo(t *testing.T) {
	st := testStore(t)
	for _, s := range []string{"one", "two", "three"} {
		st.Append(board.Event{Op: "thought", Text: s})
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
	if s := hintLine(a); s != "      enter 開く  e 直す  x 取り消す  s 送る  m 書く  ? キー" {
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
	if len(a.b.State.Thoughts) != 2 {
		t.Fatal("x pressed twice took back the next thought")
	}
	// u takes it back, even after other keys; the cursor stays.
	a.handle(key{r: 'k'})
	a.handle(key{r: 'u'})
	if len(a.b.State.Thoughts) != 3 || a.pcur != 2 || a.msg != "取り消したコメントを戻しました: three" {
		t.Errorf("u must undo: thoughts %d pcur %d msg %q", len(a.b.State.Thoughts), a.pcur, a.msg)
	}
	evs := st.Events()
	if e := evs[len(evs)-1]; e.Op != "undo" || e.Undo != evs[len(evs)-2].N {
		t.Errorf("last event %+v", e)
	}
	a.handle(key{r: 'u'})
	if a.msg != "取り消せる操作がありません" {
		t.Errorf("nothing left: %q", a.msg)
	}
	// ✎ on the line goes too.
	th, _ := st.Append(board.Event{Op: "thought", ID: "hook", Text: "on hook"})
	a.loadEvents()
	if c := len(a.b.State.On("hook")); c != 1 {
		t.Fatalf("on hook %d", c)
	}
	st.Append(board.Event{Op: "retract", Thought: th.N})
	a.loadEvents()
	if c := len(a.b.State.On("hook")); c != 0 {
		t.Errorf("✎ still counts a taken-back thought")
	}
}

func TestRetractInList(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "one"})
	a := testApp(t, st)
	a.handle(key{name: "tab"})
	a.render()
	if s := hintLine(a); s != "      enter 開く  e 直す  x 取り消す  s 送る  m 書く  ? キー" {
		t.Errorf("list hints %q", s)
	}
	a.handle(key{r: 'x'})
	a.render()
	if len(a.b.State.Thoughts) != 0 {
		t.Fatal("x in the list must take the thought back")
	}
	msg := a.msg
	a.msg = ""
	if s := hintLine(a); !strings.Contains(s, "tab 報告へ") {
		t.Errorf("empty list hints %q", s)
	}
	a.msg = msg
	a.handle(key{r: 'u'})
	if len(a.b.State.Thoughts) != 1 || a.lcur != 1 {
		t.Errorf("u in the list: %d lcur %d", len(a.b.State.Thoughts), a.lcur)
	}
}

// ---- the thoughts pane ----

func TestThoughtsPane(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	if p := a.pane(30); p != nil {
		t.Errorf("no thoughts, no pane: %q", p)
	}
	for i, txt := range []string{"一\n二", "settled one", "three", "four", "five", "six"} {
		e := board.Event{Op: "thought", Text: txt}
		if i == 0 {
			e.ID, e.Title = "hook", "hook を入れるか決める"
		}
		st.Append(e)
	}
	sendAll(st)
	st.Changes() // thoughts 1-6 received
	st.Settle(2, "todo にした")
	st.Append(board.Event{Op: "thought", Text: "new"}) // 9, not sent
	a.loadEvents()
	var p []string
	for _, l := range a.pane(30) {
		p = append(p, reEsc.ReplaceAllString(l, ""))
	}
	s := strings.Join(p, "\n")
	for _, w := range []string{"コメント", "new", "six", "five", "既読"} {
		if !strings.Contains(s, w) {
			t.Errorf("pane lacks %q:\n%s", w, s)
		}
	}
	for _, w := range []string{"行き先待ち", "ほか", "✎", "←", "→", "four"} {
		if strings.Contains(s, w) {
			t.Errorf("pane has %q:\n%s", w, s)
		}
	}
	if len(p) != 6 { // blank, heading, 3 thoughts, blank
		t.Errorf("pane has %d lines:\n%s", len(p), s)
	}
	// Screens under 20 rows: the heading only.
	a.t.h = 19
	if p := a.pane(17); len(p) != 1 {
		t.Errorf("low screen pane: %q", p)
	}
	// The settled one comes after those waiting, with its destination.
	a.t.h = 40
	order := a.paneOrder()
	if order[0].N != 9 || order[len(order)-1].N != 2 {
		t.Errorf("order: first %d last %d", order[0].N, order[len(order)-1].N)
	}
	if w, _ := a.statusOf(order[len(order)-1]); w != "対応済" {
		t.Errorf("settled status %q", w)
	}
	// The report keeps 6 rows: with 10 rows above the hints, one thought.
	if p := a.pane(10); len(p) != 4 {
		t.Errorf("pane in 10 rows has %d lines", len(p))
	}
}

func TestThoughtShowsTheTitleItWasWrittenOn(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", ID: "hook", Text: "BBB について", Title: "hook を入れるか決める"})
	os.WriteFile(st.ReportPath(), []byte(`<Item id="hook">まったく別の物</Item>`), 0o644)
	a := testApp(t, st)
	pv := a.top()
	pv.open["hook"] = true
	var screen []string
	for _, r := range a.rows(pv) {
		screen = append(screen, reEsc.ReplaceAllString(r.line, ""))
	}
	if s := strings.Join(screen, "\n"); !strings.Contains(s, "(書いたときは: hook を入れるか決める)") {
		t.Errorf("under the line:\n%s", s)
	}
	a.lopen[1] = true
	screen = nil
	for _, r := range a.listRows() {
		screen = append(screen, reEsc.ReplaceAllString(r.line, ""))
	}
	if s := strings.Join(screen, "\n"); !strings.Contains(s, "行: まったく別の物 (書いたときは: hook を入れるか決める)") {
		t.Errorf("in the list:\n%s", s)
	}
}

// 16: the end of the report under the comments is said.
func TestMoreUnderTheComments(t *testing.T) {
	st := testStore(t)
	src := ""
	for i := range 40 {
		src += fmt.Sprintf("<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.ReportPath(), []byte(src), 0o644)
	st.Append(board.Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	ls, _ := screen(t, a)
	if !strings.Contains(strings.Join(ls, "\n"), "↓ あと") {
		t.Errorf("no sign of more:\n%s", strings.Join(ls, "\n"))
	}
}
