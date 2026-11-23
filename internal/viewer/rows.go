package viewer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// toneColor is the color of a Tone: yellow needs the person, green moves,
// faint waits or is done.
func toneColor(t page.Tone) string {
	switch t {
	case page.ToneNeed:
		return yellow
	case page.ToneMove:
		return green
	}
	return dim
}

// wordText is a Word drawn in its color.
func wordText(w page.Word) text {
	if w.Text == "" {
		return nil
	}
	return plainText(w.Text, toneColor(w.Tone))
}

// right is what goes on the right end of a line (page.Right): the state of
// the person's latest input on it (a reaction, or a comment), then the
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
	case "y":
		re = plainText("y", bold+green)
	case "n":
		re = plainText("n", bold+red)
	default:
		re = plainText(r.Reaction, bold+cyan)
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
		if long := join(mine(head), st); inline(n.Text, "").width() <= a.room(ind, long) {
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
const answerHead = 12

// markOf is the mark at the head of a line. Done items always show ✓;
// otherwise opening wins over the kind.
func markOf(k report.Kind, canOpen, open bool) text {
	switch {
	case k == report.KindLink:
		return plainText("→", blue)
	case k == report.KindDone:
		return plainText("✓", dim)
	case k == report.KindQuestion:
		return plainText("?", yellow) // even when a long question opens
	case canOpen && open:
		return plainText("▾", "")
	case canOpen:
		return plainText("▸", "")
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
		rows = append(rows, a.para(0, plainText("読めませんでした: "+pv.Err, magenta))...)
		blank()
	}
	if pv.Err == "" && pv.ReadErr == "" && len(pv.Nodes) == 0 && len(pv.Probs) == 0 {
		return append(rows, a.para(0, plainText(msgEmpty, dim))...)
	}
	if pv.ReadErr != "" {
		rows = append(rows, a.para(0, plainText(pv.ReadErr, magenta+bold))...)
		blank()
	}
	if len(pv.Probs) > 0 {
		head := page.MsgProblems
		if report.Partial(pv.Probs) {
			head += page.MsgPartial
		}
		rows = append(rows, a.para(0, plainText(head, magenta+bold))...)
		for _, pr := range pv.Probs {
			rows = append(rows, a.para(0, plainText(page.ProblemLine(pr), magenta))...)
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
		sel := n.Key != "" && n.Key == pv.cur && !a.inPane
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
			rows = append(rows, a.para(0, inline(n.Text, bold))...)
			rows = append(rows, row{})
		case report.KindSection:
			blank()
			h := a.para(0, inline(n.Text, bold+blue))
			h[0].jump = n
			rows = append(rows, h...)
		case report.KindPara:
			blank()
			rows = append(rows, a.para(ind, paraText(n, ""))...)
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
				mark = plainText("●", bold+cyan)
			}
			lns := inline(n.Text, "").wrap(a.room(ind, nil) - label.width())
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
				base = blue
			}
			hidden := pv.isHidden(n) // drawn only while H shows hidden lines
			heading := outer[n] && !hidden
			switch {
			case hidden:
				base = dim
			case heading:
				base = bold + blue
			}
			body, right := inline(n.Text, base).capped(titleCap), a.right(pv, n)
			nav := pv.navigable(n)
			// Titles wrap; nothing is cut, so a line opens only for what it
			// holds.
			can := nav && a.canOpen(pv, n)
			if can && open {
				// Open: the title in bold, so it stands apart from what it
				// holds (in the normal weight).
				body = inline(n.Text, base+bold).capped(titleCap)
			}
			// A line the cursor cannot stop at cannot be opened either, so
			// what it holds is always shown.
			shown := (can && open) || (!nav && n.Kind != report.KindDetails)
			mark := markOf(n.Kind, can, open)
			switch {
			case hidden:
				mark = plainText("⌀", dim)
			case heading:
				mark = plainText(mark.plain(), bold+blue)
			}
			var lns []row
			if n.Kind == report.KindPlain {
				if !hidden {
					mark = nil // the list's own mark
				}
				lns = a.listItem(sel, ind, n, base, mark, right)
			} else if q := a.b.Quote(n); q != nil {
				lns = a.quoted(sel, ind, mark, page.QuoteText(q.Text), base, body, right)
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
					c = a.thoughtRows(n, ind+4)
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

// quoted is an Item that answers a comment (re="N"), as page.QuoteText
// says: its first row (the cursor's, with the mark and the right part) is
// > and the comment, in the color of the title, wrapped and page.QuoteRows
// rows at most (the last ending in …); the title follows on rows of its
// own, starting where the comment's text starts, wrapped.
func (a *app) quoted(sel bool, ind int, mark text, q, base string, title, right text) []row {
	head := plainText("> ", base)
	var below []row
	if len(right) > 0 && !a.rightFits(ind, right) {
		below, right = a.rightRows(right), nil
	}
	w := max(a.room(ind, right)-head.width(), 4)
	lns := withImages(q, base).capped(titleCap).wrap(w)
	if len(lns) > page.QuoteRows {
		last := append(append(text{}, lns[page.QuoteRows-1]...), plainText(" ", base)...)
		last = append(last, lns[page.QuoteRows]...)
		lns = append(lns[:page.QuoteRows-1:page.QuoteRows-1], last.trunc(w))
	}
	out := []row{{line: a.compose(sel, ind, mark, append(append(text{}, head...), lns[0]...), right), isCur: sel}}
	for _, l := range lns[1:] {
		out = append(out, row{line: a.compose(false, ind+head.width(), nil, l, nil), cont: true})
	}
	out = append(out, below...)
	for _, l := range title.wrap(a.room(ind+head.width(), nil)) {
		out = append(out, row{line: a.compose(false, ind+head.width(), nil, l, nil), cont: true})
	}
	return out
}

// paraText is a paragraph as drawn: a small heading is bold in the color of
// section headings.
func paraText(n *report.Node, base string) text {
	if n.Head {
		return inline(n.Text, base+bold+blue)
	}
	return inline(n.Text, base)
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
	if len(mark) == 0 {
		mark = listMark(n)
	}
	extra := max(mark.width()-markW, 0)
	out := a.titled(sel, ind, extra, mark, inline(n.Text, base), right, 0)
	for _, s := range n.Subs {
		out = append(out, a.listItem(false, ind+markW+extra, s, base, nil, nil)...)
	}
	return out
}

// code draws a fenced block. Long lines wrap; a diff is colored.
func (a *app) code(n *report.Node, ind int) []row {
	var rows []row
	for _, ln := range n.Code {
		st := cyan
		if n.Lang == "diff" {
			switch {
			case strings.HasPrefix(ln, "@@"):
				st = cyan
			case strings.HasPrefix(ln, "+"):
				st = green
			case strings.HasPrefix(ln, "-"):
				st = red
			default:
				st = ""
			}
		}
		for _, w := range plainText(ln, st).wrap(a.room(ind, nil)) {
			rows = append(rows, row{line: a.compose(false, ind, nil, w, nil)})
		}
	}
	return rows
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
		rows = append(rows, a.para(ind, append(plainText("理由: ", dim), inline(n.Reason, base)...))...)
	}
	for _, b := range n.Body {
		// A blank line in the text stays one; a small heading always has
		// one above it, unless it comes first.
		if len(rows) > 0 && (b.Gap || b.Head) {
			rows = append(rows, row{})
		}
		switch b.Kind {
		case report.KindPara:
			rows = append(rows, a.para(ind, paraText(b, base))...)
		case report.KindCode:
			rows = append(rows, a.code(b, ind+markW)...)
		default: // list lines
			rows = append(rows, a.listItem(false, ind+markW, b, base, nil, nil)...)
		}
	}
	return append(rows, a.thoughtRows(n, ind+markW)...)
}

// thoughtRows is the thoughts written on n, as shown under it when it is
// open.
func (a *app) thoughtRows(n *report.Node, ind int) []row {
	var rows []row
	if id := report.Target(n); id != "" {
		for _, t := range a.b.State.On(id) {
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
		}
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
