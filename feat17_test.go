package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

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
			os.WriteFile(st.reportPath(), []byte(longReport(40)), 0o644)
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
			if ls[line+1] != "" || strings.TrimSpace(ls[line+2]) != "一" {
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
	os.WriteFile(st.reportPath(), []byte(longReport(10)), 0o644)
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
	os.WriteFile(st.reportPath(), []byte("<Item id=\"a\">"+strings.Repeat("とても長い題", 200)+"</Item>\n"), 0o644)
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
		st.Append(Event{Op: "thought", Text: fmt.Sprintf("コメント文 %d", i+1)})
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
