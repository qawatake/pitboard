package viewer

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Of two lines saying the same in one row, a comment on the second shows
// on the second only, and g on the comments screen goes to the second.
func TestCommentOnASecondSameLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Item id=\"a\">項目\n  - 同じ\n  - 間\n  - 同じ\n</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 60, 24
	wait(t, time.Hour)
	pv := a.top()
	pv.open["a"] = true
	a.render()
	for pv.cur != "a/-2:同じ" {
		a.handle(key{r: 'j'})
		a.render()
	}
	a.handle(key{r: 'c'})
	if a.in == nil {
		t.Fatal("c did not open the editor")
	}
	a.in.ed = newEdit("2 つ目に")
	a.handle(key{name: "enter"})
	if evs := st.Events(); len(evs) != 1 || evs[0].ID != "Line#2|a\n同じ" {
		t.Fatalf("events %+v", evs)
	}
	a.render()
	var marked []int
	for i, l := range rowsText(a) {
		if strings.Contains(l, "同じ") && strings.Contains(l, "未送信") {
			marked = append(marked, i)
		}
	}
	ls := rowsText(a)
	if len(marked) != 1 || marked[0] < rowOf(ls, "間") {
		t.Errorf("未送信 on %v:\n%s", marked, strings.Join(ls, "\n"))
	}
	pv.cur = "a"
	a.goToComment(a.b.State.Thoughts[0])
	if pv.cur != "a/-2:同じ" {
		t.Errorf("g went to %q", pv.cur)
	}
}
