package e2e_test

import (
	"strings"
	"testing"
)

// tallReport is an Item whose contents are much taller than the screen,
// with a line before and after it.
func tallReport() string {
	var b strings.Builder
	b.WriteString("<Item id=\"first\">最初の項目</Item>\n<Item id=\"tall\">高い項目\n")
	for i := 0; i < 40; i++ {
		b.WriteString("  - 中身の行 " + itoa(i) + "\n")
	}
	b.WriteString("</Item>\n<Item id=\"next\">次の項目</Item>\n")
	return b.String()
}

// firstBody is the number of the first body line on screen (-1 none).
func firstBody(v *viewer) int {
	for _, l := range v.Lines() {
		if i := strings.Index(l, "中身の行 "); i >= 0 {
			n := 0
			for _, r := range strings.TrimSpace(l[i+len("中身の行 "):]) {
				if r < '0' || r > '9' {
					break
				}
				n = n*10 + int(r-'0')
			}
			return n
		}
	}
	return -1
}

// SPEC.md "画面": the scroll keys move the view, never the cursor:
// ctrl+d / ctrl+u, pgdn / pgup, alt+↓ / alt+↑ by half a screen, ctrl+e /
// ctrl+y and ↓ / ↑ (the mouse wheel, by alternate scroll) by one row.
func TestScrollKeys(t *testing.T) {
	t.Parallel()
	v, _ := open(t, tallReport(), 100, 16, "最初の項目")
	v.MoveTo("高い項目")
	v.Keys("enter")
	v.WaitFor("中身の行 0")
	moved := func(what string, key string, cond func(before, after int) bool) {
		t.Helper()
		before := firstBody(v)
		v.Keys(key)
		v.Until(what, func() bool { a := firstBody(v); return a >= 0 && cond(before, a) })
	}
	half := func(b, a int) bool { return a-b >= 4 }
	back := func(b, a int) bool { return b-a >= 4 }
	one := func(b, a int) bool { return a-b == 1 }
	oneUp := func(b, a int) bool { return b-a == 1 }
	moved("ctrl+d", "ctrl+d", half)
	v.WaitBottom("↑ カーソルは上")    // the cursor stayed on 高い項目
	v.WaitBottom("^d ^u スクロール") // the page is taller than the screen
	moved("ctrl+u", "ctrl+u", back)
	moved("pgdn", "pgdn", half)
	moved("pgup", "pgup", back)
	moved("alt+↓", "\x1b[1;3B", half)
	moved("alt+↑", "\x1b[1;3A", back)
	moved("alt+↓ as ESC first", "\x1b\x1b[B", half)
	moved("ctrl+e", "ctrl+e", one)
	moved("ctrl+y", "ctrl+y", oneUp)
	moved("↓ (the wheel)", "down", one)
	moved("↑ (the wheel)", "up", oneUp)
	// The cursor never moved: j from here reads on from 高い項目.
	v.Keys("ctrl+u", "ctrl+u", "ctrl+u")
	v.WaitCursorOn("高い項目")
}

// SPEC.md "画面": k reads back up through the contents j read down
// through, before it moves to the line before.
func TestKReadsBackUp(t *testing.T) {
	t.Parallel()
	v, _ := open(t, tallReport(), 60, 16, "最初の項目")
	v.MoveTo("高い項目")
	v.Keys("enter")
	v.WaitFor("中身の行 0")
	v.Keys("j", "j", "j")
	v.Until("read down", func() bool { return firstBody(v) >= 12 })
	low := firstBody(v)
	v.Keys("k")
	v.Until("k read back up", func() bool { a := firstBody(v); return a >= 0 && a < low })
	// k by k, up through the contents (the cursor stays on 高い項目) until
	// its title is on screen; only then does k move on.
	for i := 0; i < 8 && v.CursorRow() < 0; i++ {
		before := firstBody(v)
		v.Keys("k")
		v.Until("k read on up", func() bool { return v.CursorRow() >= 0 || firstBody(v) < before })
	}
	v.WaitCursorOn("高い項目")
	v.Keys("k")
	v.WaitCursorOn("最初の項目")
}

// SPEC.md "画面": while writing, pgup / pgdn (and alt+↑↓, ↑ on the first
// line) scroll the page with the editor in it; typing brings it back; the
// text stays and the comment goes on the line it was written on.
func TestScrollWhileWriting(t *testing.T) {
	t.Parallel()
	v, ss := open(t, tallReport(), 100, 16, "最初の項目")
	v.MoveTo("高い項目")
	v.Keys("enter")
	v.WaitFor("中身の行 0")
	v.Keys("c")
	v.WaitFor("pgup pgdn スクロール")
	v.Keys("あい")
	v.WaitFor("❯ あい")
	// Up to the top of the long contents: the editor goes off the screen.
	v.Keys("pgup", "pgup", "pgup", "pgup", "pgup", "pgup")
	v.WaitFor("中身の行 0")
	v.WaitGone("❯ あい")
	v.Keys("pgdn")
	v.Until("pgdn scrolled down", func() bool { return firstBody(v) > 0 })
	v.Keys("up") // the first (only) line of the text: ↑ scrolls
	// Typing brings the editor back, with its text.
	v.Keys("う")
	v.WaitFor("❯ あいう")
	v.Keys("enter")
	rs := ss.waitRecords(t, "thought", 1)
	if rs[0].str("id") != "tall" || rs[0].str("text") != "あいう" {
		t.Errorf("record: %v", rs[0])
	}
}
