package viewer

import (
	"errors"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// ---- keys ----

func (a *app) handle(k key) (quit bool) {
	if k.name == "kbd" {
		a.kbd = true // the terminal's answer, not a key the person pressed
		return false
	}
	if page.MsgClears(page.OnKey) {
		a.msg, a.msgLines = "", 0
	}
	if k.name == "ctrl+c" {
		return true
	}
	if a.in == nil {
		k = imeFree(k)
	}
	if a.help {
		a.handleHelp(k)
		return false
	}
	if a.in != nil {
		a.handleInput(k)
		if a.in == nil {
			imeOff() // placed, given up, or back from $EDITOR: typing keys again
		}
		return false
	}
	if a.jumping {
		a.handleJump(k)
		return false
	}
	if k.r == '?' {
		a.help = true
		return false
	}
	if k.r == 'u' {
		a.undoLast()
		return false
	}
	if a.onComments() {
		return a.handleList(k)
	}
	pv := a.top()
	switch {
	case k.r == 'q':
		return true
	case k.r == 'g':
		a.startJump(pv)
	case k.r == 'k' && pv.offScreen() < 0 && a.inCurBlock(pv):
		// Reading down through the cursor's line's contents went past its
		// title: k reads back up through them, half a screen at a time, as
		// j read down, before it moves to the line before.
		pv.top = max(pv.top-max(pv.viewH/2, 1), pv.curRow)
		pv.placed = false
	case k.r == 'j' && pv.offScreen() > 0, k.r == 'k' && pv.offScreen() < 0:
		// The cursor's line is off the screen in this direction: bring it
		// back first, without moving it. The next j/k moves from there.
		pv.follow = true
	case pv.offScreen() != 0 && reactKey(pv, k):
		// The cursor's line is off the screen: a reaction only brings it
		// back, so nothing goes on a line the person is not looking at.
		pv.follow = true
		a.msg = msgBack
	case pv.lock && pv.curShown && reactKey(pv, k):
		// The line under the cursor was rewritten: no key acts on it until
		// j/k, and every one says so (page's guard).
		a.react(pv, k.r)
	case scrollKey(k, a.t.h) != 0:
		// The view moves; the cursor stays (it may go off the screen).
		a.scrollView(pv, scrollKey(k, a.t.h))
	case k.r == 'j':
		// What is between the cursor's line and the next stop is read
		// first, half a screen at a time; the cursor stays. Reading on is
		// the person's own move: the cursor is no longer only placed.
		if a.scrollToward(pv, 1) {
			pv.placed = false
			break
		}
		if pv.step(1) {
			pv.lock, pv.placed = false, false
			break
		}
		pv.lock, pv.placed = false, false
		pv.top += max(pv.viewH/2, 1) // past the last stop: scroll the page
	case k.r == ' ':
		a.jump(pv)
	case k.r == 'A':
		a.viewAll(pv)
	case k.r == 'r':
		a.react(pv, k.r)
	case k.r == 'k':
		if a.scrollToward(pv, -1) {
			pv.placed = false
			break
		}
		pv.lock, pv.placed = false, false
		if !pv.step(-1) {
			pv.top = max(pv.top-max(pv.viewH/2, 1), 0)
		}
	case k.r == 'l' || k.name == "tab":
		a.nextView()
	case k.r == 'm' && a.canWrite():
		a.openInput(newInput("", "", "宛先なしのコメント"))
		a.in.at = pv.cur
	case k.r == 's':
		a.wake()
	case k.name == "enter" && pv.curShown:
		a.enter(pv)
	case k.r == 'x':
		a.react(pv, k.r)
	case k.r == 'H':
		before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
		pv.showHidden = !pv.showHidden
		pv.moveFrom(before, i)
	case k.r != 0:
		a.react(pv, k.r)
	}
	return false
}

// imeFree makes the keys work with a Japanese input method left on, outside
// the editor: full-width letters and symbols (and the full-width space)
// become their ASCII keys, and ctrl+n / ctrl+p are down / up (keys that do
// not go through the input method).
func imeFree(k key) key {
	switch {
	case k.r >= 0xff01 && k.r <= 0xff5e:
		k.r -= 0xfee0
	case k.r == 0x3000:
		k.r = ' '
	case k.name == "ctrl+n":
		k.name, k.r = "", 'j' // the cursor keys (↑ ↓ scroll the view)
	case k.name == "ctrl+p":
		k.name, k.r = "", 'k'
	}
	return k
}

// keysFor lists what keys n takes: its options (an Ask), and comments.
func keysFor(n *report.Node) (choices []string, thought bool) { return page.KeysFor(n) }

// react is a key that acts on the cursor's line (x r c, an option's
// key, or any other letter): page's guard says what it does there.
func (a *app) react(pv *view, r rune) {
	n := pv.shownLine()
	v := a.b.Allow(n, string(r), pv.touch())
	pv.again = v.Again
	switch v.Do {
	case page.DoNothing:
		if v.Msg != "" {
			a.msg = v.Msg
		}
	case page.DoOpen:
		pv.open[n.Key] = true
	case page.DoComment:
		a.openInput(newInput(report.Target(n), n.Title(), "「"+plainText(n.Title(), "").trunc(max(a.textWidth()-24, 8)).plain()+"」にコメント"))
	case page.DoReact:
		if _, err := a.b.React(page.SeenOf(n), v.Value); err != nil {
			a.failed(pv, err)
		}
	case page.DoHide:
		a.hide(pv, n)
	case page.DoRead:
		// Only the person says a line is read; u takes it back. On a line
		// the viewer just moved the cursor to, it says which line it was.
		settling := pv.settling()
		if msg, _ := a.b.Read(n.Key, "r (読んだ)"); settling {
			a.msg = msg
		}
	}
}

// failed says why an action was not recorded. When the line changed under
// the cursor, the page is read again first.
func (a *app) failed(pv *view, err error) {
	if errors.Is(err, page.ErrChanged) {
		a.reload(pv)
		if !pv.lock {
			a.msg = msgChanged
		}
		return
	}
	a.msg = err.Error()
}

// hide hides n, the cursor's line, or shows it again when it is hidden (and
// hidden lines are being shown). The report is not changed: the record says
// what the line says, and the viewer leaves out lines that say that.
func (a *app) hide(pv *view, n *report.Node) {
	before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
	msg, _, err := a.b.Hide(page.SeenOf(n), pv.showHidden)
	if err != nil {
		a.failed(pv, err)
		return
	}
	pv.moveFrom(before, i)
	a.msg = msg
}

func (a *app) enter(pv *view) {
	n := pv.nodeAt(pv.cur)
	switch {
	case n == nil:
	case a.canOpen(pv, n):
		// Only open or close: the comments the AI is done with stay
		// folded into one line (all are on the comments screen).
		pv.open[pv.cur] = !pv.open[pv.cur]
	}
}

// ---- the comments screen ----

// handleList is the keys on the comments screen: j k move, enter opens
// the comment (its whole text and what the AI wrote) and closes it, g goes
// to its line in the report, e x rewrite and take it back, l tab q esc go
// back to the report.
func (a *app) handleList(k key) (quit bool) {
	ts := a.listed()
	i := 0
	for j, t := range ts {
		if t.N == a.lcur {
			i = j
		}
	}
	switch {
	case k.r == 'l' || k.name == "tab":
		a.nextView()
	case k.r == 'q' || k.name == "esc":
		a.view = viewReport
		a.top().follow = true
	case k.r == 'j':
		if i+1 < len(ts) {
			a.lcur, a.lfollow = ts[i+1].N, true
		} else {
			a.ltop++
		}
	case k.r == 'k':
		if i > 0 && len(ts) > 0 {
			a.lcur, a.lfollow = ts[i-1].N, true
		} else {
			a.ltop = max(a.ltop-1, 0)
		}
	case scrollKey(k, a.t.h) != 0:
		a.ltop, a.lfollow = max(a.ltop+scrollKey(k, a.t.h), 0), false
	case k.name == "enter" && len(ts) > 0 && a.lcurShown:
		a.lopen[ts[i].N] = !a.lopen[ts[i].N]
	case k.r == 'g' && len(ts) > 0 && a.lcurShown:
		a.goToComment(ts[i])
	case k.r == 'x' && len(ts) > 0 && a.lcurShown:
		a.retract(ts, ts[i].N, func(n int) { a.lcur, a.lfollow = n, true })
	case k.r == 'e' && len(ts) > 0 && a.lcurShown:
		a.rewrite(ts[i].N)
	case k.r == 'm' && a.canWrite():
		a.openInput(newInput("", "", "宛先なしのコメント"))
	case k.r == 's':
		a.wake()
	}
	return false
}

// retract takes comment n back (x on the comments screen).
// The cursor goes to the next thought in order (or the one before), which
// then waits settleDelay before x works on it, so x pressed twice does not
// take back a thought the person has not seen. u undoes it.
func (a *app) retract(order []*board.Thought, n int, move func(n int)) {
	if !a.canWrite() || a.tsettling(n) {
		return
	}
	msg, err := a.b.Retract(n)
	if err != nil {
		a.msg = err.Error()
		return
	}
	for j, t := range order {
		if t.N != n {
			continue
		}
		next := -1
		switch {
		case j+1 < len(order):
			next = order[j+1].N
		case j > 0:
			next = order[j-1].N
		}
		if next >= 0 {
			move(next)
			a.tauto, a.tautoAt = next, time.Time{}
		}
	}
	a.msg = msg
}

// undoLast takes back the newest thing the person did since the viewer
// opened (u). The record only grows: an undo is recorded, never a line
// removed. The cursor stays where it is; the message says what was taken
// back.
func (a *app) undoLast() {
	pv := a.top()
	before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
	desc, err := a.b.Undo()
	if err != nil {
		a.msg = err.Error()
		return
	}
	// A line hidden again, or a comment gone: the cursor goes to what is
	// next, as when it is hidden or taken back with x.
	pv.moveFrom(before, i)
	if ts := a.listed(); a.onComments() && a.thoughtIn(ts, a.lcur) < 0 && len(ts) > 0 {
		a.lcur = ts[0].N
	}
	a.msg = desc
}

func (a *app) thoughtIn(ts []*board.Thought, n int) int {
	for i, t := range ts {
		if t.N == n {
			return i
		}
	}
	return -1
}

// tsettling says whether the thought under the cursor was put there by the
// viewer (after x) and has not been on screen for settleDelay yet.
func (a *app) tsettling(n int) bool {
	return a.tauto != 0 && a.tauto == n && (a.tautoAt.IsZero() || clock().Sub(a.tautoAt) < settleDelay)
}

// canWrite says whether thoughts can be recorded: the session must exist
// (the viewer never creates one).
func (a *app) canWrite() bool { return a.b.CanWrite() }

// handleHelp scrolls the list of keys; q, esc and ? close it.
func (a *app) handleHelp(k key) {
	switch {
	case k.r == 'q' || k.r == '?' || k.name == "esc":
		a.help, a.htop = false, 0
	case k.r == 'j' || k.name == "down":
		a.htop++ // draw keeps it within the list
	case k.r == 'k' || k.name == "up":
		a.htop = max(a.htop-1, 0)
	}
}

// scrollKey is how many rows a scroll key moves the view (0: k is not
// one): half a screen for ctrl+d / ctrl+u, PageDown / PageUp and
// alt+↓ / alt+↑, one row for ctrl+e / ctrl+y and ↓ / ↑ (what the mouse
// wheel sends, by alternate scroll: see scrollOn). They never move the
// cursor; j k do.
func scrollKey(k key, h int) int {
	half := max(h/2, 1)
	switch k.name {
	case "ctrl+d", "pgdn", "alt+down":
		return half
	case "ctrl+u", "pgup", "alt+up":
		return -half
	case "ctrl+e", "down":
		return 1
	case "ctrl+y", "up":
		return -1
	}
	return 0
}

// scrollView moves what the screen shows of the page by d rows, within
// the page (draw keeps it there); the cursor stays where it is.
func (a *app) scrollView(pv *view, d int) {
	if d > 0 && pv.atEnd {
		return
	}
	pv.top, pv.follow = max(pv.top+d, 0), false
}

// inCurBlock says whether what the screen shows is inside the cursor's
// line's own rows (its contents, read down through with j).
func (a *app) inCurBlock(pv *view) bool {
	end, ok := pv.ends[pv.cur]
	return ok && pv.curRow < pv.top && end > pv.top
}

// taller says whether the page is taller than what the last frame showed
// of it.
func (pv *view) taller() bool {
	return pv != nil && pv.viewH > 0 && pv.rowCount > pv.viewH
}
