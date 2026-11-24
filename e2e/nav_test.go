package e2e_test

import (
	"strings"
	"testing"
)

const navReport = `<Notice id="n1" at="2020-11-22 21:46">お知らせ</Notice>
<Title>題</Title>
<Section title="あなたへ">
  <Item id="a">項目A</Item>
  <Item id="b">項目B</Item>
</Section>
<Section title="進行中">
  <Item id="c">項目C</Item>
  <Fold title="中の畳み">
    <Item id="e">項目E</Item>
  </Fold>
</Section>
<Fold title="決まったこと">
  <Item id="d">項目D</Item>
</Fold>
`

// SPEC.md "画面": j k ctrl+n ctrl+p, full-width keys move the cursor (↑ ↓
// scroll the view: TestScrollKeys); k from the top goes into the Notice.
func TestMoveKeys(t *testing.T) {
	t.Parallel()
	v, _ := open(t, navReport, 80, 24, "項目A")
	// The first cursor is on the Notice.
	v.WaitCursorOn("お知らせ")
	for _, x := range []struct{ key, want string }{
		{"j", "項目A"}, {"j", "項目B"}, {"ctrl+n", "項目C"}, {"ｊ", "中の畳み"}, {"j", "決まったこと"},
		{"k", "中の畳み"}, {"k", "項目C"}, {"k", "項目B"}, {"ctrl+p", "項目A"}, {"ｋ", "お知らせ"},
	} {
		v.Keys(x.key)
		v.WaitCursorOn(x.want)
	}
	v.WaitBottom("c 行に書く  x 隠す") // moved by the person: x works on the Notice
}

// SPEC.md "画面": reading contents taller than the screen.
func TestReadTallContents(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("<Item id=\"tall\">高い項目\n")
	for i := 0; i < 40; i++ {
		b.WriteString("  - 中身の行 " + itoa(i) + "\n")
	}
	b.WriteString("</Item>\n<Item id=\"next\">次の項目</Item>\n")
	v, ss := open(t, b.String(), 60, 16, "高い項目")
	v.MoveTo("次の項目")
	v.MoveTo("高い項目")
	v.Keys("enter")
	v.WaitFor("中身の行 0")
	// j scrolls by half a screen without moving the cursor.
	v.Keys("j")
	v.WaitFor("↑ カーソルは上")
	if v.CursorRow() >= 0 {
		t.Errorf("the cursor row should be off screen:\n%s", v.dump())
	}
	wantColor(t, v, "↑ カーソルは上", "dim")
	// It wins over every key in the guide, even at a narrow width.
	v.Resize(24, 16)
	v.Until("the mark kept when narrow", func() bool {
		return strings.Contains(v.Bottom(), "カーソルは上") && v.CursorRow() < 0
	})
	v.Resize(60, 16)
	v.WaitBottom("↑ カーソルは上")
	// A key for the line brings the cursor back and is not taken.
	v.Keys("x")
	v.WaitCursorOn("高い項目")
	v.Sync()
	ss.noRecords(t, "hide")
	// Keep going: the next stop is reached only after the gap is shown.
	for i := 0; i < 20 && !strings.Contains(v.Screen(), "中身の行 39"); i++ {
		v.Keys("j")
		v.Sync()
		if r := v.CursorRow(); r >= 0 && strings.Contains(v.Line(r), "次の項目") {
			t.Fatalf("the cursor jumped past unseen contents:\n%s", v.dump())
		}
	}
	v.WaitFor("中身の行 39")
	v.Keys("j")
	v.WaitCursorOn("次の項目")
}

