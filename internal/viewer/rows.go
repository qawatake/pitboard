package viewer

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// right is what goes on the right end of a line: the state of the
// person's latest input on it (a reaction, or a comment), then the status
// the AI wrote. Colors mean: yellow needs the person (not sent, wait="you"),
// green moves (a doing Item's status), faint waits or is done.
func (a *app) right(pv *view, n *report.Node) text {
	var st text
	switch {
	case n.Status == "" && n.Kind == report.KindDoing:
		// The mark at the head no longer tells how far it is: the words do.
		st = plainText("作業中", green)
	case n.Status == "":
	case n.Wait == "you":
		st = plainText(n.Status, yellow)
	case n.Kind == report.KindDoing:
		st = plainText(n.Status, green)
	default:
		st = plainText(n.Status, dim)
	}
	if n.Kind == report.KindNotice && !n.At.IsZero() {
		st = plainText(atWord(n.At), dim) // when the AI put it up
	}
	id := report.Target(n)
	if id == "" {
		return st
	}
	// The reaction, while the line is what it was reacted to.
	var re text
	e, reacted := a.state.React[id]
	reacted = reacted && e.Line == n.Sig()
	if reacted {
		switch e.Value {
		case "y":
			re = plainText("y", bold+green)
		case "n":
			re = plainText("n", bold+red)
		default:
			re = plainText(e.Value, bold+cyan)
		}
	}
	// The state of the latest input: the reaction's, or the latest
	// comment's (with how many comments there are, when more than one).
	cs := a.state.On(id)
	var last *board.Thought
	for _, t := range cs {
		if last == nil || t.LastN > last.LastN {
			last = t
		}
	}
	var word text
	switch {
	case a.state.Undoing[id] || a.state.UndoingKey[report.HideKey(n)]:
		// A sent reaction (or hide) taken back, the taking back not sent.
		re, word = nil, plainText("取消 未送信", yellow)
	case reacted && (last == nil || e.N > last.LastN):
		w, color := statusWord(a.state.SentStatus(e.N)), dim
		if w == "未送信" {
			color = yellow
		}
		word = plainText(w, color)
	case last != nil:
		w, color := a.statusOf(last)
		word = plainText(w, color)
		if len(cs) > 1 {
			word = append(plainText(fmt.Sprintf("%d ", len(cs)), dim), word...)
		}
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
	if n.Kind == report.KindQuestion && len(re) > 0 && !pv.open[n.Key] {
		// A closed question shows the head of the chosen option too, if
		// the line has room for it.
		if ch := n.Choice(re.plain()); ch != nil {
			head := append(append(text{}, re...), plainText(" ", "")...)
			head = append(head, plainText(report.PlainOf(ch.Text), dim).trunc(answerHead)...)
			ind := n.Depth * 2
			if long := join(mine(head), st); inline(n.Text, "").width() <= a.room(ind, long) {
				return long
			}
		}
	}
	return join(mine(re), st)
}

// atWord is when a notice was put up: the time today, else the day too.
func atWord(t time.Time) string {
	now := clock()
	if y, m, d := now.Date(); t.Year() == y && t.Month() == m && t.Day() == d {
		return t.Format("15:04")
	}
	return t.Format("1/2 15:04")
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
	case a.noSession:
		return append(rows, a.para(0, plainText(fmt.Sprintf("セッション %s の報告はまだありません", a.st.Session), dim))...)
	case pv.missing:
		return append(rows, a.para(0, plainText(msgNoReport, dim))...)
	case pv.err != "":
		rows = append(rows, a.para(0, plainText("読めませんでした: "+pv.err, magenta))...)
		blank()
	}
	if pv.err == "" && pv.readErr == "" && len(pv.nodes) == 0 && len(pv.probs) == 0 {
		return append(rows, a.para(0, plainText(msgEmpty, dim))...)
	}
	if pv.readErr != "" {
		rows = append(rows, a.para(0, plainText(pv.readErr, magenta+bold))...)
		blank()
	}
	if len(pv.probs) > 0 {
		head := "報告に書き方の誤りがあります"
		if report.Partial(pv.probs) {
			head += " (読めた所までを出しています)"
		}
		rows = append(rows, a.para(0, plainText(head, magenta+bold))...)
		for _, pr := range pv.probs {
			rows = append(rows, a.para(0, plainText(fmt.Sprintf("%d 行目: %s", pr.Line, pr.Msg), magenta))...)
		}
		blank()
	}
	chosen := "" // the answered Option of the Ask being drawn
	var curAsk *report.Node
	pv.ends = map[string]int{}
	pv.walk(func(n *report.Node) {
		sel := n.Key != "" && n.Key == pv.cur && !a.inPane
		ind := n.Depth * 2
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
			rows = append(rows, a.para(0, inline(n.Text, bold+blue))...)
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
				if e, ok := a.state.React[n.ID]; ok && n.ID != "" && e.Line == n.Sig() {
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
			if hidden {
				base = dim
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
			if hidden {
				mark = plainText("⌀", dim)
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
					c = a.thoughtRows(n, ind+2)
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
		for _, t := range a.state.On(id) {
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
