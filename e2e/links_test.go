package e2e_test

import (
	"strings"
	"testing"
)

const linksReport = `<Title>題</Title>
<Section title="あなたへ">
  <Item id="a">> 順番は？ そうする: [その作業](#work)
    中身にも [もう一つ](#other) へのリンク
  </Item>
  <Item id="b">別の行から [その作業へ](#work)</Item>
  <Item id="other">もう一つの行</Item>
</Section>
<Fold title="待っている作業">
  <Item id="work">順番待ちの作業の行</Item>
</Fold>
`

// FEAT26: g numbers the cursor line's row links 1 2 … (only that line's);
// the digit goes to the row, opening the Fold around it; g - comes back.
func TestRowLinksJumpAndBack(t *testing.T) {
	t.Parallel()
	v, _ := open(t, linksReport, 80, 24, "順番は？")
	v.WaitCursorOn("順番は？")
	v.Keys("j", "k") // pointed at by the person
	v.WaitCursorOn("順番は？")
	// Two links (title, contents): no "g 1" hint (that is for one link).
	if strings.Contains(strings.Join(v.Lines()[20:], "\n"), "g 1") {
		t.Errorf("g 1 hint with two links:\n%s", v.dump())
	}
	v.Keys("g")
	v.WaitFor("esc やめる")
	guide := joinedAll(strings.Join(v.Lines()[24-3:], "\n"))
	for _, x := range []string{"1 順番待ちの作業の行", "2 もう一つの行"} {
		if !strings.Contains(guide, x) {
			t.Errorf("guide without %q:\n%s", x, v.dump())
		}
	}
	if strings.Contains(guide, "- 戻る") {
		t.Errorf("back offered before any jump:\n%s", v.dump())
	}
	// The digit is drawn right before the link on the cursor's line only.
	if r := rowOf(t, v, "順番は？"); !strings.Contains(v.Line(r), "1その作業") {
		t.Errorf("no digit before the link:\n%s", v.dump())
	}
	if r := rowOf(t, v, "別の行から"); strings.Contains(v.Line(r), "1その作業へ") {
		t.Errorf("a digit on a line the cursor is not on:\n%s", v.dump())
	}
	v.Keys("1")
	// The Fold opens; the cursor is on the target.
	v.WaitCursorOn("順番待ちの作業の行")
	v.Keys("g")
	v.WaitFor("- 戻る")
	v.Keys("-")
	v.WaitCursorOn("順番は？")
	// Another line with one link: the hint says where g 1 goes.
	v.Keys("j")
	v.WaitCursorOn("別の行から")
	v.WaitBottom("g 1 → 順番待ちの作業の行")
	// A digit the line has no link for does nothing.
	v.Keys("g", "2")
	v.WaitCursorOn("別の行から")
	v.Keys("g", "1")
	v.WaitCursorOn("順番待ちの作業の行")
}
