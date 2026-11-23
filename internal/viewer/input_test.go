package viewer

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

func TestInputStateOnTheLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="hook" status="作業中" state="doing">hook を入れるか決める</Item>`), 0o644)
	a := testApp(t, st)
	a.render()
	steps := []struct {
		do   func()
		want string
	}{
		{func() {
			st.Append(board.Event{Op: "react", ID: "hook", Value: "y", Line: report.ByID(a.top().nodes, "hook").Sig()})
		}, "y 未送信  作業中"},
		{func() { sendAll(st) }, "y 送信済  作業中"},
		{func() { st.Changes() }, "y 既読  作業中"},
		{func() { st.Append(board.Event{Op: "thought", ID: "hook", Text: "一つ目"}) }, "y 未送信  作業中"},
		{func() { st.Append(board.Event{Op: "thought", ID: "hook", Text: "二つ目"}) }, "y 2 未送信  作業中"},
		{func() { sendAll(st) }, "y 2 送信済  作業中"},
		{func() { st.Changes() }, "y 2 既読  作業中"},
		{func() { st.SettleAll([]board.Settlement{{N: 4, State: "doing", Text: "やっています"}}) }, "y 2 対応中  作業中"},
		{func() { st.SettleAll([]board.Settlement{{N: 4, State: "hold", Text: ""}}) }, "y 2 保留  作業中"},
		{func() { st.SettleAll([]board.Settlement{{N: 4, State: "", Text: "やりました"}}) }, "y 2 対応済  作業中"},
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

func longReport(n int) string {
	var b strings.Builder
	b.WriteString("<Title>題</Title>\n\n<Section title=\"s\">\n")
	for i := range n {
		fmt.Fprintf(&b, "<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	return b.String() + "</Section>\n"
}

func rowOf(ls []string, s string) int {
	for i, l := range ls {
		if strings.Contains(l, s) {
			return i
		}
	}
	return -1
}

func TestEditorUnderTheLine(t *testing.T) {
	for _, h := range []int{24, 50} {
		for _, at := range []int{0, 10, 39} {
			st := testStore(t)
			os.WriteFile(st.ReportPath(), []byte(longReport(40)), 0o644)
			a := testApp(t, st)
			a.t.w, a.t.h = 80, h
			pv := a.top()
			a.draw()
			for range at {
				a.handle(key{r: 'j'})
				a.draw()
			}
			before := pv.top
			title := fmt.Sprintf("項目 %d", at)
			a.handle(key{r: 'c'})
			if a.in == nil {
				t.Fatal("c did not open the editor")
			}
			a.in.ed = newEdit("一\n二\n三\n四\n五")
			ls, _ := screen(t, a)
			line, hints := rowOf(ls, title), rowOf(ls, "enter 置く")
			if line < 0 || hints < 0 {
				t.Fatalf("h %d at %d:\n%s", h, at, strings.Join(ls, "\n"))
			}
			// A blank line above the rule, and under the hints.
			if strings.TrimSpace(ls[line+1]) != "" || strings.Trim(ls[line+2], " ─") != "" || strings.TrimSpace(ls[line+3]) != "❯ 一" || strings.TrimSpace(ls[hints+1]) != "" {
				t.Errorf("h %d at %d: the editor is not right under the line:\n%s", h, at, strings.Join(ls, "\n"))
			}
			if rowOf(ls, "にコメント") >= 0 {
				t.Errorf("h %d at %d: a label over a line in view", h, at)
			}
			if hints > (h-1)*2/3+1 {
				t.Errorf("h %d at %d: hints on row %d, below two thirds", h, at, hints)
			}
			if last := regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`).ReplaceAllString(ls[len(ls)-1], ""); strings.TrimSpace(last) != "" {
				t.Errorf("h %d at %d: the last line is %q", h, at, ls[len(ls)-1])
			}
			a.handle(key{name: "esc"})
			a.draw()
			if pv.top != before || pv.cur != fmt.Sprintf("i%d", at) {
				t.Errorf("h %d at %d: after closing top %d (was %d) cur %q", h, at, pv.top, before, pv.cur)
			}
		}
	}
}

func TestEditorM(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(longReport(10)), 0o644)
	a := testApp(t, st)
	a.draw()
	a.handle(key{r: 'j'})
	a.draw()
	a.handle(key{r: 'm'})
	ls, _ := screen(t, a)
	line, label := rowOf(ls, "項目 1"), rowOf(ls, "宛先なしのコメント")
	if line < 0 || label != line+2 {
		t.Errorf("m: line %d label %d:\n%s", line, label, strings.Join(ls, "\n"))
	}
}

func TestEditorLabelOverATallLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"a\">"+strings.Repeat("とても長い題", 200)+"</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 24
	a.draw()
	a.handle(key{r: 'j'})
	a.draw()
	a.handle(key{r: 'c'})
	ls, _ := screen(t, a)
	if rowOf(ls, "にコメント") < 0 || rowOf(ls, "enter 置く") < 0 {
		t.Errorf("a tall line: no label\n%s", strings.Join(ls, "\n"))
	}
}

