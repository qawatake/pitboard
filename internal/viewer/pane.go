package viewer

import (
	"fmt"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
)

// ---- the thoughts at the bottom of the report ----

// paneOrder is the thoughts the pane shows first: those not 対応済 yet
// (newest first), then those 対応済 (most recently settled first).
func (a *app) paneOrder() []*board.Thought { return a.b.Pending() }

const (
	paneMax     = 3  // thoughts shown in the pane
	paneRows    = 3  // rows a thought takes at most in the pane
	paneMinH    = 20 // below this screen height, only the pane's heading
	minBodyRows = 6  // the report keeps at least this many rows
)

// pane is the thoughts, always in view under the report: a heading with
// counts, and the first few. avail is the height left above the hints.
func (a *app) pane(avail int) []string {
	order := a.paneOrder()
	if len(order) == 0 {
		return nil
	}
	if a.inPane {
		return a.openPane(order)
	}
	n := min(paneMax, len(order))
	if a.t.h < paneMinH {
		n = 0
	}
	// Each thought wraps to paneRows rows at most; its state is never cut.
	var each [][]string
	for _, t := range order[:n] {
		word, color := a.statusOf(t)
		base := ""
		if t.Finished() {
			base = dim
		}
		body := withImages(strings.Join(strings.Fields(t.Text), " "), base).capped(4 * a.textWidth())
		var ls []string
		for _, r := range a.titled(false, 2, 0, nil, body, plainText(word, color), paneRows) {
			ls = append(ls, r.line)
		}
		each = append(each, ls)
	}
	height := func(n int) int {
		h := 3 // the heading and the blank above and below
		for _, ls := range each[:n] {
			h += len(ls)
		}
		return h
	}
	for n > 0 && avail-height(n) < minBodyRows {
		n--
	}
	// Quiet: how many are not sent is said at the top right (未送信), and
	// how to send them in the hints (s 送る).
	head := a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)
	if n == 0 {
		return []string{head}
	}
	out := []string{"", head}
	for _, ls := range each[:n] {
		out = append(out, ls...)
	}
	return append(out, "")
}

// openPane is the pane while the cursor is in it: up to half the screen,
// every thought in the pane's order, scrolled to keep the cursor's in view.
func (a *app) openPane(order []*board.Thought) []string {
	head := a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)
	rows, _ := a.paneAll(order)
	room := max(a.t.h/2-3, 1) // less the heading and the blank above and below
	// Keep the cursor's thought in view, and its opened lines as far as
	// they fit.
	ci, end := 0, len(rows)-1
	for i, r := range rows {
		if r.isCur {
			ci, end = i, len(rows)-1
		} else if r.key != "" && i > ci && end == len(rows)-1 && rows[ci].isCur {
			end = i - 1
		}
	}
	end = min(end, ci+room-1)
	if ci < a.ptop {
		a.ptop = ci
	}
	if end >= a.ptop+room {
		a.ptop = min(end-room+1, ci)
	}
	a.ptop = max(min(a.ptop, len(rows)-room), 0)
	out := []string{"", head}
	for i := a.ptop; i < len(rows) && i < a.ptop+room; i++ {
		out = append(out, rows[i].line)
	}
	return append(out, "")
}

// paneAll is every thought of the pane as drawn while the cursor is in
// it, and where each one's rows end.
func (a *app) paneAll(order []*board.Thought) ([]row, map[int]int) {
	var rows []row
	ends := map[int]int{}
	for _, t := range order {
		word, color := a.statusOf(t)
		base := ""
		if t.Finished() {
			base = dim
		}
		sel := t.N == a.pcur
		mark := plainText("▸", "")
		if a.popen[t.N] {
			mark = plainText("▾", "")
		}
		body := withImages(strings.Join(strings.Fields(t.Text), " "), base).capped(4 * a.textWidth())
		r := a.titled(sel, 0, 0, mark, body, plainText(word, color), paneRows)
		r[0].key = fmt.Sprint(t.N) // the first row of a thought
		rows = append(rows, r...)
		if a.popen[t.N] {
			rows = append(rows, a.thoughtDetails(t, markW)...)
		}
		ends[t.N] = len(rows)
	}
	return rows, ends
}

// headed is a line with a faint head word ("理由: ", "対応: ") and the
// rest in the normal color.
func headed(head, s string) text {
	return append(plainText(head, dim), plainText(s, "")...)
}

// thoughtDetails is what an opened thought shows: the line it was written
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
		rows = append(rows, a.para(ind, headed(settleHead(t.SettledState), t.Settled))...)
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

func (a *app) listRows() []row {
	rows := []row{{}, {line: a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)}}
	ts := a.listed()
	if len(ts) == 0 {
		return append(rows, a.para(0, plainText("コメントはまだありません。m で書けます", dim))...)
	}
	cur := a.listCur()
	a.lends = map[int]int{}
	for _, t := range ts {
		sel := t == cur
		open := a.lopen[t.N]
		word, color := a.statusOf(t)
		first, _, _ := strings.Cut(strings.TrimSpace(t.Text), "\n")
		body := append(plainText(fmt.Sprintf("%d  ", t.N), dim), withImages(first, "")...)
		mark := plainText("▸", "")
		if open {
			mark = plainText("▾", "")
		}
		// The first line of the thought wraps; its state is never cut.
		st := plainText(word, color).capped(2 * a.textWidth())
		rows = append(rows, a.line(sel, 0, mark, body.capped(4*a.textWidth()), st)...)
		if open {
			rows = append(rows, a.thoughtDetails(t, 0)...)
		}
		a.lends[t.N] = len(rows)
		if open {
			rows = append(rows, row{})
		}
	}
	return rows
}
