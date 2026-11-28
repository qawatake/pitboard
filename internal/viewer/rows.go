package viewer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// toneColor is the color of a Tone: yellow needs the person, green moves,
// faint waits or is done.
func toneColor(t page.Tone) string { return palette[page.ToneColor(t)] }

// wordText is a Word drawn in its color.
func wordText(w page.Word) text {
	if w.Text == "" {
		return nil
	}
	return plainText(w.Text, toneColor(w.Tone))
}

// right is what goes on the right end of a line (page.Right): the state of
// the person's latest input on it (an answer, or a comment), then the
// status the AI wrote.
func (a *app) right(pv *view, n *report.Node) text {
	r := a.b.RightOf(n)
	st := wordText(r.Status)
	if !r.At.IsZero() {
		st = plainText(page.AtWord(r.At, clock()), dim) // when the AI put it up
	}
	var re text
	switch r.Reaction {
	case "":
	default:
		re = plainText(r.Reaction, bold+cYours)
	}
	word := wordText(r.Input)
	if r.Count > 0 {
		word = append(plainText(fmt.Sprintf("%d ", r.Count), dim), word...)
	}
	mine := func(re text) text {
		switch {
		case len(re) == 0:
			return word
		case len(word) == 0:
			return re
		}
		return append(append(append(text{}, re...), plainText(" ", "")...), word...)
	}
	if n.Kind == report.KindQuestion && len(re) > 0 && r.Option != "" && !pv.open[n.Key] {
		// A closed question shows the head of the chosen option too, if
		// the line has room for it.
		head := append(append(text{}, re...), plainText(" ", "")...)
		head = append(head, plainText(r.Option, dim).trunc(answerHead)...)
		ind := indentOf(n)
		if long := join(mine(head), st); a.inline(n.Text, "").width() <= a.room(ind, long) {
			return long
		}
	}
	return join(mine(re), st)
}

// indentOf is how far n's mark is indented: what a Fold holds goes 4 past
// the Fold's mark (2 past the first character of its title), each Fold
// within a Fold as much again; an Ask's options go 2 past its mark.
func indentOf(n *report.Node) int {
	if n.Kind == report.KindChoice {
		return max(n.Depth-1, 0)*4 + 2
	}
	return n.Depth * 4
}

// answerHead is how much of the chosen option a closed question shows.
const answerHead = page.AnswerHead

// markOf is the mark at the head of a line. Done items always show ✓;
// otherwise opening wins over the kind.
func markOf(k report.Kind, canOpen, open bool) text {
	switch {
	case k == report.KindLink:
		return plainText("→", cStructure)
	case k == report.KindDone:
		return plainText("✓", dim)
	case k == report.KindQuestion:
		return plainText("?", cNeed) // even when a long question opens
	case canOpen && open:
		return plainText(page.SignOpen, bold)
	case canOpen:
		return plainText(page.SignClosed, "")
	case k == report.KindDoing, k == report.KindTodo:
		// Nothing to open: a quiet dot. How far it is goes at the right end.
		return plainText("·", dim)
	case k == report.KindPlain:
		return plainText("·", dim)
	}
	return nil
}

