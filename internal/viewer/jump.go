package viewer

import (
	"fmt"
	"strconv"
	"time"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// g goes to a place in the report: each notice, each Section, each
// top-level Fold gets a letter (a to z but g, from the top of the report,
// on screen or not). The letter is drawn at the head of its row (the
// cursor's column) when it is on screen, and the hints line lists them.
// The letters are given when g is pressed and stay until it is left: a
// rewrite of the report meanwhile leaves it (msgMoved), so a letter never
// comes to mean another place.

// jumpTargets is where g can go, in the order of the report, 25 at most.
func (a *app) jumpTargets(pv *view) []*report.Node {
	if pv == nil || a.st == nil || a.b.NoSession || pv.Missing {
		return nil
	}
	return a.b.Jumps(pv.showHidden)
}

// jumpLetter is the letter of the i-th place: a to z, skipping g (g in
// goto mode always leaves it).
func jumpLetter(i int) string { return page.JumpLetter(i) }

// jumpIndex is which place letter r goes to (-1: none).
func jumpIndex(r rune, n int) int { return page.JumpIndex(r, n) }

// startJump enters goto mode with the letters given now, the digits of
// the cursor line's row links, and - when a jump can be gone back.
func (a *app) startJump(pv *view) {
	ts := a.jumpTargets(pv)
	var links []page.Link
	if pv != nil && a.st != nil && !a.b.NoSession {
		for _, l := range a.b.LinksOf(pv.nodeAt(pv.cur)) {
			if l.Num > 0 {
				links = append(links, l)
			}
		}
	}
	if len(ts) > 0 || len(links) > 0 || (pv != nil && len(pv.back) > 0) {
		a.jumping, a.jumpTo, a.jumpLinks = true, ts, links
	}
}

// stopJump leaves goto mode.
func (a *app) stopJump() { a.jumping, a.jumpTo, a.jumpLinks = false, nil, nil }

// remember pushes where pv is now, for - to come back to.
func (pv *view) remember() {
	pv.back = append(pv.back, backPos{cur: pv.cur, top: pv.top})
	if len(pv.back) > maxBack {
		pv.back = pv.back[len(pv.back)-maxBack:]
	}
}

// followLink goes to the row l points at: the Folds around it open, the
// cursor on it, scrolled into view. The viewer moved the cursor, so the
// line waits settleDelay before options and x.
func (a *app) followLink(pv *view, l page.Link) {
	f, ok := a.b.Follow(l.Target)
	switch {
	case !ok:
		a.msg = msgMoved
		return
	case f.Hidden && !pv.showHidden:
		a.msg = page.MsgLinkHidden
		return
	}
	pv.remember()
	for _, k := range f.Open {
		pv.open[k] = true
	}
	pv.cur, pv.lock, pv.placed = f.Key, false, false
	pv.auto, pv.autoAt = f.Key, time.Time{}
	pv.follow = true
}

// goBack returns to where the last jump came from (only the cursor and
// the scroll: nothing is opened or closed).
func (a *app) goBack(pv *view) {
	if len(pv.back) == 0 {
		return
	}
	p := pv.back[len(pv.back)-1]
	pv.back = pv.back[:len(pv.back)-1]
	if pv.nodeAt(p.cur) == nil {
		return
	}
	pv.cur, pv.top, pv.lock, pv.placed = p.cur, p.top, false, false
	pv.auto, pv.autoAt = p.cur, time.Time{}
}

// markJumps puts the letters at the head of the rows they go to.
func (a *app) markJumps(rows []row, ts []*report.Node) {
	for i := range rows {
		if rows[i].jump == nil || rows[i].line == "" {
			continue
		}
		for j, n := range ts {
			if n == rows[i].jump {
				rows[i].line = plainText(jumpLetter(j), cStructure+bold).String() + rows[i].line[1:]
			}
		}
	}
}

// jumpTitle is what the hints call a place.
func jumpTitle(n *report.Node) string { return report.PlainOf(n.Text) }

// minJumpTitle is how short the titles in the list of places may be cut
// before places are left out of it.
const minJumpTitle = 6

// jumpLines replaces the hints while g waits: "esc やめる" first, then
// every place with its letter, wrapped on at most a third of the screen
// (three lines at most, one at least). When they do not fit, the titles are
// cut evenly (none shorter than minJumpTitle), then the places at the end
// are left out for "… +N" (their letters still work).
func (a *app) jumpLines(ts []*report.Node) []string {
	room := a.room(0, nil)
	most := max(min(3, a.t.h/3), 1)
	titles := make([]text, len(ts))
	long := 0
	for i, n := range ts {
		titles[i] = plainText(jumpTitle(n), "").capped(titleCap)
		long = max(long, titles[i].width())
	}
	// lay wraps the first k places, titles cut to cut cells (0: whole).
	// The cursor line's links (digits) and - back come after the places,
	// and are never left out.
	var tail []hint
	for _, l := range a.jumpLinks {
		tail = append(tail, hint{k: strconv.Itoa(l.Num), d: a.b.LinkHintWord(l, minJumpTitle*2)[len("→ "):]})
	}
	if pv := a.top(); pv != nil && len(pv.back) > 0 {
		tail = append(tail, hint{k: "-", d: page.WordBack})
	}
	lay := func(k, cut int) [][]hint {
		hs := []hint{{k: "esc", d: "やめる"}}
		for i := range k {
			t := titles[i]
			if cut > 0 && t.width() > cut {
				t = t.trunc(cut)
			}
			hs = append(hs, hint{k: jumpLetter(i), d: t.plain()})
		}
		if k < len(ts) {
			hs = append(hs, hint{k: "…", d: fmt.Sprintf("+%d", len(ts)-k)})
		}
		hs = append(hs, tail...)
		var out [][]hint
		var line []hint
		for _, h := range hs {
			if len(line) > 0 && keyHints(append(line, h)).width() > room {
				out, line = append(out, line), nil
			}
			line = append(line, h)
		}
		return append(out, line)
	}
	draw := func(ls [][]hint) []string {
		out := make([]string, len(ls))
		for i, l := range ls {
			out[i] = a.compose(false, 0, nil, keyHints(l), nil)
		}
		return out
	}
	if ls := lay(len(ts), 0); len(ls) <= most {
		return draw(ls)
	}
	for cut := long - 1; cut >= minJumpTitle; cut-- {
		if ls := lay(len(ts), cut); len(ls) <= most {
			return draw(ls)
		}
	}
	for k := len(ts) - 1; k > 0; k-- {
		if ls := lay(k, minJumpTitle); len(ls) <= most {
			return draw(ls)
		}
	}
	// Too narrow for even one place: esc and how many there are.
	ls := lay(0, minJumpTitle)
	return draw(ls[:min(most, len(ls))])
}

// handleJump takes the key after g: a letter goes to its place, a digit
// follows that link of the cursor's line, - goes back where the last jump
// came from; anything else (g, esc, a key nothing has) leaves without
// doing anything.
func (a *app) handleJump(k key) {
	ts, links := a.jumpTo, a.jumpLinks
	a.stopJump()
	if k.name != "" {
		return
	}
	pv := a.top()
	if k.r == '-' {
		a.goBack(pv)
		return
	}
	if i := page.LinkIndex(k.r, links); i >= 0 {
		a.followLink(pv, links[i])
		return
	}
	if i := jumpIndex(k.r, len(ts)); i >= 0 {
		pv.remember()
		a.goTo(pv, ts[i])
	}
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
	return k.r != 0 && page.ActKey(pv.nodeAt(pv.cur), string(k.r))
}
