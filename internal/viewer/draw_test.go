package viewer

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

func TestOnlyChangedRowsAreWritten(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a" ask="y">一</Item>
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
	for _, want := range []string{"  ● · 一", "> ● · 二", "    · 三"} {
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

func TestHeadMarksAndDoingWord(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a">まだの行</Item>
<Item id="b" state="doing">作業している行</Item>
<Item id="c" state="doing" status="順番待ち">別の語の行</Item>
<Item id="d" state="done">済んだ行</Item>
<Item id="e">開ける行
中身
</Item>
`), 0o644)
	a := testApp(t, st)
	s := shown(a)
	for _, want := range []string{"· まだの行", "· 作業している行", "· 別の語の行", "✓ 済んだ行", "▸ 開ける行"} {
		if !strings.Contains(s, want) {
			t.Errorf("want %q in\n%s", want, s)
		}
	}
	if strings.ContainsAny(s, "○~") {
		t.Errorf("an old mark:\n%s", s)
	}
	for _, l := range strings.Split(s, "\n") {
		switch {
		case strings.Contains(l, "作業している行"):
			if !strings.HasSuffix(l, "作業中") {
				t.Errorf("doing without a status: %q", l)
			}
		case strings.Contains(l, "別の語の行"):
			if !strings.HasSuffix(l, "順番待ち") || strings.Contains(l, "作業中") {
				t.Errorf("doing with a status: %q", l)
			}
		case strings.Contains(l, "まだの行"), strings.Contains(l, "済んだ行"):
			if strings.Contains(l, "作業中") {
				t.Errorf("not doing: %q", l)
			}
		}
	}
	// 作業中 is green, as a doing status is.
	pv := a.top()
	if r := a.right(pv, report.ByID(pv.Nodes, "b")); r.plain() != "作業中" || r[0].st != green {
		t.Errorf("right %q", r.plain())
	}
}

// ---- opened contents are in the normal color ----

func TestOpenedContentsColor(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a" status="待ち" note="上の答え待ち">項目
中身の文
</Item>
<Item id="d" state="done">済んだ項目
済んだ中身
</Item>`), 0o644)
	th, _ := st.Append(board.Event{Op: "thought", ID: "a", Text: "思いつき"})
	st.Settle(th.N, "todo にした")
	a := testApp(t, st)
	pv := a.top()
	pv.open["a"], pv.open["d"] = true, true
	var raw []string
	for _, r := range a.rows(pv) {
		raw = append(raw, r.line)
	}
	s := strings.Join(raw, "\n")
	for _, c := range []struct{ want, not string }{
		{"中身の文", dim + "中身の文"},
		{dim + "理由: ", magenta + "上の答え待ち"},
		{"上の答え待ち", dim + "上の答え待ち"},
		{dim + "対応: ", dim + "対応: todo"},
		{dim + "済んだ中身", ""},
	} {
		if !strings.Contains(s, c.want) || (c.not != "" && strings.Contains(s, c.not)) {
			t.Errorf("want %q, not %q in:\n%q", c.want, c.not, s)
		}
	}
}

func TestRichTextDrawn(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(richItem), 0o644)
	a := testApp(t, st)
	a.t.w = 44
	pv := a.top()
	a.render()
	a.handle(key{name: "enter"})
	s := shown(a)
	for _, want := range []string{
		"\n        おすすめは折衷。\n\n        · cwd に置く\n          - 良い点: 書ける続きの行\n",
		"\n        · 折衷: データは cwd\n\n        手順\n        1. まず cwd\n        2. 次に XDG\n           - 入れ子\n",
		"\n        行末で改行する\n        次の行\n        その次の行\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("want %q in\n%s", want, s)
		}
	}
	_ = pv
	// Wrapping keeps to where the text starts, past the number.
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"a\">t\n    1. "+strings.Repeat("あ", 40)+"\n      - "+strings.Repeat("い", 40)+"\n</Item>"), 0o644)
	a.reload(pv)
	pv.open["a"] = true
	s = shown(a)
	if !strings.Contains(s, "\n        1. あああ") || !strings.Contains(s, "\n           ああ") || !strings.Contains(s, "\n           - いい") || !strings.Contains(s, "\n             いい") {
		t.Errorf("wrapping:\n%s", s)
	}
}

// ---- opened items ----

func TestOpenTitleIsBold(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{name: "enter"}) // the Ask
	rows := a.rows(pv)
	var line string
	for _, r := range rows {
		if strings.Contains(r.line, "ページの単位") {
			line = r.line
		}
	}
	if !strings.Contains(line, bold) {
		t.Errorf("open title not bold: %q", line)
	}
	s := shown(a)
	if !strings.Contains(s, "プロジェクトごと\n\n") {
		t.Errorf("no blank line after the open Ask:\n%s", s)
	}
	a.handle(key{name: "enter"})
	for _, r := range a.rows(pv) {
		if strings.Contains(r.line, "ページの単位") && strings.Contains(r.line, bold) {
			t.Errorf("closed title bold: %q", r.line)
		}
	}
}