// SPEC.md "反応待ちと未読": space goes to the nearest marked line below,
// round to the top; opens a closed Fold.
func TestSpaceToNextMark(t *testing.T) {
	t.Parallel()
	report := `<Item id="a">普通A</Item>
<Ask id="m1">印一
  <Option key="a">A</Option>
  <Option key="b">B</Option>
</Ask>
<Item id="b">普通B</Item>
<Item id="w" wait="you" status="あなた待ち">待ちの項目</Item>
<Fold title="畳んだ">
  <Item id="g" look="y">畳んだ中の未読</Item>
</Fold>
`
	v, ss := open(t, report, 80, 24, "普通A")
	v.MoveTo("普通B")
	v.MoveTo("普通A")
	v.WaitBottom("space 次へ")
	for _, want := range []string{"印一", "待ちの項目", "畳んだ中の未読", "印一"} {
		v.Keys("space")
		v.WaitCursorOn(want)
	}
	// Right after space, x and the options are dropped for 0.4 s; then
	// they work.
	v.Keys("x")
	v.Sync()
	ss.noRecords(t, "hide")
	v.pressUntil(ss, "x", "hide", 1)

	t.Run("nothing marked", func(t *testing.T) {
		v, _ := open(t, "<Item id=\"a\">普通</Item>\n", 60, 10, "普通")
		v.Keys("space")
		v.WaitFor("印のある行はありません")
	})
}

// SPEC.md "画面": g jumps to Notices, Sections and top-level Folds.
func TestJump(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString(navReport)
	for i := 0; i < 30; i++ {
		b.WriteString("<Section title=\"節" + itoa(i) + "\">\n  <Item id=\"s" + itoa(i) + "\">節の項目" + itoa(i) + "</Item>\n</Section>\n")
	}
	v, ss := open(t, b.String(), 80, 24, "項目A")
	v.WaitBottom("g 飛ぶ")
	v.Keys("g")
	// The guide lists the targets; labels in yellow at the start of the
	// heading rows (the cursor column).
	v.WaitFor("esc やめる")
	guide := joinedAll(strings.Join(v.Lines()[24-3:], "\n"))
	// esc やめる comes first; g is never a label (a-f, h-z: 25 at most).
	if !strings.HasPrefix(guide, "esc やめる") {
		t.Errorf("the guide does not start with esc やめる:\n%s", v.dump())
	}
	for _, x := range []string{"a お知らせ", "c 進行中", "e 節0", "f 節1", "h 節2", "z 節20"} {
		if !strings.Contains(guide, x) {
			t.Errorf("guide without %q:\n%s", x, v.dump())
		}
	}
	if strings.Contains(guide, "g 節") || strings.Contains(guide, "節21") {
		t.Errorf("g used as a label, or more than 25 targets:\n%s", v.dump())
	}
	// A Fold inside a Section is not a target: no label on its row.
	if l := v.Line(rowOf(t, v, "中の畳み")); l[0] != ' ' {
		t.Errorf("a Fold inside a Section is a target:\n%s", v.dump())
	}
	r := rowOf(t, v, "進行中")
	if !strings.HasPrefix(v.Line(r), "c") || v.ColorAt(r, 0) != "yellow" {
		t.Errorf("label not yellow at the heading row:\n%s", v.dump())
	}
	v.Keys("h")
	v.WaitCursorOn("節の項目2")
	v.Keys("g", "c")
	v.WaitCursorOn("項目C")
	// The heading comes to the top (under the counts row).
	if r := rowOf(t, v, "進行中"); r > 2 {
		t.Errorf("heading not at the top (row %d):\n%s", r, v.dump())
	}
	// Moved by the viewer: 0.4 s without x.
	v.Keys("x")
	v.Sync()
	ss.noRecords(t, "hide")

	t.Run("g again leaves", func(t *testing.T) {
		v.In(t).Keys("g")
		v.WaitFor("esc やめる")
		v.Keys("g")
		v.Until("left jump mode", func() bool { return !strings.Contains(v.Screen(), "esc やめる") })
		v.WaitCursorOn("項目C")
	})

	t.Run("a rewrite while jumping leaves and says so", func(t *testing.T) {
		v.In(t).Keys("g")
		v.WaitFor("esc やめる")
		ss.write(strings.Replace(b.String(), "項目A", "項目A改", 1))
		v.WaitForJoined("報告が書き換わりました。カーソルの行を確かめてください")
		v.Until("left jump mode", func() bool { return !strings.Contains(v.Screen(), "esc やめる") })
	})

	t.Run("esc and unknown letters leave", func(t *testing.T) {
		v.In(t).Keys("g")
		v.WaitFor("esc やめる")
		v.Keys("esc")
		v.Until("left jump mode", func() bool { return !strings.Contains(v.Screen(), "esc やめる") })
		v.WaitCursorOn("項目C")
		v.Keys("g")
		v.WaitFor("esc やめる")
		v.Keys("8")
		v.Until("left jump mode", func() bool { return !strings.Contains(v.Screen(), "esc やめる") })
		v.WaitCursorOn("項目C")
	})

	t.Run("a Fold target: onto the Fold, not opened", func(t *testing.T) {
		v.In(t).Keys("g")
		v.WaitFor("esc やめる")
		v.Keys("d")
		v.WaitCursorOn("決まったこと")
		if strings.Contains(v.Screen(), "項目D") {
			t.Errorf("g opened the Fold")
		}
	})
}

