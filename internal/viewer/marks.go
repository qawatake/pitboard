package viewer

import (
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// Two marks keep the person from losing track of what wants them on a long
// report, in a column of their own between the cursor's > and the line's
// mark (so moving the cursor never moves or hides them):
//
//	● yellow  waiting: an Ask not answered yet, or an Item / Link with
//	          ask="y" without a y / n on what it says now
//	● green   unread: the AI marked the line look="y", and the person has
//	          not looked at what it says now
//
// "Looked at" is the cursor leaving the line after being on it (on screen),
// or reacting to it, opening it, writing on it, hiding it, or A. Being on
// screen is not enough. What a line says is its viewKey; the keys looked at
// are kept, hashed, in .pitboard/<session>/viewed (board.Store.Viewed) (one per line, never sent
// to the AI; a line "-<hash>" takes one back, for u after A).

// The rules of the marks (which line waits, which is unread, what a Fold's
// mark is) are internal/page's; here is where they are drawn and how the
// cursor goes to them.

// isNew says whether n is unread.
func (a *app) isNew(n *report.Node) bool { return a.b.IsNew(n) }

const (
	markNone    = page.MarkNone
	markNew     = page.MarkNew
	markWaiting = page.MarkWaiting
)

// markOfLine is the mark n gets in the gutter.
func (a *app) markOfLine(pv *view, n *report.Node) page.Mark { return a.b.MarkOf(n) }

// gutterMark puts the mark in its column of a drawn line (the cursor's
// too: the cursor has a column of its own).
func gutterMark(line string, m page.Mark) string {
	if m == markNone || len(line) < lgutter || line[markCol:lgutter] != strings.Repeat(" ", lgutter-markCol) {
		return line
	}
	color := green
	if m == markWaiting {
		color = yellow
	}
	return line[:markCol] + plainText("●", color).String() + line[markCol+1:]
}

// allLines is every line the cursor can stop at once Folds are open, in the
// order of the page, with its mark: what space walks through.
func (a *app) allLines(pv *view) []page.Marked { return a.b.Lines(pv.showHidden) }

// counts is how many lines are waiting, and how many are unread (and not
// waiting), in the whole report.
func (a *app) counts(pv *view) (waiting, fresh int) {
	if pv == nil {
		return 0, 0
	}
	return a.b.Counts(pv.showHidden)
}

// anyMarked says whether space has somewhere to go.
func (a *app) anyMarked(pv *view) bool { return pv != nil && a.b.AnyMarked(pv.showHidden) }

// jump moves the cursor to the nearest marked line below it (waiting,
// wait="you" or unread, all alike), or if none, from the top. Folds on the
// way are opened. The person moved the cursor, but the line still waits
// settleDelay before x y n and options work on it.
func (a *app) jump(pv *view) {
	ls := a.allLines(pv)
	at := -1
	for i, l := range ls {
		if l.Node.Key == pv.cur {
			at = i
		}
	}
	for d := 1; d <= len(ls); d++ {
		l := ls[(at+d+len(ls))%len(ls)]
		if l.Mark == markNone && !l.You {
			continue
		}
		for _, f := range l.Folds {
			pv.open[f.Key] = true
		}
		pv.cur, pv.follow, pv.lock, pv.placed = l.Node.Key, true, false, false
		pv.auto, pv.autoAt = l.Node.Key, time.Time{}
		return
	}
	a.msg = "印のある行はありません"
}

// viewAll marks every line now in the report as looked at (A). u takes it
// back.
func (a *app) viewAll(pv *view) {
	if !a.canWrite() {
		return
	}
	a.msg, _ = a.b.ReadAll("A (すべて読んだことにする)")
}

// readTarget is the line r acts on: the cursor's line, on screen, with a
// green mark (its own, or for a Fold, of what it holds; not a line whose
// mark is yellow), and the page not changed under it.
func (a *app) readTarget(pv *view) *report.Node {
	if !pv.curShown || pv.lock || pv.settling() || !a.canWrite() {
		return nil
	}
	n := pv.nodeAt(pv.cur)
	if n == nil || a.markOfLine(pv, n) != markNew {
		return nil
	}
	return n
}

// read marks the cursor's line as read (r): its green mark goes. u takes it
// back. Only the person says a line is read; moving the cursor, opening or
// showing it does not.
func (a *app) read(pv *view) {
	if n := a.readTarget(pv); n != nil {
		a.b.Read(n.Key, "r (読んだ)")
	}
}