func (a *app) rows(pv *view) []row {
	var rows []row
	blank := func() {
		if len(rows) > 0 && rows[len(rows)-1].line != "" {
			rows = append(rows, row{})
		}
	}
	rows = append(rows, row{}) // top padding
	switch {
	case a.st == nil && !isDir(filepath.Join(a.base, ".pitboard")):
		return append(rows, a.para(0, plainText(msgNoBoard, dim))...)
	case a.st == nil:
		return append(rows, a.para(0, plainText(msgNoSession, dim))...)
	case a.b.NoSession:
		return append(rows, a.para(0, plainText(page.MsgNoSuchSession(a.st.Session), dim))...)
	case pv.Missing:
		return append(rows, a.para(0, plainText(msgNoReport, dim))...)
	case pv.Err != "":
		rows = append(rows, a.para(0, plainText("読めませんでした: "+pv.Err, cBad))...)
		blank()
	}
	if pv.Err == "" && pv.ReadErr == "" && len(pv.Nodes) == 0 && len(pv.Probs) == 0 {
		return append(rows, a.para(0, plainText(msgEmpty, dim))...)
	}
	if pv.ReadErr != "" {
		rows = append(rows, a.para(0, plainText(pv.ReadErr, cBad+bold))...)
		blank()
	}
	if pv.Fixing {
		rows = append(rows, a.para(0, plainText(page.MsgFixing, dim))...)
		blank()
	}
	if len(pv.Probs) > 0 {
		head := page.MsgProblems
		if report.Partial(pv.Probs) {
			head += page.MsgPartial
		}
		rows = append(rows, a.para(0, plainText(head, cBad+bold))...)
		for _, pr := range pv.Probs {
			rows = append(rows, a.para(0, plainText(page.ProblemLine(pr), cBad))...)
		}
		blank()
	}
	// A Fold at the top level is a division of the page as a Section is:
	// its title is drawn as a Section's heading.
	outer := map[*report.Node]bool{}
	for _, n := range pv.Nodes {
		if n.Kind == report.KindDetails {
			outer[n] = true
		}
	}
	chosen := "" // the answered Option of the Ask being drawn
	var curAsk *report.Node
	pv.ends = map[string]int{}
	pv.walk(func(n *report.Node) {
		sel := n.Key != "" && n.Key == pv.cur
		ind := indentOf(n)
		open := pv.open[n.Key]
		switch n.Kind {
		case report.KindNotice:
			// Pinned above the page (pinned), unless opened too tall to
			// pin: then the head of the page.
			if a.unpinned[n.Key] {
				rows = append(rows, a.noticeHead(pv, n, 0)...)
				c := a.noticeBody(pv, n)
				rows = append(rows, c...)
				pv.ends[n.Key] = len(rows)
				blank()
			}
		case report.KindTitle:
			rows = append(rows, a.para(0, a.inline(n.Text, bold))...)
			rows = append(rows, row{})
		case report.KindSection:
			blank()
			h := a.para(0, a.inline(n.Text, bold+cStructure))
			h[0].jump = n
			rows = append(rows, h...)
			if page.EmptySection(n) {
				rows = append(rows, a.para(0, plainText(page.MsgNone, dim))...)
			}
		case report.KindPara:
			blank()
			rows = append(rows, a.para(ind, a.paraText(n, ""))...)
			rows = append(rows, row{})
		case report.KindCode:
			blank()
			rows = append(rows, a.code(n, ind+2)...)
			rows = append(rows, row{})
		case report.KindChoice:
			defer func() {
				// After the last option of an open Ask, a blank line.
				if curAsk != nil && len(curAsk.Kids) > 0 && curAsk.Kids[len(curAsk.Kids)-1] == n {
					blank()
				}
			}()
			label := plainText(n.Label+"  ", dim)
			var mark text
			if chosen != "" && n.Label == chosen {
				mark = plainText("●", bold+cYours)
			}
			lns := a.inline(n.Text, "").wrap(a.room(ind, nil) - label.width())
			rows = append(rows, row{line: a.compose(false, ind, mark, append(label, lns[0]...), nil)})
			for _, ln := range lns[1:] {
				rows = append(rows, row{line: a.compose(false, ind+label.width(), nil, ln, nil)})
			}
		default: // todo, done, doing, question, plain, link, details
			if (n.Kind == report.KindDetails && n.Depth == 0) || (n.Kind == report.KindPlain && n.Gap) {
				blank()
			}
			if n.Kind == report.KindQuestion {
				chosen, curAsk = "", n
				if e, ok := a.b.State.React[n.ID]; ok && n.ID != "" && e.Line == n.Sig() {
					chosen = e.Value
				}
			}
			base := ""
			switch n.Kind {
			case report.KindDone:
				base = dim
			case report.KindLink:
				base = cStructure
			}
			// While g waits, the cursor line's links carry their digits.
			if a.jumping && n.Key == pv.cur && len(a.jumpLinks) > 0 {
				a.linkNums = a.b.LinkNums(n)
				defer func() { a.linkNums = nil }()
			}
			hidden := pv.isHidden(n) // drawn only while H shows hidden lines
			heading := outer[n] && !hidden
			switch {
			case hidden:
				base = dim
			case heading:
				base = bold + cStructure
			}
			body, right := a.inline(n.Text, base).capped(titleCap), a.right(pv, n)
			nav := pv.navigable(n)
			// Titles wrap; nothing is cut, so a line opens only for what it
			// holds.
			can := nav && a.canOpen(pv, n)
			if can && open {
				// Open: the title in bold, so it stands apart from what it
				// holds (in the normal weight).
				body = a.inline(n.Text, base+bold).capped(titleCap)
			}
			// A line the cursor cannot stop at cannot be opened either, so
			// what it holds is always shown.
			shown := (can && open) || (!nav && n.Kind != report.KindDetails)
			mark := markOf(n.Kind, can, open)
			switch {
			case hidden:
				mark = plainText("⌀", dim)
			case heading:
				mark = plainText(mark.plain(), bold+cStructure) // open or not, as the heading
			}
			var lns []row
			if n.Kind == report.KindPlain {
				if !hidden {
					mark = nil // the list's own mark
				}
				lns = a.listItem(sel, ind, n, base, mark, right)
			} else {
				lns = a.line(sel, ind, mark, body, right)
			}
			if nav {
				lns[0].key = n.Key
				lns[0].line = gutterMark(lns[0].line, a.markOfLine(pv, n))
			}
			if n.Kind == report.KindDetails {
				lns[0].jump = n
			}
			rows = append(rows, lns...)
			if n.Kind == report.KindLink {
				rows = append(rows, a.linkTarget(n, ind)...)
			}
			if shown {
				// What it holds, 2 past the title's text (4 past its mark),
				// and a blank line after it when opened.
				var c []row
				if n.Kind == report.KindDetails {
					// An open Fold: the thoughts on it right under its
					// title, among what it holds (marks in their column).
					c = a.thoughtRows(pv, n, ind+4)
				} else {
					c = a.contents(pv, n, ind+2)
				}
				rows = append(rows, c...)
				if nav {
					pv.ends[n.Key] = len(rows)
				}
				if can && open && len(c) > 0 && n.Kind != report.KindDetails {
					blank()
				}
			} else if nav {
				pv.ends[n.Key] = len(rows)
			}
		}
	})
	return rows
}

