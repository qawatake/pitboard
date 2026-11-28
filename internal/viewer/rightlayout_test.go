package viewer

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// stripRight is row l without the right-end word w and the spaces before
// it: what is left is the line's own layout.
func stripRight(l, w string) string {
	if i := strings.LastIndex(l, w); i >= 0 {
		l = l[:i]
	}
	l = strings.TrimRight(l, " ")
	if strings.HasPrefix(l, ">") {
		l = " " + l[1:]
	}
	return l
}

// A comment on a line never changes how it is laid out: a "**head**: body"
// line that does not fit stays a definition (the head alone, the body
// under it), its 未送信 at the end of the head's row.
func TestLayoutIndependentOfTheRightEnd(t *testing.T) {
	for _, w := range []int{50, 80} {
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte(`<Title>題</Title>

<Section title="進め方">
- **まず**: 仕様を読んで、何を作るかを短くまとめる。それから手を動かす前に一度だけ確かめる
- **短い**: 一行
1. **番号**: 番号つきの行も、頭の語があれば定義の形になる長さの文をここに書く

<Item id="a">項目
  - **中身の頭**: 開いた中身の箇条にも、頭の語があって長い文がつづくものがある。ここがその例
  **段落の頭**: 中身の段落にも頭の語があって、収まらなければ定義の形で描く。ここはその例
</Item>
</Section>
`), 0o644)
		a := testApp(t, st)
		a.t.w, a.t.h = w, 60
		wait(t, time.Hour)
		pv := a.top()
		pv.open["a"] = true
		draw := func() []string {
			var out []string
			for _, l := range rowsText(a) {
				// A right end that does not fit beside the line goes on a
				// row of its own under it: not part of the line's layout.
				if s := stripRight(l, "未送信"); s != "" || !strings.Contains(l, "未送信") {
					out = append(out, s)
				}
			}
			return out
		}
		want := draw()
		if i := rowOf(want, "まず"); i < 0 || strings.Contains(want[i], "まず:") {
			t.Fatalf("%d: not a definition:\n%s", w, strings.Join(want, "\n"))
		}
		var targets []string
		report.WalkDeep(pv.Nodes, func(n *report.Node) {
			if report.Line(n) || n.Kind == report.KindPlain {
				targets = append(targets, report.Target(n))
			}
		})
		for i, id := range targets {
			st.Append(board.Event{Op: "thought", ID: id, Text: "c" + string(rune('0'+i))})
		}
		a.loadEvents()
		got := rowsText(a)
		if g := draw(); strings.Join(g, "\n") != strings.Join(want, "\n") {
			t.Errorf("%d: a comment changes the layout:\n%s\nwant:\n%s", w, strings.Join(g, "\n"), strings.Join(want, "\n"))
		}
		for _, s := range []string{"まず", "中身の頭", "段落の頭", "短い: 一行"} {
			if i := rowOf(got, s); i < 0 || !strings.Contains(got[i], "未送信") {
				t.Errorf("%d: no 未送信 on the row of %q:\n%s", w, s, strings.Join(got, "\n"))
			}
		}
	}
}

// A comment on a code block shows 未送信 at its right end, and the cursor
// on a code block that starts with a blank line is on its first row that
// is not blank.
func TestCodeRightEndAndCursor(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Section title=\"節\">\n<Code>\n\ngo test ./...\n</Code>\n</Section>\n"), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 60, 24
	wait(t, time.Hour)
	pv := a.top()
	var code *report.Node
	report.WalkDeep(pv.Nodes, func(n *report.Node) {
		if n.Kind == report.KindCode {
			code = n
		}
	})
	if code == nil {
		t.Fatal("no code block")
	}
	pv.cur = code.Key
	a.render()
	ls := rowsText(a)
	i := rowOf(ls, "go test")
	if i < 0 || !strings.HasPrefix(ls[i], ">") {
		t.Errorf("the cursor is not on the code's first row that is not blank:\n%s", strings.Join(ls, "\n"))
	}
	for _, l := range ls {
		if strings.HasPrefix(l, ">") && strings.TrimSpace(l) == ">" {
			t.Errorf("the cursor on a blank row:\n%s", strings.Join(ls, "\n"))
		}
	}
	a.handle(key{r: 'c'})
	if a.in == nil {
		t.Fatal("c did not open the editor")
	}
	a.in.ed = newEdit("ここは？")
	a.handle(key{name: "enter"})
	a.render()
	ls = rowsText(a)
	if i := rowOf(ls, "go test"); i < 0 || !strings.Contains(ls[i], "未送信") {
		t.Errorf("no 未送信 on the code:\n%s", strings.Join(ls, "\n"))
	}
}
