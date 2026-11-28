package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

func listText(a *app) string {
	var out []string
	for _, r := range a.listRows() {
		out = append(out, reEsc.ReplaceAllString(r.line, ""))
	}
	return strings.Join(out, "\n")
}

// ---- l: the report, the comments, the report ----

func TestLCyclesTheScreens(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", ID: "hook", Text: "コメント一"})
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	a.handle(key{name: "enter"})
	a.render()
	if pv.cur != "hook" || !pv.open["hook"] {
		t.Fatalf("cursor %q open %v", pv.cur, pv.open)
	}
	// The comments are not under the report any more.
	if s := shown(a); strings.Count(s, "コメント一") > 1 || strings.Contains(s, "↓ あと") {
		t.Errorf("the report shows the comments under it:\n%s", s)
	}
	for i, k := range []key{{r: 'l'}, {r: 'l'}, {name: "tab"}, {name: "tab"}, {r: 'l'}, {r: 'q'}} {
		quit := a.handle(k)
		a.render()
		want := []int{viewComments, viewReport, viewComments, viewReport, viewComments, viewReport}[i]
		if quit || a.view != want {
			t.Fatalf("key %d %+v: quit %v view %d, want %d", i, k, quit, a.view, want)
		}
	}
	if pv.cur != "hook" || !pv.open["hook"] || !pv.curShown {
		t.Errorf("back on the report: cursor %q open %v shown %v", pv.cur, pv.open, pv.curShown)
	}
	// j at the end of the report stays on the report.
	for range 5 {
		a.handle(key{r: 'j'})
		a.render()
	}
	if a.view != viewReport {
		t.Error("j past the end left the report")
	}
	// q on the report quits.
	if !a.handle(key{r: 'q'}) {
		t.Error("q on the report did not quit")
	}
}

func TestCommentsKeyOnTheReport(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.render()
	if s := hintLine(a); strings.Contains(s, "l コメント") {
		t.Errorf("no comments, no l: %q", s)
	}
	st.Append(board.Event{Op: "thought", Text: "一"})
	st.Append(board.Event{Op: "thought", Text: "二"})
	sendAll(st)
	st.Changes()
	st.Settle(1, "やった")
	a.loadEvents()
	a.render()
	// The count is of those the AI has not finished (the browser's コメント N).
	if s := hintLine(a); !strings.Contains(s, "m 書く  l コメント 1  ? キー") {
		t.Errorf("report hints %q", s)
	}
	// Narrow: it goes with m, before "? キー".
	a.t.w = 30
	if s := hintLine(a); strings.Contains(s, "l コメント") || !strings.Contains(s, "? キー") {
		t.Errorf("narrow report hints %q", s)
	}
}

// ---- the comments screen ----

