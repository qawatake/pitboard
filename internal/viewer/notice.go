package viewer

import (
	"fmt"

	"github.com/qawatake/pitboard/internal/report"
)

// The AI's notices are pinned above the page, however far it is scrolled.
// They take at most a third of the screen (one row at least): what does not
// fit is summed up on the last row (ほか N 件のお知らせ), and the notice the
// cursor is on is always among those shown. A notice is a title and, like
// an Item, contents that enter opens. An open notice too tall for its third
// is no longer pinned: it heads the page and scrolls with it, so that all
// of it can be read.

// pinning is how the notices are laid out above the page.
type pinning struct {
	rows []row
	// ends is where each pinned notice's rows end among rows.
	ends map[string]int
}

// noticeHead is a notice's title, wrapped, not cut (most > 0 cuts it to
// that many rows, ending in …, for a screen too low for it). Its mark is
// ▸ / ▾ (yellow) when it opens, as an Item's, and ! when it does not.
func (a *app) noticeHead(pv *view, n *report.Node, most int) []row {
	sel := n.Key != "" && n.Key == pv.cur && !a.inPane
	mark, base := plainText("!", yellow+bold), yellow
	switch {
	case pv.isHidden(n):
		mark, base = plainText("⌀", dim), dim
	case a.canOpen(pv, n) && pv.open[n.Key]:
		mark = plainText("▾", yellow)
	case a.canOpen(pv, n):
		mark = plainText("▸", yellow)
	}
	body := inline(n.Text, base).capped(titleCap)
	lns := a.titled(sel, 0, 0, mark, body, a.right(pv, n), most)
	lns[0].key, lns[0].jump = n.Key, n
	return lns
}

// noticeBody is what an open notice shows under its title ("" when it is
// closed).
func (a *app) noticeBody(pv *view, n *report.Node) []row {
	if !pv.open[n.Key] || !a.canOpen(pv, n) {
		return nil
	}
	return a.contents(pv, n, 2)
}

// notices is the notices shown (hidden ones only while H shows them).
func (pv *view) notices() []*report.Node {
	var out []*report.Node
	pv.walk(func(n *report.Node) {
		if n.Kind == report.KindNotice {
			out = append(out, n)
		}
	})
	return out
}

// pin lays out the notices for a screen h rows high, and sets which are
// too tall to pin (a.unpinned). must is a notice that has to be shown (the
// one the cursor is on, or being written about); "" for the cursor's.
func (a *app) pin(pv *view, h int, must string) pinning {
	a.unpinned = map[string]bool{}
	out := pinning{ends: map[string]int{}}
	if a.st == nil || a.b.NoSession || pv.Missing {
		return out
	}
	if must == "" && !a.inPane {
		must = pv.cur
	}
	room := max(h/3, 1)
	type block struct {
		n    *report.Node
		rows []row
	}
	var all []block
	for _, n := range pv.notices() {
		rs := append(a.noticeHead(pv, n, 0), a.noticeBody(pv, n)...)
		if pv.open[n.Key] && len(rs) > room {
			a.unpinned[n.Key] = true
			continue
		}
		all = append(all, block{n, rs})
	}
	total := 0
	for _, b := range all {
		total += len(b.rows)
	}
	keep := make([]bool, len(all))
	if total <= room {
		for i := range keep {
			keep[i] = true
		}
	} else {
		// The one that must be shown first, then the others in order, one
		// row left for the sum of the rest.
		used := 0
		for i, b := range all {
			if b.n.Key == must {
				if len(b.rows) > max(room-1, 1) {
					// Too low a screen for all of it: its head, cut.
					all[i].rows = a.noticeHead(pv, b.n, max(room-1, 1))
				}
				keep[i], used = true, len(all[i].rows)
			}
		}
		for i, b := range all {
			if !keep[i] && used+len(b.rows) <= room-1 {
				keep[i], used = true, used+len(b.rows)
			}
		}
	}
	left := 0
	for i, b := range all {
		if !keep[i] {
			left++
			continue
		}
		out.rows = append(out.rows, b.rows...)
		out.ends[b.n.Key] = len(out.rows)
	}
	if left > 0 {
		// The sum takes the last row of the room.
		if len(out.rows) < room {
			out.rows = append(out.rows, row{line: a.compose(false, 0, plainText("!", yellow), plainText(fmt.Sprintf("ほか %d 件のお知らせ", left), yellow), nil)})
		}
	}
	if len(out.rows) > 0 && len(out.rows) < room {
		out.rows = append(out.rows, row{}) // apart from the page scrolling under it
	}
	return out
}
