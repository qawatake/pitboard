package viewer

import (
	"os"
	"slices"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// ---- keys ----

func (a *app) handle(k key) (quit bool) {
	if k.name == "kbd" {
		a.kbd = true // the terminal's answer, not a key the person pressed
		return false
	}
	a.msg, a.msgLines = "", 0
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
	if a.list {
		return a.handleList(k)
	}
	if a.inPane {
		return a.handlePane(k)
	}
	pv := a.top()
	switch {
	case k.r == 'q':
		return true
	case k.r == 'g':
		if len(a.jumpTargets(pv)) > 0 {
			a.jumping = true
		}
	case (k.r == 'j' || k.name == "down") && pv.offScreen() > 0,
		(k.r == 'k' || k.name == "up") && pv.offScreen() < 0:
		// The cursor's line is off the screen in this direction: bring it
		// back first, without moving it. The next j/k moves from there.
		pv.follow = true
	case pv.offScreen() != 0 && reactKey(pv, k):
		// The cursor's line is off the screen: a reaction only brings it
		// back, so nothing goes on a line the person is not looking at.
		pv.follow = true
	case k.r == 'j' || k.name == "down":
		// What is between the cursor's line and the next stop is read
		// first, half a screen at a time; the cursor stays.
		if a.scrollToward(pv, 1) {
			break
		}
		if pv.step(1) {
			pv.lock, pv.placed = false, false
			break
		}
		if pv.atEnd && len(a.paneOrder()) > 0 {
			// Past the end of the page: into the thoughts under it. The
			// report's cursor stays where it is for coming back.
			a.inPane, a.pcur = true, a.paneOrder()[0].N
			break
		}
		pv.lock, pv.placed = false, false
		pv.top += max(pv.viewH/2, 1) // past the last stop: scroll the page
	case k.r == ' ':
		a.jump(pv)
	case k.r == 'A':
		a.viewAll(pv)
	case k.r == 'r':
		a.read(pv)
	case k.r == 'k' || k.name == "up":
		if a.scrollToward(pv, -1) {
			break
		}
		pv.lock, pv.placed = false, false
		if !pv.step(-1) {
			pv.top = max(pv.top-max(pv.viewH/2, 1), 0)
		}
	case k.name == "pgup":
		pv.top = max(pv.top-max(a.t.h/2, 1), 0)
	case k.name == "pgdn":
		if !pv.atEnd {
			pv.top += max(a.t.h/2, 1) // draw keeps it within the page
		}
	case k.name == "tab":
		a.list, a.lfollow = true, true
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "宛先なしのコメント")
		a.in.at = pv.cur
	case k.r == 's':
		a.wake()
	case k.name == "enter" && pv.curShown:
		a.enter(pv)
	case k.r == 'x':
		a.hide(pv)
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
		k.name = "down"
	case k.name == "ctrl+p":
		k.name = "up"
	}
	return k
}

// keysFor lists what the reaction keys do on n, in the order shown. An
// Option whose key the viewer uses itself (a check error) cannot be chosen.
func keysFor(n *report.Node) (yn bool, choices []string, thought bool) {
	switch n.Kind {
	case report.KindTodo, report.KindDoing, report.KindDone, report.KindLink, report.KindNotice:
		yn = true
	case report.KindQuestion:
		for _, c := range n.Kids {
			if c.Kind == report.KindChoice && len(c.Label) == 1 && !strings.Contains(report.ViewerKeys+"?", c.Label) {
				choices = append(choices, c.Label)
			}
		}
	}
	return yn, choices, true
}

func (a *app) react(pv *view, r rune) {
	n := a.target(pv)
	if n == nil {
		return
	}
	yn, choices, thought := keysFor(n)
	if n.Kind == report.KindQuestion && !pv.open[n.Key] && slices.Contains(choices, string(r)) {
		// The options are not on screen: the key only opens the question.
		pv.open[n.Key] = true
		return
	}
	if pv.placed {
		yn, choices = false, nil // the person has not pointed at this line yet
	}
	value := ""
	switch s := string(r); {
	case yn && (s == "y" || s == "n"):
		value = s
	case slices.Contains(choices, s):
		value = s
	case thought && s == "c":
		a.in = newInput(report.Target(n), n.Title(), "「"+plainText(n.Title(), "").trunc(max(a.textWidth()-24, 8)).plain()+"」にコメント")
		return
	default:
		return
	}
	if pv.settling() {
		return // dropped, not kept for later
	}
	// Read the report again: the reaction is recorded only if the line is
	// still what the person is looking at.
	b, err := os.ReadFile(pv.path)
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	fresh, _, perr := report.Parse(string(b))
	now := report.ByID(fresh, report.Target(n))
	if perr != nil || now == nil || now.Sig() != n.Sig() {
		a.reload(pv)
		if !pv.lock {
			a.msg = msgChanged
		}
		return
	}
	e := board.Event{Op: "react", ID: report.Target(n), Value: value, Line: n.Sig(), Title: n.Title()}
	if ch := n.Choice(value); n.Kind == report.KindQuestion && ch != nil {
		e.Option = report.PlainOf(ch.Text)
	}
	rec, err := a.st.Append(e)
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
	} else {
		what := value + " "
		if n.Kind == report.KindQuestion {
			what = "答え " + value + " "
		}
		a.did(rec, what+"を取り消しました: "+report.PlainOf(n.Title()))
	}
	a.markViewed(n)
	a.loadEvents()
}