func TestCommentsScreen(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	if s := listText(a); !strings.Contains(s, "コメントはまだありません") {
		t.Errorf("no comments:\n%s", s)
	}
	st.Append(board.Event{Op: "thought", ID: "hook", Title: "hook を入れるか決める", Text: "hook は Stop でいい？"})      // 1
	st.Append(board.Event{Op: "thought", Text: "画面の色は後で"})                                                   // 2
	st.Append(board.Event{Op: "thought", ID: "gone", Title: "消えた行", Text: strings.Repeat("とても長いコメント ", 30)}) // 3
	sendAll(st)
	st.Changes()
	st.SettleAll([]board.Settlement{{N: 2, State: "", Text: "あとで決めることにした\n二行目"}})
	last, _ := st.Append(board.Event{Op: "thought", ID: "unit", Text: "aqua はどうしているか"}) // not sent
	a.loadEvents()
	a.handle(key{r: 'l'})
	ls, _ := screen(t, a)
	s := strings.Join(ls, "\n")
	// No heading: the comments start under the top row.
	if strings.Contains(s, "コメント 3") || !strings.Contains(ls[1], "aqua") {
		t.Errorf("heading:\n%s", s)
	}
	// Pending order: not finished (newest first), then a rule, then finished.
	order := []string{fmt.Sprintf("未送信  %d  aqua", last.N), " はどうしているか", "← ページの単位", "3  とても長いコメント", "← (報告にもうない行) (書いたときは: 消えた行)",
		"既読    1  hook は Stop でいい？", "← hook を入れるか決める", "─ 済んだもの ─", "対応済  2  画面の色は後で", "(宛先なし)   対応: あとで決めることにした"}
	at := -1
	for _, w := range order {
		i := strings.Index(s, w)
		if i < 0 || i < at {
			t.Fatalf("%q missing or out of order:\n%s", w, s)
		}
		at = i
	}
	if strings.Contains(s, "二行目") {
		t.Errorf("only the first line of what the AI wrote:\n%s", s)
	}
	// The cursor on the first; a long text takes two rows at most.
	if r := rowOf(ls, "とても長いコメント"); r < 0 || !strings.Contains(ls[r+1], "とても長いコメント") || !strings.HasSuffix(strings.TrimSpace(ls[r+1]), "…") || !strings.Contains(ls[r+2], "←") {
		t.Errorf("a long comment:\n%s", s)
	}
	if r := rowOf(ls, "aqua"); r < 0 || !strings.HasPrefix(ls[r], ">") {
		t.Errorf("cursor:\n%s", s)
	}
	// Its continuation rows start where the text starts.
	if r := rowOf(ls, "とても長いコメント"); r >= 0 && width(ls[r+1]) - width(strings.TrimLeft(ls[r+1], " ")) != width(ls[r][:strings.Index(ls[r], "とても")]) {
		t.Errorf("not aligned:\n%s", s)
	}
	if s := hintLine(a); s != "      enter 開く  g 行へ  e 直す  x 取り消す  l 報告へ  s 送る  m 書く  ? キー" {
		t.Errorf("hints %q", s)
	}
	// enter opens the whole text and what the AI wrote in full.
	for _, k := range "jjj" {
		a.handle(key{r: k})
		a.render()
	}
	if a.lcur != 2 {
		t.Fatalf("lcur %d", a.lcur)
	}
	if s := hintLine(a); s != "      enter 開く  e 直す  x 取り消す  l 報告へ  s 送る  m 書く  ? キー" {
		t.Errorf("no line, no g: %q", s)
	}
	a.handle(key{name: "enter"})
	a.render()
	if s := listText(a); !strings.Contains(s, "対応: あとで決めることにした") || !strings.Contains(s, "二行目") || a.view != viewComments {
		t.Errorf("enter:\n%s", s)
	}
	if s := hintLine(a); !strings.Contains(s, "enter 閉じる") {
		t.Errorf("open hints %q", s)
	}
	// Nothing is recorded on a comment by other letters.
	n := len(st.Events())
	for _, r := range "ycHao" {
		a.handle(key{r: r})
	}
	if len(st.Events()) != n {
		t.Errorf("keys recorded something on the comments screen")
	}
}

