package viewer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/qawatake/pitboard/internal/report"
)

// ---- rendering ----

type row struct {
	line  string
	key   string // set on the first line of a stop
	isCur bool
	cont  bool // the rest of a line wrapped (or its right part below it)
	// jump is set on the first row of a place g goes to (a Section's
	// heading, a top-level Fold, a notice).
	jump *report.Node
}

func (a *app) textWidth() int {
	return max(min(maxText, a.t.w-lgutter-margin), 10)
}

func (a *app) draw() {
	if a.in != nil && !a.help {
		a.drawEditing()
		return
	}
	var rows []row
	var pins pinning
	var cur *int
	var top *int
	var follow *bool
	var shown *bool
	if a.list {
		rows = a.listRows()
		top, follow, shown = &a.ltop, &a.lfollow, &a.lcurShown
	} else {
		pv := a.top()
		pv.settleCursor()
		pins = a.pin(pv, a.t.h, "")
		rows = a.rows(pv)
		if a.jumping {
			a.markJumps(pins.rows, a.jumpTo)
			a.markJumps(rows, a.jumpTo)
		}
		top, follow, shown, cur = &pv.top, &pv.follow, &pv.curShown, &pv.curRow
	}
	if a.help {
		rows = a.helpRows()
		top, follow, shown, cur = &a.htop, new(bool), new(bool), nil
	}
	ci := -1
	for i, r := range rows {
		if r.isCur {
			ci = i
		}
	}
	// The bottom lines say what works on the cursor's line, which depends on
	// whether the cursor is on screen, which depends on how many lines the
	// bottom takes. Lay out with the last guess, then once more if it was
	// wrong.
	top0, follow0 := *top, *follow
	// The AI's notices are on the top lines, however far the page is
	// scrolled. The cursor on one is always on screen.
	var banner []string
	onNotice := false
	if !a.list && !a.help {
		for _, r := range pins.rows {
			if len(banner) < max(a.t.h-2, 0) {
				banner = append(banner, r.line)
				onNotice = onNotice || r.isCur
			}
		}
	}
	// What the person has not sent, and what the AI has not read yet, on a
	// line of their own at the top of the page however far it is scrolled
	// (never over a line of the report). At the top of the page, that is
	// the blank line the page starts with.
	mark := a.sendMark()
	head, off := 0, 0
	if !a.help && len(mark) > 0 {
		head = 1
		if len(rows) > 0 && rows[0].line == "" && !rows[0].isCur {
			// Laid out without that line: positions are kept as among
			// all the rows.
			rows = rows[1:]
			ci--
			off = 1
			top0 = max(top0-1, 0)
			defer func(top *int) { *top++ }(top)
		}
	}
	// Everything below lays out in what the banner leaves.
	fullH := a.t.h
	a.t.h -= len(banner) + head
	defer func() { a.t.h = fullH }()
	var bottom []string
	for range 2 {
		guess := len(bottom)
		if guess == 0 {
			guess = len(a.bottomLines(len(rows)))
		}
		h := max(a.t.h-guess, 1)
		*top, *follow = top0, follow0
		if ci >= 0 && *follow {
			*follow = false
			// The whole of the cursor's line (a wrapped title) comes into
			// view; its head when it is taller than the screen.
			ce := ci
			for ce+1 < len(rows) && rows[ce+1].cont {
				ce++
			}
			if ce >= *top+h-1 {
				*top = ce - h + 2
			}
			if ci < *top+1 {
				*top = max(ci-1, 0)
			}
		}
		*top = max(min(*top, len(rows)-h), 0)
		*shown = (ci >= *top && ci < *top+h) || onNotice
		bottom = a.bottomLines(len(rows))
		if len(bottom) == guess {
			break
		}
	}
	if cur != nil {
		*cur = ci
		if ci >= 0 {
			*cur = ci + off
		}
		if pv := a.top(); pv.auto != "" {
			switch {
			case pv.cur != pv.auto:
				pv.auto = "" // the person moved on
			case pv.curShown && pv.autoAt.IsZero():
				pv.autoAt = clock() // the line is on screen from now
			}
		}
	}
	h := max(a.t.h-len(bottom), 1)
	if cur != nil {
		a.top().atEnd = *top+h >= len(rows)
		a.top().viewH = h
	}
	if a.tauto != 0 {
		on, shown := 0, true
		switch {
		case a.list:
			on, shown = a.lcur, a.lcurShown
		case a.inPane:
			on = a.pcur
		}
		switch {
		case on != a.tauto:
			a.tauto = 0 // the person moved on
		case shown && a.tautoAt.IsZero() && !a.help:
			a.tautoAt = clock() // on screen from now
		}
	}
	lines := make([]string, 0, a.t.h)
	for i := *top; i < len(rows) && len(lines) < h; i++ {
		lines = append(lines, rows[i].line)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if more := len(rows) - (*top + h); more > 0 && !a.list && !a.help && a.in == nil && len(a.paneOrder()) > 0 && len(bottom) > 0 {
		// The comments below cover the end of the report: say there is
		// more, faintly, on the blank line above them (or the last line).
		say := func(n int) string {
			return a.compose(false, 0, nil, plainText(fmt.Sprintf("↓ あと %d 行", n), dim), nil)
		}
		if bottom[0] == "" {
			bottom[0] = say(more)
		} else if len(lines) > 1 {
			lines[len(lines)-1] = say(more + 1)
		}
	}
	if head > 0 {
		lines = append([]string{a.overlayRight("", mark)}, lines...)
	}
	var at *[2]int
	if a.in != nil && !a.help {
		// The terminal's own cursor goes where the person writes, so an
		// input method shows what is being composed there.
		if r := h + a.inAt[0]; r < a.t.h {
			at = &[2]int{len(banner) + head + r + 1, a.inAt[1] + 1}
		}
	}
	a.t.h = fullH
	a.t.draw(append(append(banner, lines...), bottom...), at)
}

// sendMark is the top right: "送信済 N" (sent, not read by the AI yet;
// faint) and "● 未送信 N" (not sent; yellow), after the counts of marks.
// What does not fit in the width goes, from the right.
func (a *app) sendMark() text {
	var t text
	room := a.textWidth() - 2
	add := func(s, color string) {
		part := plainText(s, color)
		gap := 0
		if len(t) > 0 {
			gap = 2
		}
		if t.width()+gap+part.width() > room {
			room = -1 // and nothing after it either
			return
		}
		if gap > 0 {
			t = append(t, plainText("  ", "")...)
		}
		t = append(t, part...)
	}
	if w, f := a.counts(a.top()); w > 0 || f > 0 {
		if w > 0 {
			add(fmt.Sprintf("反応待ち %d", w), yellow)
		}
		if f > 0 {
			add(fmt.Sprintf("未読 %d", f), green)
		}
	}
	if a.b.Waiter {
		add("AI 待機中", dim)
	}
	if a.b.Unread > 0 {
		add(fmt.Sprintf("送信済 %d", a.b.Unread), dim)
	}
	if a.b.Unsent > 0 {
		add(fmt.Sprintf("● 未送信 %d", a.b.Unsent), yellow)
	}
	return t
}

// overlayRight puts right at the right end of an already drawn line, where
// compose puts a line's right part. A line too long for both is cut with ….
func (a *app) overlayRight(line string, right text) string {
	end := lgutter + a.textWidth() // the column right ends at
	avail := end - right.width() - 2
	if w := visibleWidth(line); w <= avail {
		return line + strings.Repeat(" ", max(end-right.width()-w, 2)) + right.String()
	}
	cut := cutVisible(line, max(avail-1, 0))
	return cut + "…" + strings.Repeat(" ", max(end-right.width()-visibleWidth(cut)-1, 2)) + right.String()
}

// visibleWidth is how many cells a drawn line takes (escape sequences take
// none).
func visibleWidth(s string) int {
	w := 0
	forVisible(s, func(_ int, r rune) bool { w += runeWidth(r); return true })
	return w
}

// cutVisible keeps the first w cells of a drawn line, and ends any style
// or hyperlink it was in.
func cutVisible(s string, w int) string {
	n, end := 0, 0
	forVisible(s, func(i int, r rune) bool {
		if n+runeWidth(r) > w {
			return false
		}
		n += runeWidth(r)
		end = i + len(string(r))
		return true
	})
	// Keep the escape sequences right after the last kept rune too (they
	// may close a style), then reset everything.
	return s[:end] + rst + "\x1b]8;;\x1b\\"
}

// forVisible calls f with each rune of s that is drawn (not part of an
// escape sequence) and its byte offset, until f returns false.
func forVisible(s string, f func(i int, r rune) bool) {
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) {
			switch s[i+1] {
			case '[': // CSI: up to a final byte
				j := i + 2
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				i = j + 1
				continue
			case ']': // OSC: up to ESC \ or BEL
				j := i + 2
				for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
					j++
				}
				if j < len(s) && s[j] == 0x1b {
					j++
				}
				i = j + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !f(i, r) {
			return
		}
		i += size
	}
}