func TestEditorRewriteInPlace(t *testing.T) {
	st := testStore(t)
	for i := range 5 {
		st.Append(board.Event{Op: "thought", Text: fmt.Sprintf("コメント文 %d", i+1)})
	}
	a := testApp(t, st)
	a.handle(key{name: "tab"})
	a.draw()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'j'})
	a.draw()
	a.handle(key{r: 'e'})
	if a.in == nil || a.in.edit == 0 {
		t.Fatal("e did not open the editor")
	}
	n := a.in.edit
	ls, _ := screen(t, a)
	line := rowOf(ls, fmt.Sprintf("コメント文 %d", n))
	label := rowOf(ls, fmt.Sprintf("コメント %d を直す", n))
	if line < 0 || label != line+2 {
		t.Errorf("e in the list: line %d label %d:\n%s", line, label, strings.Join(ls, "\n"))
	}
	a.handle(key{name: "esc"})
	a.list = false
	// In the pane.
	a.draw()
	for range 10 {
		a.handle(key{r: 'j'})
	}
	if !a.inPane {
		t.Fatal("not in the pane")
	}
	a.draw()
	a.handle(key{r: 'e'})
	ls, _ = screen(t, a)
	n = a.in.edit
	line = rowOf(ls, fmt.Sprintf("コメント文 %d", n))
	label = rowOf(ls, fmt.Sprintf("コメント %d を直す", n))
	if line < 0 || label != line+2 {
		t.Errorf("e in the pane: line %d label %d:\n%s", line, label, strings.Join(ls, "\n"))
	}
}

func TestEditKey(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "途中で出し"})
	a := testApp(t, st)
	a.handle(key{name: "tab"})
	a.render()
	if s := hintLine(a); !strings.Contains(s, "enter 開く  e 直す  x 取り消す") {
		t.Errorf("list hints %q", s)
	}
	a.handle(key{r: 'e'})
	if a.in == nil || a.in.ed.String() != "途中で出し" {
		t.Fatalf("editor %+v", a.in)
	}
	a.handle(key{name: "esc"})
	if len(st.Events()) != 1 {
		t.Error("esc recorded something")
	}
	// Empty, or unchanged: nothing.
	a.handle(key{r: 'e'})
	a.handle(key{name: "ctrl+u"})
	a.handle(key{name: "enter"})
	a.handle(key{r: 'e'})
	a.handle(key{name: "enter"})
	if len(st.Events()) != 1 {
		t.Errorf("empty or unchanged text recorded: %+v", st.Events())
	}
	a.handle(key{r: 'e'})
	for _, r := range "た文" {
		a.handle(key{r: r})
	}
	a.handle(key{name: "enter"})
	evs := st.Events()
	if len(evs) != 2 || evs[1].Op != "edit" || evs[1].Text != "途中で出した文" || evs[1].Thought != 1 {
		t.Errorf("events %+v", evs)
	}
	if a.state.Thoughts[0].Text != "途中で出した文" {
		t.Errorf("shown text %q", a.state.Thoughts[0].Text)
	}
}

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

// A comment on a notice is written right under it, among the notices
// pinned at the top, without a label.
func TestEditorUnderANotice(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n1\">一つ目のお知らせ</Notice>\n<Notice id=\"n2\">二つ目のお知らせ</Notice>\n"+longReport(40)), 0o644)
	for _, h := range []int{24, 50} {
		a := testApp(t, st)
		a.t.w, a.t.h = 80, h
		a.render()
		if a.top().cur != "n1" {
			t.Fatalf("cursor %q", a.top().cur)
		}
		a.handle(key{r: 'c'})
		a.in.ed = newEdit("コメント")
		ls, raw := screen(t, a)
		if !strings.Contains(ls[0], "一つ目のお知らせ") || strings.TrimSpace(ls[1]) != "" || strings.Trim(ls[2], " ─") != "" || strings.TrimSpace(ls[3]) != "❯ コメント" || !strings.Contains(ls[5], "enter 置く") || strings.TrimSpace(ls[6]) != "" || !strings.Contains(ls[7], "二つ目のお知らせ") {
			t.Errorf("h %d:\n%s", h, strings.Join(ls, "\n"))
		}
		if rowOf(ls, "にコメント") >= 0 {
			t.Errorf("h %d: a label", h)
		}
		if !strings.HasSuffix(raw, fmt.Sprintf("\x1b[4;%dH\x1b[?25h", lgutter+markW+width("コメント")+1)) {
			t.Errorf("h %d: terminal cursor %q", h, raw[max(len(raw)-20, 0):])
		}
	}
	// A screen too low for the blank lines: they go, the input and the
	// hints stay.
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 9
	a.render()
	a.handle(key{r: 'c'})
	a.in.ed = newEdit("コメント")
	ls, _ := screen(t, a)
	if !strings.Contains(ls[0], "一つ目のお知らせ") || strings.Trim(ls[1], " ─") != "" || strings.TrimSpace(ls[2]) != "❯ コメント" || !strings.Contains(ls[4], "enter 置く") {
		t.Errorf("h 9:\n%s", strings.Join(ls, "\n"))
	}
}