// g goes to the comment's line in the report: the Folds around it open,
// the cursor on it. A comment on no line, a line gone, or a hidden one
// stays, and says why.
func TestGGoesToTheCommentsLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="top">上の行</Item>
<Fold title="中">
<Item id="deep">奥の行</Item>
</Fold>
<Item id="bye">消す行</Item>
`), 0o644)
	st.Append(board.Event{Op: "thought", ID: "deep", Text: "奥へ"})    // 1
	st.Append(board.Event{Op: "thought", Text: "どこでもない"})            // 2
	st.Append(board.Event{Op: "thought", ID: "nosuch", Text: "消えた"}) // 3
	st.Append(board.Event{Op: "thought", ID: "bye", Text: "隠す行へ"})   // 4
	a := testApp(t, st)
	pv := a.top()
	a.render()
	// Hide the last line.
	for range 3 {
		a.handle(key{r: 'j'})
		a.render()
	}
	if pv.cur == "" || pv.nodeAt(pv.cur).ID != "bye" {
		t.Fatalf("cursor %q", pv.cur)
	}
	a.handle(key{r: 'x'})
	a.render()
	wait(t, settleDelay*2)
	a.msg = ""
	a.handle(key{r: 'l'})
	a.render()
	tries := []struct {
		n   int
		msg string
	}{{4, page.MsgLinkHidden}, {3, page.MsgGoGone}, {2, page.MsgGoNoLine}}
	for _, c := range tries {
		a.lcur, a.lfollow = c.n, true
		a.render()
		if strings.Contains(hintLine(a), "g 行へ") {
			t.Errorf("%d: g offered: %q", c.n, hintLine(a))
		}
		a.handle(key{r: 'g'})
		a.render()
		if a.view != viewComments || a.msg != c.msg {
			t.Errorf("%d: view %d msg %q, want %q", c.n, a.view, a.msg, c.msg)
		}
		// Said as a refusal.
		if _, ok := refusalWord(c.msg); !ok {
			t.Errorf("%q is not among page.Refusals", c.msg)
		}
	}
	a.lcur, a.lfollow = 1, true
	a.render()
	a.handle(key{r: 'g'})
	a.render()
	if a.view != viewReport || pv.nodeAt(pv.cur) == nil || pv.nodeAt(pv.cur).ID != "deep" || !pv.curShown {
		t.Fatalf("g: view %d cursor %q shown %v", a.view, pv.cur, pv.curShown)
	}
	if s := shown(a); !strings.Contains(s, "> ") || !strings.Contains(s, "奥の行") {
		t.Errorf("the Fold did not open:\n%s", s)
	}
	// g - goes back where it was.
	if len(pv.back) != 1 {
		t.Errorf("back %v", pv.back)
	}
}

func refusalWord(msg string) (string, bool) {
	for k, v := range page.Refusals {
		if v == msg {
			return k, true
		}
	}
	return "", false
}

func TestRetractOnTheCommentsScreen(t *testing.T) {
	st := testStore(t)
	for _, s := range []string{"one", "two", "three"} {
		st.Append(board.Event{Op: "thought", Text: s})
	}
	a := testApp(t, st)
	a.handle(key{r: 'l'})
	a.render()
	if a.lcur != 3 {
		t.Fatalf("lcur %d", a.lcur)
	}
	a.handle(key{r: 'x'})
	if a.msg != "取り消しました (u で戻す)" || a.lcur != 2 {
		t.Fatalf("msg %q lcur %d", a.msg, a.lcur)
	}
	a.render()
	if s := listText(a); strings.Contains(s, "three") {
		t.Errorf("taken back, still shown:\n%s", s)
	}
	// x again at once: the viewer moved the cursor, so it waits.
	a.handle(key{r: 'x'})
	if len(a.b.State.Thoughts) != 2 {
		t.Fatal("x pressed twice took back the next comment")
	}
	a.handle(key{r: 'u'})
	if len(a.b.State.Thoughts) != 3 || a.msg != "取り消したコメントを戻しました: three" {
		t.Errorf("u must undo: comments %d msg %q", len(a.b.State.Thoughts), a.msg)
	}
	// All taken back: the screen says so, and l goes back.
	for range 3 {
		wait(t, settleDelay*2)
		a.render()
		a.handle(key{r: 'x'})
		a.render()
	}
	if len(a.b.State.Thoughts) != 0 {
		t.Fatalf("comments left: %d", len(a.b.State.Thoughts))
	}
	a.msg = ""
	if s := hintLine(a); !strings.HasPrefix(s, "      l 報告へ") {
		t.Errorf("empty screen hints %q", s)
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
	if s := listText(a); !strings.Contains(s, "← まったく別の物 (書いたときは: hook を入れるか決める)") {
		t.Errorf("on the comments screen:\n%s", s)
	}
	a.lopen[1] = true
	if s := listText(a); !strings.Contains(s, "行: まったく別の物 (書いたときは: hook を入れるか決める)") {
		t.Errorf("opened:\n%s", s)
	}
}

// The comments screen scrolls to keep the cursor's comment in view.
func TestCommentsScreenScrolls(t *testing.T) {
	st := testStore(t)
	for i := 1; i <= 20; i++ {
		st.Append(board.Event{Op: "thought", Text: fmt.Sprintf("t%02d", i)})
	}
	a := testApp(t, st)
	a.t.h = 14
	a.handle(key{r: 'l'})
	a.render()
	for range 15 {
		a.handle(key{r: 'j'})
		a.render()
	}
	ls, _ := screen(t, a)
	s := strings.Join(ls, "\n")
	if r := rowOf(ls, "t05"); r < 0 || !strings.HasPrefix(ls[r], ">") || strings.Contains(s, "t20") {
		t.Errorf("scrolled:\n%s", s)
	}
}

// g reaches a Fold without an id: its comments go by its title.
func TestGGoesToAFoldWithoutID(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"top\">上の行</Item>\n<Fold title=\"外\">\n<Fold title=\"題なしの内\">\n<Item id=\"deep\">奥</Item>\n</Fold>\n</Fold>\n"), 0o644)
	st.Append(board.Event{Op: "thought", ID: "Fold|title=題なしの内", Title: "題なしの内", Text: "内へ"})
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'l'})
	a.render()
	if s := listText(a); !strings.Contains(s, "← 題なしの内") {
		t.Fatalf("comments screen:\n%s", s)
	}
	if s := hintLine(a); !strings.Contains(s, "g 行へ") {
		t.Errorf("hints %q", s)
	}
	a.handle(key{r: 'g'})
	a.render()
	if n := pv.nodeAt(pv.cur); a.view != viewReport || n == nil || n.Text != "題なしの内" || !pv.curShown {
		t.Fatalf("g: view %d cursor %q msg %q", a.view, pv.cur, a.msg)
	}
}

// At 80 columns x stays when a comment is opened (enter 閉じる is wider).
func TestRetractKeyStaysWhenOpened(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", ID: "hook", Text: "一"})
	a := testApp(t, st)
	a.render()
	a.handle(key{r: 'l'})
	a.render()
	if a.t.w != 80 {
		t.Fatalf("width %d", a.t.w)
	}
	for _, open := range []bool{false, true} {
		if open {
			a.handle(key{name: "enter"})
			a.render()
		}
		if s := hintLine(a); !strings.Contains(s, "x 取り消す") || !strings.Contains(s, "l 報告へ") {
			t.Errorf("open %v: hints %q", open, s)
		}
	}
}