// hideTarget is the line x acts on: the cursor's line under the same rules
// as reactions (on screen, pointed at by the person, page unchanged).
func (a *app) hideTarget(pv *view) *report.Node {
	if !pv.curShown || pv.lock || pv.placed || !a.canWrite() {
		return nil
	}
	n := pv.nodeAt(pv.cur)
	if n == nil || report.HideKey(n) == "" {
		return nil
	}
	return n
}

// hide hides the cursor's line, or shows it again when it is hidden (and
// hidden lines are being shown). The report is not changed: the record says
// what the line says, and the viewer leaves out lines that say that.
func (a *app) hide(pv *view) {
	n := a.hideTarget(pv)
	if n == nil || pv.settling() {
		return
	}
	// As with y and n: read the report again, and record only if the line
	// is still what the screen shows.
	b, err := os.ReadFile(pv.path)
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	fresh, _, perr := report.Parse(string(b))
	var now *report.Node
	report.WalkAll(fresh, func(x *report.Node) {
		if now == nil && x.Key == n.Key {
			now = x
		}
	})
	if perr != nil || now == nil || now.Sig() != n.Sig() || report.HideKey(now) != report.HideKey(n) {
		a.reload(pv)
		if !pv.lock {
			a.msg = msgChanged
		}
		return
	}
	op, msg := "hide", "隠しました (H で表示)"
	switch {
	case pv.isHidden(n):
		op, msg = "unhide", "表示に戻しました"
	case pv.showHidden:
		msg = "隠しました"
	}
	a.markViewed(n)
	before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
	rec, err := a.st.Append(board.Event{Op: op, Key: report.HideKey(n), Title: n.Title()})
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	undoMsg := "隠したのを取り消しました: "
	if op == "unhide" {
		undoMsg = "表示に戻したのを取り消しました: "
	}
	a.did(rec, undoMsg+report.PlainOf(n.Title()))
	a.loadEvents()
	pv.moveFrom(before, i)
	a.msg = msg
}

func (a *app) enter(pv *view) {
	n := pv.nodeAt(pv.cur)
	switch {
	case n == nil:
	case a.canOpen(pv, n):
		pv.open[pv.cur] = !pv.open[pv.cur]
	}
}

// ---- the thoughts list ----

// listed is the thoughts, newest first.
func (a *app) listed() []*board.Thought {
	ts := a.state.Thoughts
	out := make([]*board.Thought, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		out = append(out, ts[i])
	}
	return out
}

func (a *app) handleList(k key) (quit bool) {
	ts := a.listed()
	i := 0
	for j, t := range ts {
		if t.N == a.lcur {
			i = j
		}
	}
	switch {
	case k.name == "tab" || k.r == 'q' || k.name == "esc":
		a.list = false
	case k.r == 'j' || k.name == "down":
		if i+1 < len(ts) {
			a.lcur, a.lfollow = ts[i+1].N, true
		} else {
			a.ltop++
		}
	case k.r == 'k' || k.name == "up":
		if i > 0 && len(ts) > 0 {
			a.lcur, a.lfollow = ts[i-1].N, true
		} else {
			a.ltop = max(a.ltop-1, 0)
		}
	case k.name == "pgup":
		a.ltop = max(a.ltop-max(a.t.h/2, 1), 0)
	case k.name == "pgdn":
		a.ltop += max(a.t.h/2, 1)
	case k.name == "enter" && len(ts) > 0 && a.lcurShown:
		a.lopen[ts[i].N] = !a.lopen[ts[i].N]
	case k.r == 'x' && len(ts) > 0 && a.lcurShown:
		a.retract(ts, ts[i].N, func(n int) { a.lcur, a.lfollow = n, true })
	case k.r == 'e' && len(ts) > 0 && a.lcurShown:
		a.rewrite(ts[i].N)
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "宛先なしのコメント")
	case k.r == 's':
		a.wake()
	}
	return false
}

