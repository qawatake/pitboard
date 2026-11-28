package viewer

import (
	"time"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// touch is how the cursor came to its line, for page's guard.
func (pv *view) touch() page.Touch {
	t := page.Touch{Placed: pv.placed, Locked: pv.lock, Open: pv.open[pv.cur], Again: pv.again}
	if pv.auto != "" && pv.auto == pv.cur {
		t.Auto = true
		if !pv.autoAt.IsZero() {
			t.Shown = clock().Sub(pv.autoAt)
		}
	}
	return t
}

// settling says whether the cursor's line was put there by the viewer and
// has not been on screen for settleDelay yet.
func (pv *view) settling() bool { return pv.touch().Settling() }

// shownLine is the cursor's line while it is on screen (keys act only on
// it), or nil.
func (pv *view) shownLine() *report.Node {
	if !pv.curShown {
		return nil
	}
	return pv.nodeAt(pv.cur)
}

// nearest is the key of the stop closest to screen row r.
func (a *app) nearest(pv *view, r int) string {
	best, bd := "", -1
	for i, row := range a.rows(pv) {
		if row.key == "" {
			continue
		}
		if d := abs(i - r); bd < 0 || d < bd {
			best, bd = row.key, d
		}
	}
	return best
}

// keepCursor makes the next frame scroll to the cursor's line if it is in
// view now: when the screen changes size, or the bottom takes more room,
// the line the person is on stays on screen.
func (a *app) keepCursor() {
	if pv := a.top(); pv != nil && pv.curShown {
		pv.follow = true
	}
}

// target is the line keys act on: the cursor's line, only while it is on
// screen and the page has not changed under it.
func (a *app) target(pv *view) *report.Node {
	if !pv.curShown || pv.lock {
		return nil
	}
	n := pv.nodeAt(pv.cur)
	if n == nil || report.Target(n) == "" || !a.canWrite() {
		return nil
	}
	return n
}

// offScreen says where the cursor's line is when it is not on screen: -1
// above, 1 below, 0 on screen (or no cursor).
func (pv *view) offScreen() int {
	if pv.curShown || pv.curRow < 0 || pv.nodeAt(pv.cur) == nil {
		return 0
	}
	if pv.curRow < pv.top {
		return -1
	}
	return 1
}

// moveFrom keeps the cursor on a line still shown after lines were hidden:
// if its line went, it goes to the next line that is still there (or the
// one before), as j would.
func (pv *view) moveFrom(before []string, i int) {
	if pv.nodeAt(pv.cur) != nil || i < 0 {
		return
	}
	now := pv.navKeys()
	for _, k := range before[i+1:] {
		if index(now, k) >= 0 {
			pv.cur, pv.follow, pv.auto, pv.autoAt = k, true, k, time.Time{}
			return
		}
	}
	for j := i - 1; j >= 0; j-- {
		if k := before[j]; index(now, k) >= 0 {
			pv.cur, pv.follow, pv.auto, pv.autoAt = k, true, k, time.Time{}
			return
		}
	}
}

// isHidden says whether the person hid n.
func (pv *view) isHidden(n *report.Node) bool {
	return pv.b.IsHidden(n)
}

// hiddenCount is how many lines of the report are hidden.
func (pv *view) hiddenCount() int { return pv.b.HiddenCount() }

// settleCursor puts the cursor on the first stop when it is on none. The
// person did not choose that line, so it is marked placed.
func (pv *view) settleCursor() {
	if pv.nodeAt(pv.cur) == nil {
		pv.cur = ""
		pv.step(0)
		pv.placed = pv.cur != ""
	}
}

// navigable says whether the cursor stops at n: a Fold, a part with an id,
// or a plain "- " line (which can only be hidden; not a numbered one, and
// never a nested one).
func (pv *view) navigable(n *report.Node) bool { return page.Stops(n) }

// step moves the cursor d stops and reports whether it moved. When it did
// not (no stop in that direction), j/k scroll the page instead.
func (pv *view) step(d int) bool {
	keys := pv.navKeys()
	if len(keys) == 0 {
		pv.cur = ""
		return false
	}
	i := index(keys, pv.cur)
	if i < 0 {
		pv.cur, pv.follow = keys[0], true
		return true
	}
	j := min(max(i+d, 0), len(keys)-1)
	if j == i {
		return false
	}
	pv.cur, pv.follow = keys[j], true
	return true
}

func index(xs []string, x string) int {
	for i, y := range xs {
		if y == x {
			return i
		}
	}
	return -1
}

func (a *app) canOpen(pv *view, n *report.Node) bool { return a.b.CanOpen(n) }

// ---- walking the visible tree ----

// walk visits the nodes that are on screen.
func (pv *view) walk(f func(n *report.Node)) {
	var rec func(ns []*report.Node, notices bool)
	rec = func(ns []*report.Node, notices bool) {
		for _, n := range ns {
			if (n.Kind == report.KindNotice) != notices {
				continue
			}
			if !pv.showHidden && (pv.isHidden(n) || (n.Kind == report.KindSection && pv.allHidden(n.Kids))) {
				continue
			}
			f(n)
			if n.Kind == report.KindSection || ((n.Kind == report.KindDetails || n.Kind == report.KindQuestion) && pv.open[n.Key]) {
				rec(n.Kids, false)
			}
		}
	}
	// Notices first: they are drawn above the page, so the cursor meets
	// them first (and starts on them).
	rec(pv.Nodes, true)
	rec(pv.Nodes, false)
}

// allHidden says whether every one of ns is hidden (a Section whose lines
// are all hidden is not drawn at all).
func (pv *view) allHidden(ns []*report.Node) bool {
	for _, n := range ns {
		if !pv.isHidden(n) {
			return false
		}
	}
	return len(ns) > 0
}

func (pv *view) navKeys() []string {
	var keys []string
	pv.walk(func(n *report.Node) {
		if pv.navigable(n) && n.Key != "" {
			keys = append(keys, n.Key)
		}
	})
	return keys
}

func (pv *view) nodeAt(key string) *report.Node {
	if key == "" {
		return nil
	}
	var found *report.Node
	pv.walk(func(n *report.Node) {
		if n.Key == key && pv.navigable(n) {
			found = n
		}
	})
	return found
}