// compose lays out one screen line: the left gutter (holding the cursor
// mark), indent, the 2-cell mark field, the content, and an optional
// right-aligned part (status, reaction, thoughts).
func (a *app) compose(selected bool, indent int, mark, body, right text) string {
	w := a.textWidth()
	gutter := strings.Repeat(" ", lgutter)
	if selected {
		gutter = ">" + strings.Repeat(" ", lgutter-1)
	}
	if len(right) > 0 {
		right = right.trunc(max(w-indent-markW-8, 4))
	}
	body = body.trunc(a.room(indent, right))
	s := gutter + strings.Repeat(" ", indent) + mark.String() + strings.Repeat(" ", max(markW-mark.width(), 0)) + body.String()
	if len(right) > 0 {
		gap := w - indent - markW - body.width() - right.width()
		s += strings.Repeat(" ", max(gap, 1)) + right.String()
	}
	return s
}

// room is the width left for the content of a line.
func (a *app) room(indent int, right text) int {
	r := a.textWidth() - indent - markW
	if len(right) > 0 {
		r -= right.width() + 2
	}
	return max(r, 4)
}

func (a *app) para(indent int, t text) []row {
	var out []row
	for _, ln := range t.wrap(a.room(indent, nil)) {
		out = append(out, row{line: a.compose(false, indent, nil, ln, nil)})
	}
	return out
}