// handlePane is the keys while the cursor is in the thoughts pane: j k move
// (k on the first goes back to the report), enter opens a thought. Nothing
// is recorded on a thought; m s tab q work as on the report.
func (a *app) handlePane(k key) (quit bool) {
	ts := a.paneOrder()
	i := -1
	for j, t := range ts {
		if t.N == a.pcur {
			i = j
		}
	}
	if i < 0 {
		if len(ts) == 0 {
			a.inPane = false
			return false
		}
		i, a.pcur = 0, ts[0].N
	}
	switch {
	case k.r == 'q':
		return true
	case k.r == ' ':
		a.inPane, a.ptop = false, 0
		a.jump(a.top())
	case k.r == 'A':
		a.viewAll(a.top())
	case k.r == 'j' || k.name == "down":
		if i+1 < len(ts) {
			a.pcur = ts[i+1].N
		}
	case k.r == 'k' || k.name == "up":
		if i == 0 {
			// Back to the report's last stop: the person pressed k to get
			// there, so it works at once.
			a.inPane, a.ptop = false, 0
			a.top().follow = true
			break
		}
		a.pcur = ts[i-1].N
	case k.name == "enter":
		if a.popen == nil {
			a.popen = map[int]bool{}
		}
		a.popen[a.pcur] = !a.popen[a.pcur]
	case k.r == 'e':
		a.rewrite(a.pcur)
	case k.r == 'x':
		a.retract(ts, a.pcur, func(n int) { a.pcur = n })
		if len(a.paneOrder()) == 0 {
			a.inPane, a.ptop = false, 0
			a.top().follow = true
		}
	case k.name == "tab":
		a.list, a.lfollow = true, true
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "宛先なしのコメント")
	case k.r == 's':
		a.wake()
	}
	return false
}

// retract takes thought n back (x on a thought in the pane or the list).
// The cursor goes to the next thought in order (or the one before), which
// then waits settleDelay before x works on it, so x pressed twice does not
// take back a thought the person has not seen. u undoes it.
func (a *app) retract(order []*board.Thought, n int, move func(n int)) {
	if !a.canWrite() || a.tsettling(n) {
		return
	}
	head := ""
	if t := a.thoughtN(n); t != nil {
		head = textHead(t.Text)
	}
	rec, err := a.st.Append(board.Event{Op: "retract", Thought: n})
	if err != nil {
		a.msg = "取り消せませんでした: " + err.Error()
		return
	}
	a.did(rec, "取り消したコメントを戻しました: "+head)
	a.loadEvents()
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
	a.msg = "取り消しました (u で戻す)"
}

// done is one thing the person did, for u: an event recorded (n), or the
// lines A marked as read (viewed). desc is what u says it took back.
type done struct {
	n      int
	viewed []string
	desc   string
}

// did remembers an event the person caused, for u.
func (a *app) did(e board.Event, desc string) {
	a.hist = append(a.hist, done{n: e.N, desc: desc})
}

// textHead is the start of a comment, on one line, for messages.
func textHead(s string) string { return board.CutTitle(s, 30) }

// undoLast takes back the newest thing the person did since the viewer
// opened (u). The record only grows: an undo is recorded, never a line
// removed. The cursor stays where it is; the message says what was taken
// back.
func (a *app) undoLast() {
	if !a.canWrite() || len(a.hist) == 0 {
		a.msg = "取り消せる操作がありません"
		return
	}
	h := a.hist[len(a.hist)-1]
	pv := a.top()
	before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
	if h.viewed != nil {
		if err := a.st.RemoveViewed(h.viewed); err != nil {
			a.msg = "記録できませんでした: " + err.Error()
			return
		}
		a.loadViewed()
	} else if _, err := a.st.Append(board.Event{Op: "undo", Undo: h.n}); err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	a.hist = a.hist[:len(a.hist)-1]
	a.loadEvents()
	// A line hidden again, or a comment gone: the cursor goes to what is
	// next, as when it is hidden or taken back with x.
	pv.moveFrom(before, i)
	if ts := a.paneOrder(); a.inPane && a.thoughtIn(ts, a.pcur) < 0 {
		if len(ts) == 0 {
			a.inPane, a.ptop = false, 0
			pv.follow = true
		} else {
			a.pcur = ts[0].N
		}
	}
	if ts := a.state.Thoughts; a.list && a.thoughtIn(ts, a.lcur) < 0 && len(ts) > 0 {
		a.lcur = ts[len(ts)-1].N
	}
	a.msg = h.desc
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
func (a *app) canWrite() bool { return a.st != nil && !a.noSession }

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