// paraText is a paragraph as drawn: a small heading is bold in the color of
// section headings.
func (a *app) paraText(n *report.Node, base string) text {
	if n.Head {
		return a.inline(n.Text, base+bold+cStructure)
	}
	return a.inline(n.Text, base)
}

// listMark is the mark of a list line: its number, or · - · by level.
func listMark(n *report.Node) text {
	switch {
	case n.Num != "":
		return plainText(n.Num+" ", "")
	case n.Level == 1:
		return plainText("-", dim)
	}
	return plainText("·", dim)
}

// listItem draws a list line and the lines nested under it. Its text
// wraps to where it starts (past a number wider than the mark field), and
// each level is indented 2 more. mark replaces the list's own mark if set.
func (a *app) listItem(sel bool, ind int, n *report.Node, base string, mark, right text) []row {
	rows, _ := a.listItemOwn(sel, ind, n, base, mark, right)
	return rows
}

// listItemOwn is listItem, and how many rows its own text takes (its
// sub-items left out): what makes its list loose (page.Loose).
func (a *app) listItemOwn(sel bool, ind int, n *report.Node, base string, mark, right text) ([]row, int) {
	if len(mark) == 0 {
		mark = listMark(n)
	}
	extra := max(mark.width()-markW, 0)
	var out []row
	if len(right) > 0 || sel {
		out = a.titled(sel, ind, extra, mark, a.inline(n.Text, base), right, 0)
	} else {
		out = a.measured(ind, extra, mark, n.Text, base)
	}
	own := len(out)
	return append(out, a.listLines(ind+markW+extra, n.Subs, base)...), own
}

