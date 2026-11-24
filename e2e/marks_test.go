package e2e_test

import (
	"strings"
	"testing"
)

// markOf returns the mark column (●) of the row with title and its color.
func markOf(t *testing.T, v *viewer, title string) (string, string) {
	t.Helper()
	r := rowOf(t, v, title)
	line := []rune(v.Line(r) + "   ")
	return string(line[2]), v.ColorAt(r, 2)
}

// SPEC.md "反応待ちと未読": which lines get a yellow or green ●, and the
// counts at the top right.
func TestMarksAndCounts(t *testing.T) {
	t.Parallel()
	report := `<Notice id="n">お知らせ</Notice>
<Section title="s">
  <Ask id="q">問い
    <Option key="a">A</Option>
    <Option key="b">B</Option>
  </Ask>
  <Ask id="ask">よしだめを聞く
    <Option key="y">よし</Option>
    <Option key="n">だめ</Option>
  </Ask>
  <Link id="lk" path="x">ただのリンク</Link>
  <Item id="look" look="y">読んでほしい</Item>
  <Ask id="both" look="y">両方
    <Option key="a">A</Option>
    <Option key="b">B</Option>
  </Ask>
  <Item id="wait" wait="you" status="あなた待ち">あなた待ち項目</Item>
  <Item id="plain">普通</Item>
</Section>
<Fold title="黄を含む畳み">
  <Ask id="in1">中の聞く
    <Option key="a">A</Option>
    <Option key="b">B</Option>
  </Ask>
  <Item id="in2" look="y">中の読む</Item>
</Fold>
<Fold title="緑だけの畳み">
  <Item id="in3" look="y">中の読む2</Item>
</Fold>
<Fold title="読む畳み" look="y">
  <Item id="in4">ただの中身</Item>
</Fold>
`
	v, _ := open(t, report, 80, 30, "普通")
	for _, x := range []struct{ title, mark, color string }{
		{"問い", "●", "yellow"},
		{"よしだめを聞く", "●", "yellow"},
		{"ただのリンク", " ", ""}, // ask="y" is gone: a Link asks nothing
		{"読んでほしい", "●", "green"},
		{"両方", "●", "yellow"}, // yellow wins
		{"あなた待ち項目", " ", ""},  // wait="you" is not counted as waiting
		{"普通", " ", ""},
		{"お知らせ", " ", ""}, // a Notice is always seen
		{"黄を含む畳み", "●", "yellow"},
		{"緑だけの畳み", "●", "green"},
		{"読む畳み", "●", "green"},
	} {
		m, c := markOf(t, v, x.title)
		if m != x.mark || (x.color != "" && c != x.color) {
			t.Errorf("%s: mark %q %q, want %q %q", x.title, m, c, x.mark, x.color)
		}
	}
	// 反応待ち: the Asks q, ask, both, in1 = 4. 未読 (not also
	// waiting): look, in2, in3, the look Fold = 4.
	v.WaitFor("反応待ち 4  未読 4")
	wantColor(t, v, "反応待ち 4", "yellow")
	wantColor(t, v, "未読 4", "green")
	// The counts row is not a report row: it sits under pinned Notices.
	if r := rowOf(t, v, "反応待ち 4"); strings.Contains(v.Line(r), "お知らせ") {
		t.Errorf("counts on the Notice row")
	}
	// wait="you": its status in yellow.
	r := rowOf(t, v, "あなた待ち項目")
	if c := v.ColorAt(r, colOf(v.Line(r))-1); c != "yellow" {
		t.Errorf("wait=\"you\" status is %q, want yellow", c)
	}
}

// SPEC.md "反応待ちと未読": hidden lines get no mark and are not counted.
func TestHiddenLinesNotCounted(t *testing.T) {
	t.Parallel()
	report := `<Item id="a">普通</Item>
<Item id="look" look="y">読んでほしい</Item>
`
	v, ss := open(t, report, 80, 20, "読んでほしい")
	v.WaitFor("未読 1")
	v.MoveTo("読んでほしい")
	v.Keys("x")
	ss.waitRecords(t, "hide", 1)
	v.WaitGone("未読 1")
	v.Keys("H")
	v.WaitFor("読んでほしい")
	if m, _ := markOf(t, v, "読んでほしい"); m == "●" {
		t.Errorf("a hidden line shown with H has a mark")
	}
}
