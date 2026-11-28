package viewer

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

const linesReport = `<Title>題</Title>

<Section title="節">
段落の文。

<Item id="a">項目
  中身の文
  - 中身の箇条
</Item>
</Section>
`

// Every line on screen is a stop: the title, a heading, a paragraph, and
// the lines of an opened body. On a line of text only c acts: x says it
// cannot hide, and the comment is recorded by the row it is in and what
// it says.
func TestEveryLineIsAStop(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(linesReport), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 24
	wait(t, time.Hour)
	pv := a.top()
	a.render()
	if pv.cur != "a" {
		t.Fatalf("the first cursor is on the first row: %q", pv.cur)
	}
	if got := strings.Join(pv.navKeys(), "|"); got != "t1:題|h1:節|p1:段落の文。|a" {
		t.Errorf("stops %q", got)
	}
	a.handle(key{name: "enter"})
	a.render()
	if got := strings.Join(pv.navKeys(), "|"); got != "t1:題|h1:節|p1:段落の文。|a|a/p1:中身の文|a/-1:中身の箇条" {
		t.Errorf("stops, opened %q", got)
	}
	a.handle(key{r: 'k'})
	a.render()
	if pv.cur != "p1:段落の文。" || !strings.HasPrefix(rowsText(a)[rowOf(rowsText(a), "段落の文。")], ">") {
		t.Fatalf("k: cur %q\n%s", pv.cur, strings.Join(rowsText(a), "\n"))
	}
	if s := hintLine(a); !strings.Contains(s, "c 行に書く") || strings.Contains(s, "x 隠す") || strings.Contains(s, "enter") {
		t.Errorf("hints on a paragraph %q", s)
	}
	a.handle(key{r: 'x'})
	if a.msg != page.MsgNoHide || len(st.Events()) != 0 {
		t.Errorf("x on a paragraph: %q %+v", a.msg, st.Events())
	}
	a.handle(key{name: "enter"}) // nothing to open
	a.handle(key{r: 'c'})
	if a.in == nil {
		t.Fatal("c did not open the editor")
	}
	a.in.ed = newEdit("ここは？")
	a.handle(key{name: "enter"})
	evs := st.Events()
	if len(evs) != 1 || evs[0].ID != "Line|\n段落の文。" || evs[0].Title != "段落の文。" {
		t.Fatalf("events %+v", evs)
	}
	a.render()
	if ls := rowsText(a); !strings.Contains(ls[rowOf(ls, "段落の文。")], "未送信") {
		t.Errorf("the paragraph's right end:\n%s", strings.Join(ls, "\n"))
	}

	// A line of the body: recorded with its row's id.
	for pv.cur != "a/-1:中身の箇条" {
		a.handle(key{r: 'j'})
		a.render()
	}
	a.handle(key{r: 'c'})
	a.in.ed = newEdit("箇条に")
	a.handle(key{name: "enter"})
	if evs := st.Events(); len(evs) != 2 || evs[1].ID != "Line|a\n中身の箇条" {
		t.Fatalf("events %+v", evs)
	}
	// The comments screen says which line each is on; enter goes there.
	a.handle(key{r: 'l'})
	ls := plainLines(func() []string {
		var out []string
		for _, r := range a.listRows() {
			out = append(out, r.line)
		}
		return out
	}())
	if !strings.Contains(ls, "← 中身の箇条") || !strings.Contains(ls, "← 段落の文。") {
		t.Errorf("comments screen:\n%s", ls)
	}
	pv.open["a"] = false
	a.goToComment(a.b.State.Thoughts[1])
	if pv.cur != "a/-1:中身の箇条" || !pv.open["a"] || a.onComments() {
		t.Errorf("to the body line: cur %q open %v", pv.cur, pv.open["a"])
	}
}

// rowsText is the report's rows as drawn, without styles.
func rowsText(a *app) []string {
	var out []string
	for _, r := range a.rows(a.top()) {
		out = append(out, reEsc.ReplaceAllString(r.line, ""))
	}
	return out
}