// listLines draws the lines of one list (one level under one parent): with
// a blank row between all of them when any takes more than one row
// (page.Loose). A nested list decides for itself.
func (a *app) listLines(ind int, ls []*report.Node, base string) []row {
	var parts [][]row
	var lines []int
	for _, s := range ls {
		r, own := a.listItemOwn(false, ind, s, base, nil, nil)
		parts, lines = append(parts, r), append(lines, own)
	}
	loose := page.Loose(lines)
	var out []row
	for i, p := range parts {
		if loose && i > 0 {
			out = append(out, row{})
		}
		out = append(out, p...)
	}
	return out
}

// measured draws text s (inline Markdown) of a list line or a paragraph
// after mark, wrapped at the room left but no wider than page.TextMeasure.
// A line written "**head**: body" that does not fit on one row is drawn
// as a definition: the head alone on the first row (bold, no colon), the
// body under it, one step in (page.HeadWord).
func (a *app) measured(indent, extra int, mark text, s, base string) []row {
	w := max(min(a.room(indent, nil)-extra, page.TextMeasure), 4)
	all := a.inline(s, base)
	lns := all.wrap(w)
	if head, body, ok := page.HeadWord(s); ok && len(lns) > 1 {
		out := []row{{line: a.compose(false, indent, mark, a.inline("**"+head+"**", base), nil)}}
		for _, ln := range a.inline(body, base).wrap(max(w-markW, 4)) {
			out = append(out, row{line: a.compose(false, indent+extra+markW, nil, ln, nil), cont: true})
		}
		return out
	}
	out := []row{{line: a.compose(false, indent, mark, lns[0], nil)}}
	for _, ln := range lns[1:] {
		out = append(out, row{line: a.compose(false, indent+extra, nil, ln, nil), cont: true})
	}
	return out
}

// code draws a fenced block. Long lines wrap; a diff is colored.
func (a *app) code(n *report.Node, ind int) []row {
	var rows []row
	for _, ln := range n.Code {
		// Tabs as they are drawn (clean: 4 spaces each), so the hang below
		// is counted in the columns the line takes on screen.
		ln = strings.ReplaceAll(ln, "\t", "    ")
		st := ""
		if n.Lang == "diff" {
			switch {
			case strings.HasPrefix(ln, "@@"):
				st = cQuiet
			case strings.HasPrefix(ln, "+"):
				st = cMove
			case strings.HasPrefix(ln, "-"):
				st = cBad
			default:
				st = ""
			}
		}
		// A wrapped line goes on under the code's own first column (past
		// its indentation, and a diff's sign), so it reads as one line.
		room := a.room(ind, nil)
		hang := min(codeHang(ln, n.Lang == "diff"), room/2)
		t := plainText(ln, st)
		first := t.wrap(room)
		if len(first) <= 1 || hang == 0 {
			for _, w := range first {
				rows = append(rows, row{line: a.compose(false, ind, nil, w, nil)})
			}
			continue
		}
		rows = append(rows, row{line: a.compose(false, ind, nil, first[0], nil)})
		rest := t[len(first[0]):]
		for len(rest) > 0 && rest[0].r == ' ' {
			rest = rest[1:] // the space the line broke at
		}
		for _, w := range rest.wrap(room - hang) {
			rows = append(rows, row{line: a.compose(false, ind+hang, nil, w, nil)})
		}
	}
	return rows
}

// codeHang is where a code line's own text starts: past its leading
// spaces, and for a diff past the sign and the spaces after it (tabs are
// expanded before: see code).
func codeHang(ln string, diff bool) int {
	i := 0
	if diff && len(ln) > 0 && strings.ContainsRune("+- ", rune(ln[0])) {
		i = 1
	}
	for i < len(ln) && (ln[i] == ' ' || ln[i] == '\t') {
		i++
	}
	if i >= len(ln) {
		return 0
	}
	return i
}

