package viewer

import (
	"fmt"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

// ---- the screens l cycles through ----

// The screens, in the order l (and tab) goes through them: the report,
// then the comments, then the report again.
const (
	viewReport = iota
	viewComments
	nViews
)

// nextView goes to the next screen. The report's cursor, scroll and folds
// stay as they are, for coming back.
func (a *app) nextView() {
	a.view = (a.view + 1) % nViews
	if a.view == viewComments {
		a.lfollow = true
	} else {
		a.top().follow = true
	}
}

// onComments says whether the comments screen is shown.
func (a *app) onComments() bool { return a.view == viewComments }

// ---- the comments screen ----

// commentBodyRows is how many rows a comment's text takes at most on the
// comments screen (o opens it whole).
const commentBodyRows = 2

// listed is the comments in the order the screen shows them: those the AI
// has not finished (newest first), then those finished (most recently
// settled first), as the browser's panel.
func (a *app) listed() []*board.Thought { return a.b.Pending() }

// openCount is how many comments the AI has not finished: the count the
// comments screen and its key say (the browser's コメント N).
func (a *app) openCount() int {
	n := 0
	for _, t := range a.b.State.Thoughts {
		if !t.Finished() {
			n++
		}
	}
	return n
}

// headed is a line with a faint head word ("行: ", "対応: ") and the
// rest in the normal color.
func headed(head, s string) text {
	return append(plainText(head, dim), plainText(s, "")...)
}

// thoughtDetails is what an opened comment shows: the line it was written
// on (and its title then), the whole text, and what the AI did with it.
func (a *app) thoughtDetails(t *board.Thought, ind int) []row {
	var rows []row
	if about := a.b.About(t); about != "" {
		rows = append(rows, a.para(ind, headed("行: ", about))...)
	}
	for _, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
		rows = append(rows, a.para(ind, withImages(l, ""))...)
	}
	if t.SettledState != "" {
		for i, l := range strings.Split(strings.TrimSpace(t.Settled), "\n") {
			if i == 0 {
				rows = append(rows, a.para(ind, headed(settleHead(t.SettledState), l))...)
			} else {
				rows = append(rows, a.para(ind, plainText(l, ""))...)
			}
		}
	} else {
		rows = append(rows, a.para(ind, plainText(a.b.ThoughtNote(t), dim))...)
	}
	return rows
}

func (a *app) listCur() *board.Thought {
	ts := a.listed()
	for _, t := range ts {
		if t.N == a.lcur {
			return t
		}
	}
	if len(ts) > 0 {
		a.lcur = ts[0].N
		return ts[0]
	}
	return nil
}

// aboutLine is the faint line under a comment: the title of the line it is
// about (← title; (宛先なし) for none, (報告にもうない行) for one gone),
// and the first line of what the AI wrote on it, if it did.
func (a *app) aboutLine(t *board.Thought) text {
	about := "(宛先なし)"
	if s := a.b.About(t); s != "" {
		about = "← " + s
	}
	out := plainText(about, dim)
	if t.SettledState != "" {
		first, _, _ := strings.Cut(strings.TrimSpace(t.Settled), "\n")
		out = append(out, plainText("   "+settleHead(t.SettledState), dim)...)
		out = append(out, plainText(first, "")...)
	}
	return out
}

// listRows is the comments screen: a heading with how many the AI has not
// finished, then each comment in two rows — its state in a column of its
// own, its number and text (two rows at most), then faintly the line it is
// about and the first line of what the AI wrote. Those finished come under
// a rule. o opens a comment: its whole text and what the AI wrote in full.
func (a *app) listRows() []row {
	head := plainText("コメント", bold+cStructure)
	if n := a.openCount(); n > 0 {
		head = append(head, plainText(fmt.Sprintf(" %d", n), bold)...)
	}
	rows := []row{{}, {line: a.compose(false, 0, nil, head, nil)}}
	ts := a.listed()
	if len(ts) == 0 {
		return append(rows, a.para(0, plainText("コメントはまだありません。m で書けます", dim))...)
	}
	cur := a.listCur()
	// The columns: the state words, then the numbers, each as wide as the
	// widest.
	sw, nw := 0, 0
	for _, t := range ts {
		w, _ := a.statusOf(t)
		sw = max(sw, width(w))
		nw = max(nw, len(fmt.Sprint(t.N)))
	}
	a.lends = map[int]int{}
	ruled := false
	for _, t := range ts {
		if t.Finished() && !ruled {
			ruled = true
			rows = append(rows, row{line: a.compose(false, 0, plainText("─ 済んだもの ─", dim), nil, nil)})
		}
		word, color := a.statusOf(t)
		prefix := plainText(word, color)
		prefix = append(prefix, plainText(strings.Repeat(" ", sw-width(word)+2), "")...)
		prefix = append(prefix, plainText(fmt.Sprintf("%*d  ", nw, t.N), dim)...)
		extra := max(prefix.width()-markW, 0)
		base := ""
		if t.Finished() {
			base = dim
		}
		body := withImages(strings.Join(strings.Fields(t.Text), " "), base).capped(4 * a.textWidth())
		rows = append(rows, a.titled(t == cur, 0, extra, prefix, body, nil, commentBodyRows)...)
		if a.lopen[t.N] {
			rows = append(rows, a.thoughtDetails(t, extra)...)
		} else {
			rows = append(rows, row{line: a.compose(false, extra, nil, a.aboutLine(t).trunc(a.room(extra, nil)), nil), cont: true})
		}
		a.lends[t.N] = len(rows)
		if a.lopen[t.N] {
			rows = append(rows, row{})
		}
	}
	return rows
}

// goToComment goes from the comments screen to the line comment t is on:
// back to the report, the Folds around the line open, the cursor on it.
// A comment on no line, on a line gone from the report, or on one hidden
// (while hidden lines are not shown) stays, and says why.
func (a *app) goToComment(t *board.Thought) {
	pv := a.top()
	if t.ID == "" {
		a.msg = page.MsgGoNoLine
		return
	}
	f, ok := a.b.Follow(t.ID)
	switch {
	case !ok:
		a.msg = page.MsgGoGone
		return
	case f.Hidden && !pv.showHidden:
		a.msg = page.MsgLinkHidden
		return
	}
	a.followLink(pv, page.Link{Target: t.ID})
	a.view = viewReport
}

// canGo says whether enter takes comment t to its line.
func (a *app) canGo(t *board.Thought) bool {
	if t == nil || t.ID == "" {
		return false
	}
	f, ok := a.b.Follow(t.ID)
	return ok && (!f.Hidden || a.top().showHidden)
}