// line draws one part: its first row carries the mark and the right part,
// and the rest of the text follows on rows indented to it.
func (a *app) line(sel bool, indent int, mark, body, right text) []row {
	return a.titled(sel, indent, 0, mark, body, right, 0)
}

// titleCap is the most of a title drawn (cut with … past it), so that no
// title costs more than a few screens' worth of work however long it is.
const titleCap = 2000

// minTitle is the least room a title keeps beside its right part.
const minTitle = 8

// rightFits says whether right goes at the end of a line's first row: it
// takes at most half the width and leaves the title room. Otherwise it goes
// on rows of its own under the title. Either way it is never cut.
func (a *app) rightFits(indent int, right text) bool {
	rw := right.width()
	return rw <= a.textWidth()/2 && rw+2+minTitle <= a.textWidth()-indent-markW
}

// rightRows is a right part too wide to share the title's row: right
// aligned on rows of its own (wrapped if it is wider than a row).
func (a *app) rightRows(right text) []row {
	var out []row
	for _, ln := range right.wrap(a.textWidth() - markW - minTitle) {
		out = append(out, row{line: a.compose(false, 0, nil, nil, ln), cont: true})
	}
	return out
}

// titled draws a title that wraps instead of being cut. The right part
// stays whole at the end of the first row, and every row of the title
// leaves its column free, so the right parts read as one column. The rows
// after the first start where the title's text starts (extra: how far a
// list number reaches past the mark field). most > 0 keeps that many rows
// of the title at most, the last ending in …, the right part included
// (on one row of its own, or left out). The cursor and the marks go
// on the first row only.
func (a *app) titled(sel bool, indent, extra int, mark, body, right text, most int) []row {
	var below []row
	if len(right) > 0 && !a.rightFits(indent, right) {
		below, right = a.rightRows(right), nil
		if most > 0 {
			// Cut to fit a low screen: the right part takes one row of its
			// own out of most, or is left out when that cannot be.
			if len(below) == 1 && most >= 2 {
				most--
			} else {
				below = nil
			}
		}
	}
	w := a.room(indent, right) - extra
	lns := body.wrap(w)
	if most > 0 && len(lns) > most {
		last := append(append(text{}, lns[most-1]...), lns[most]...)
		lns = append(lns[:most-1:most-1], last.trunc(max(w, 4)))
	}
	out := []row{{line: a.compose(sel, indent, mark, lns[0], right), isCur: sel}}
	for _, ln := range lns[1:] {
		out = append(out, row{line: a.compose(false, indent+extra, nil, ln, nil), cont: true})
	}
	return append(out, below...)
}

func join(parts ...text) text {
	var out text
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, plainText("  ", "")...)
		}
		out = append(out, p...)
	}
	return out
}