// contents is what an opened item shows under itself: the reason of its
// status, its indented lines, and the thoughts written on it.
func (a *app) contents(pv *view, n *report.Node, ind int) []row {
	var rows []row
	// What an item holds is read, so it is in the normal color; only a done
	// item's is faint, like the item.
	base := ""
	if n.Kind == report.KindDone {
		base = dim
	}
	if n.Reason != "" {
		rows = append(rows, a.para(ind, append(plainText("理由: ", dim), a.inline(n.Reason, base)...))...)
	}
	for i := 0; i < len(n.Body); i++ {
		b := n.Body[i]
		// A blank line in the text stays one; a small heading always has
		// one above it, unless it comes first.
		if len(rows) > 0 && (b.Gap || b.Head) {
			rows = append(rows, row{})
		}
		switch b.Kind {
		case report.KindPara:
			if b.Head {
				rows = append(rows, a.para(ind, a.paraText(b, base))...)
			} else {
				rows = append(rows, a.measured(ind, 0, nil, b.Text, base)...)
			}
		case report.KindCode:
			rows = append(rows, a.code(b, ind+markW)...)
		default: // list lines: the run of them up to the next blank line is one list
			j := i + 1
			for j < len(n.Body) && n.Body[j].Kind == b.Kind && !n.Body[j].Gap && !n.Body[j].Head {
				j++
			}
			rows = append(rows, a.listLines(ind+markW, n.Body[i:j], base)...)
			i = j - 1
		}
	}
	return append(rows, a.thoughtRows(pv, n, ind+markW)...)
}

// thoughtRows is the thoughts written on n, as shown under it when it is
// open: those the AI is not done with, then those done folded into one
// faint line (page.DoneFoldWord) until enter shows them (pv.shown).
func (a *app) thoughtRows(pv *view, n *report.Node, ind int) []row {
	var rows []row
	id := report.Target(n)
	if id == "" {
		return rows
	}
	var done []*board.Thought
	for _, t := range a.b.State.On(id) {
		if t.Finished() {
			done = append(done, t)
			continue
		}
		rows = append(rows, a.thoughtRow(n, t, ind)...)
	}
	switch {
	case len(done) == 0:
	case pv.shown[n.Key]:
		for _, t := range done {
			rows = append(rows, a.thoughtRow(n, t, ind)...)
		}
	default:
		rows = append(rows, a.line(false, ind, plainText("✎", dim), plainText(page.DoneFoldWord(len(done)), dim), nil)...)
	}
	return rows
}

// thoughtRow is thought t on n, as shown under it.
func (a *app) thoughtRow(n *report.Node, t *board.Thought, ind int) []row {
	var rows []row
	word, color := a.statusOf(t)
	for i, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
		if i == 0 {
			rows = append(rows, a.line(false, ind, plainText("✎", dim), withImages(l, ""), plainText(word, color))...)
		} else {
			rows = append(rows, a.line(false, ind, nil, withImages(l, ""), nil)...)
		}
	}
	if t.Title != "" && t.Title != n.Title() {
		rows = append(rows, a.line(false, ind, nil, plainText("(書いたときは: "+t.Title+")", dim).capped(8*a.textWidth()), nil)...)
	}
	if t.SettledState != "" {
		rows = append(rows, a.line(false, ind, nil, headed(settleHead(t.SettledState), t.Settled).capped(8*a.textWidth()), nil)...)
	}
	return rows
}

// linkTarget is the line under a Link: its path as written, or its URL
// (underlined, and a hyperlink where the terminal supports it). It is
// always shown and wrapped, never cut, so it can be copied.
func (a *app) linkTarget(n *report.Node, ind int) []row {
	t := plainText(n.Path, dim)
	if n.URL != "" {
		t = plainText(n.URL, dim+ul).hyperlink(n.URL)
	}
	var rows []row
	for _, l := range t.wrap(a.room(ind, nil)) {
		rows = append(rows, row{line: a.compose(false, ind, nil, l, nil)})
	}
	return rows
}