func TestSmallDisplayFixes(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.t.w = 40
	// 9c: a placed cursor on an Ask says how to answer.
	a.render()
	if pv.cur != "unit" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k") || strings.Contains(s, "→") || (!strings.Contains(s, "動かすと x が効く") && strings.Contains(s, "↑↓")) {
		t.Errorf("placed on a closed Ask: %q", s)
	}
	a.handle(key{name: "enter"})
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k") || strings.Contains(s, "→") || (!strings.Contains(s, "動かすと答えられる") && strings.Contains(s, "↑↓")) {
		t.Errorf("placed on an open Ask: %q", s)
	}
	// 9a: at width 40 the pane says there is more, and where.
	for i := 0; i < 7; i++ {
		st.Append(board.Event{Op: "thought", Text: "t"})
	}
	sendAll(st)
	st.Changes()
	st.Settle(1, "x")
	st.Append(board.Event{Op: "thought", Text: "u"})
	a.loadEvents()
	p := reEsc.ReplaceAllString(strings.Join(a.pane(30), "\n"), "")
	if strings.Contains(p, "s で送る") {
		t.Errorf("pane at width 40:\n%s", p)
	}
	// 9d: a long destination does not squeeze the thought in the list.
	a.t.w = 80
	th, _ := st.Append(board.Event{Op: "thought", Text: "the thought itself"})
	st.Settle(th.N, strings.Repeat("とても長い行き先 ", 20))
	a.loadEvents()
	l := reEsc.ReplaceAllString(strings.Join(func() []string {
		var out []string
		for _, r := range a.listRows() {
			out = append(out, r.line)
		}
		return out
	}(), "\n"), "")
	if !strings.Contains(l, "the thought itself") || strings.Contains(l, "th…") {
		t.Errorf("list:\n%s", l)
	}
}

// 3, 7: no line runs past the width, however narrow.
func TestNothingPastTheWidth(t *testing.T) {
	st := testStore(t)
	src := "<Title>テストのとても長い題</Title>\n<Item id=\"a\" ask=\"y\">一</Item>\n"
	for i := range 5 {
		src += fmt.Sprintf("<Item id=\"n%d\" look=\"y\">新しい %d</Item>\n", i, i)
	}
	os.WriteFile(st.ReportPath(), []byte(src), 0o644)
	st.Append(board.Event{Op: "thought", Text: "a"})
	for _, w := range []int{1, 5, 10, 16, 20, 21, 30} {
		for _, help := range []bool{false, true} {
			a := testApp(t, st)
			a.t.w, a.t.h = w, 24
			a.help = help
			ls, _ := screen(t, a)
			for i, l := range ls {
				if visibleWidth(l) > w {
					t.Errorf("width %d help %v: line %d is %d wide: %q", w, help, i, visibleWidth(l), l)
				}
			}
		}
	}
	// The counts at the top right go from the right when they do not fit.
	a := testApp(t, st)
	a.t.w = 30
	if m := a.sendMark().plain(); m != "反応待ち 1  未読 5" {
		t.Errorf("mark at 30: %q", m)
	}
	a.t.w = 20
	if m := a.sendMark().plain(); m != "反応待ち 1" {
		t.Errorf("mark at 20: %q", m)
	}
}

// 11, 12: a bold head word on a list line keeps a line of its own, and
// lines under "1. " may be indented to its text.
func TestListHeadsAndNumberedIndent(t *testing.T) {
	src := "<Item id=\"a\">t\n- **XDG に置く**\n  続きの行がここに来ます。\n1. 文\n   - 入れ子\n   - 入れ子 2\n2. 次\n</Item>"
	nodes, probs, _ := report.Parse(src)
	if len(probs) != 0 {
		t.Fatalf("problems %v", probs)
	}
	body := nodes[0].Body
	if body[0].Text != "**XDG に置く**\n続きの行がここに来ます。" {
		t.Errorf("head: %q", body[0].Text)
	}
	if len(body) != 3 || len(body[1].Subs) != 2 || body[2].Num != "2." {
		t.Errorf("numbered: %d, subs %d", len(body), len(body[1].Subs))
	}
	// Problems that leave nothing out do not say "what could be read".
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a" wait="me">x</Item>`), 0o644)
	a := testApp(t, st)
	if s := shown(a); !strings.Contains(s, "誤りがあります") || strings.Contains(s, "読めた所まで") {
		t.Errorf("complete report:\n%s", s)
	}
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"a\">x</Item>\n<Bogus>y</Bogus>"), 0o644)
	a.reload(a.top())
	if s := shown(a); !strings.Contains(s, "読めた所まで") {
		t.Errorf("partial report:\n%s", s)
	}
}
