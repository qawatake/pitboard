package viewer

import (
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/report"
)

// g goes to a place in the report: each notice, each Section, each
// top-level Fold gets a letter (a to z, from the top of the report, on
// screen or not). The letter is drawn at the head of its row (the cursor's
// column) when it is on screen, and the hints line lists them all.

// jumpTargets is where g can go, in the order of the report, 26 at most.
func (a *app) jumpTargets(pv *view) []*report.Node {
	if pv == nil || a.st == nil || a.b.NoSession || pv.Missing {
		return nil
	}
	return a.b.Jumps(pv.showHidden)
}

// jumpLetter is the letter of the i-th place.
func jumpLetter(i int) string { return string(rune('a' + i)) }

// markJumps puts the letters at the head of the rows they go to.
func (a *app) markJumps(rows []row, ts []*report.Node) {
	for i := range rows {
		if rows[i].jump == nil || rows[i].line == "" {
			continue
		}
		for j, n := range ts {
			if n == rows[i].jump {
				rows[i].line = plainText(jumpLetter(j), yellow+bold).String() + rows[i].line[1:]
			}
		}
	}
}

// jumpTitle is what the hints call a place.
func jumpTitle(n *report.Node) string { return report.PlainOf(n.Text) }

// jumpLines replaces the hints while g waits: every place with its letter,
// then "esc やめる"; wrapped, and the titles cut shorter until it fits in
// three lines.
func (a *app) jumpLines(ts []*report.Node) []string {
	room := a.room(0, nil)
	lay := func(cut int) []string {
		var hs []hint
		for i, n := range ts {
			t := jumpTitle(n)
			if cut > 0 {
				t = plainText(t, "").trunc(cut).plain()
			}
			hs = append(hs, hint{k: jumpLetter(i), d: t})
		}
		hs = append(hs, hint{k: "esc", d: "やめる"})
		var out []string
		var line []hint
		for _, h := range hs {
			if len(line) > 0 && keyHints(append(line, h)).width() > room {
				out = append(out, a.compose(false, 0, nil, keyHints(line), nil))
				line = nil
			}
			line = append(line, h)
		}
		return append(out, a.compose(false, 0, nil, keyHints(line), nil))
	}
	var out []string
	for _, cut := range []int{0, 16, 10, 6, 3} {
		if out = lay(cut); len(out) <= 3 {
			break
		}
	}
	return out
}

// handleJump takes the key after g: a letter goes there; anything else
// leaves without doing anything.
func (a *app) handleJump(k key) {
	a.jumping = false
	pv := a.top()
	ts := a.jumpTargets(pv)
	if k.name != "" || k.r < 'a' || int(k.r-'a') >= len(ts) {
		return
	}
	a.goTo(pv, ts[k.r-'a'])
}

// goTo puts the cursor on the first stop of n (a notice or Fold: n
// itself) and scrolls its heading to the top of the page. A Section with
// no stop only scrolls. The viewer moved the cursor, so the line waits
// settleDelay before reactions.
func (a *app) goTo(pv *view, n *report.Node) {
	key := ""
	switch n.Kind {
	case report.KindNotice, report.KindDetails:
		key = n.Key
	default:
		in := map[string]bool{}
		report.WalkAll(n.Kids, func(c *report.Node) { in[c.Key] = true })
		for _, k := range pv.navKeys() {
			if in[k] {
				key = k
				break
			}
		}
	}
	if key != "" {
		pv.cur, pv.lock, pv.placed = key, false, false
		pv.auto, pv.autoAt = key, time.Time{}
	}
	pv.follow = n.Kind == report.KindNotice
	if n.Kind == report.KindNotice {
		return
	}
	for i, r := range a.rows(pv) {
		if r.jump == n {
			pv.top = i
			break
		}
	}
}

// ---- reading what is taller than the screen ----

// stopRows is where each stop's first row is among the rows of the page.
func (a *app) stopRows(pv *view) map[string]int {
	out := map[string]int{}
	for i, r := range a.rows(pv) {
		if _, ok := out[r.key]; r.key != "" && !ok {
			out[r.key] = i
		}
	}
	return out
}

// scrollToward moves the page half a screen toward the next stop (d = 1)
// or the one before (d = -1) when that stop is off the screen with rows
// between it and the screen not seen yet, at most until the stop comes on
// screen. The cursor does not move (it may go off the screen). It reports
// whether it scrolled.
func (a *app) scrollToward(pv *view, d int) bool {
	h := max(pv.viewH, 1)
	half := max(h/2, 1)
	pos := a.stopRows(pv)
	keys := pv.navKeys()
	i := index(keys, pv.cur)
	if d > 0 {
		if i < 0 || i+1 >= len(keys) {
			return false
		}
		next, ok := pos[keys[i+1]]
		if !ok || next <= pv.top+h {
			return false
		}
		pv.top = min(pv.top+half, next-h+1)
		return true
	}
	if i <= 0 {
		return false
	}
	prev, ok := pos[keys[i-1]]
	if !ok || prev+1 >= pv.top {
		return false
	}
	pv.top = max(pv.top-half, prev)
	return true
}

// reactKey says whether k reacts to the cursor's line (y n x r c, an
// option's key): while that line is off the screen, it only brings it
// back.
func reactKey(pv *view, k key) bool {
	if k.r == 0 {
		return false
	}
	if strings.ContainsRune("ynxrc", k.r) {
		return true
	}
	n := pv.nodeAt(pv.cur)
	return n != nil && n.Kind == report.KindQuestion && n.Choice(string(k.r)) != nil
}
