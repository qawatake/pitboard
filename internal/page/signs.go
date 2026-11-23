package page

import "github.com/qawatake/pitboard/internal/report"

// The signs of a line that opens, closed and open. Both fronts draw these
// (the terminal as text, open in bold; the browser as a drawn triangle).
// They stay ▸ ▾, not ▶ ▼: U+25B6 and U+25BC are East Asian Ambiguous (two
// cells in a terminal set to treat those as wide, as CJK setups often are,
// and ▶ may take the emoji form), while U+25B8 and U+25BE are one cell
// everywhere. What is open is told by weight instead (bold).
const (
	SignClosed = "▸"
	SignOpen   = "▾"
)

// SignOf is the mark at the head of a line the person points at, as it is
// drawn closed (open false) or open: ▸ / ▾ when it opens, ✓ a done Item,
// ? an Ask, → a Link, · an Item with nothing to open, ! a notice that does
// not open, ⌀ a hidden line, and a list line's own (its number, - or ·).
func SignOf(n *report.Node, canOpen, open, hidden bool) string {
	switch {
	case hidden:
		return "⌀"
	case n.Kind == report.KindNotice && canOpen && open:
		return SignOpen
	case n.Kind == report.KindNotice && canOpen:
		return SignClosed
	case n.Kind == report.KindNotice:
		return "!"
	case n.Kind == report.KindLink:
		return "→"
	case n.Kind == report.KindDone:
		return "✓"
	case n.Kind == report.KindQuestion:
		return "?" // even when a long question opens
	case canOpen && open:
		return SignOpen
	case canOpen:
		return SignClosed
	case n.Kind == report.KindDoing, n.Kind == report.KindTodo:
		return "·"
	case n.Kind == report.KindPlain:
		return ListSign(n)
	}
	return ""
}

// ListSign is the mark of a list line: its number, or - · by level.
func ListSign(n *report.Node) string {
	switch {
	case n.Num != "":
		return n.Num
	case n.Level == 1:
		return "-"
	}
	return "·"
}

// JumpLetter is the letter g goes to the i-th place with: a to z, skipping
// g (g while choosing leaves it).
func JumpLetter(i int) string {
	if i >= 'g'-'a' {
		i++
	}
	return string(rune('a' + i))
}

// JumpIndex is which place letter r goes to among n (-1: none).
func JumpIndex(r rune, n int) int {
	if r < 'a' || r > 'z' || r == 'g' {
		return -1
	}
	i := int(r - 'a')
	if r > 'g' {
		i--
	}
	if i >= n {
		return -1
	}
	return i
}

// AnswerHead is how many cells of the chosen option a closed Ask shows.
const AnswerHead = 12

// RuneWidth is how many cells r takes on a terminal.
func RuneWidth(r rune) int {
	switch {
	case r >= 0x300 && r <= 0x36f, r >= 0x200b && r <= 0x200f, r >= 0xfe00 && r <= 0xfe0f:
		return 0
	case r >= 0x1100 && r <= 0x115f, r >= 0x2e80 && r <= 0xa4cf, r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff, r >= 0xfe30 && r <= 0xfe4f, r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6, r >= 0x1f300 && r <= 0x1faff, r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}

// Cut is s in at most w cells, ending in … when cut.
func Cut(s string, w int) string {
	rs := []rune(s)
	keep, acc := 0, 0 // keep: how many runes fit in w-1 (room for the …)
	for i, r := range rs {
		rw := RuneWidth(r)
		if acc+rw > w {
			return string(rs[:keep]) + "…"
		}
		acc += rw
		if acc <= w-1 {
			keep = i + 1
		}
	}
	return s
}