// SPEC.md "画面": j past the last stop enters the comments pane; k from
// its first item goes back.
func TestIntoCommentsPane(t *testing.T) {
	t.Parallel()
	v, _ := open(t, "<Item id=\"a\">項目A</Item>\n<Item id=\"b\">項目B</Item>\n", 80, 24, "項目A",
		ev(1, "thought", `"text":"コメント一"`), ev(2, "thought", `"text":"コメント二"`))
	v.MoveTo("項目B")
	v.Keys("j")
	v.WaitCursorOn("コメント二") // newest first
	v.Keys("j")
	v.WaitCursorOn("コメント一")
	v.Keys("k", "k")
	v.WaitCursorOn("項目B")
	// The person came back with k: keys work at once.
	v.WaitBottom("c 行に書く")
}

// SPEC.md "画面": the tab list.
func TestTabList(t *testing.T) {
	t.Parallel()
	v, ss := open(t, "<Item id=\"a\">項目A</Item>\n", 80, 24, "項目A")
	v.Keys("tab")
	v.WaitBottom("tab 報告へ  m 書く  ? キー")
	v.Keys("tab")
	v.WaitFor("項目A")
	ss.appendEvents(ev(1, "thought", `"id":"a","title":"古い題","text":"古いコメント"`), ev(2, "thought", `"text":"新しいコメント\n二行目"`))
	v.WaitFor("新しいコメント")
	v.Keys("tab")
	v.WaitBottom("e 直す")
	v.Until("newest first", func() bool {
		r1, _ := v.Find("新しいコメント")
		r2, _ := v.Find("古いコメント")
		return r1 >= 0 && r2 > r1
	})
	if strings.Contains(v.Screen(), "二行目") {
		t.Errorf("the list shows more than the first line:\n%s", v.dump())
	}
	v.MoveTo("古いコメント")
	v.Keys("enter")
	// Opened: the line's title (and the title when written).
	v.WaitFor("項目A", "古い題")
}

// SPEC.md "画面": ? lists every key; q, esc or ? close it, nothing else.
func TestHelpScreen(t *testing.T) {
	t.Parallel()
	v, _ := open(t, "<Item id=\"a\">項目A</Item>\n", 80, 20, "項目A")
	for _, closer := range []string{"?", "esc", "q"} {
		v.Keys("?")
		v.WaitFor(helpMarker)
		// Had x, y or tab closed it, the closer would open it again (?)
		// or act on the report (esc, q quits), and this would fail.
		v.Keys("x", "y", "tab", closer)
		v.WaitGone(helpMarker)
		v.WaitFor("項目A")
		v.Sync()
		if strings.Contains(v.Bottom(), "tab 報告へ") {
			t.Fatalf("tab reached the report through the ? list")
		}
	}
	// It mentions what the spec asks it to (a tall screen: all of it).
	tall, _ := open(t, "<Item id=\"a\">項目A</Item>\n", 100, 120, "項目A")
	tall.Keys("?")
	tall.WaitFor(helpMarker)
	all := joinedAll(tall.Screen())
	for _, want := range []string{"報告のいちばん上からさらに k で、上に固定されたお知らせへ", "問いに答える"} {
		if !strings.Contains(all, want) {
			t.Errorf("? list does not say %q:\n%s", want, tall.dump())
		}
	}
	v.Keys("?")
	v.WaitFor(helpMarker)
	v.Keys("q")
	v.WaitGone(helpMarker)
	// The viewer is still running: q closed only the list.
	v.WaitFor("項目A")
}

// joinedAll joins screen text with lines trimmed.
func joinedAll(s string) string {
	var b strings.Builder
	for _, l := range strings.Split(s, "\n") {
		b.WriteString(strings.TrimSpace(l))
	}
	return b.String()
}
