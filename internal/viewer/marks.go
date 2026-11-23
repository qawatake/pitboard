package viewer

import (
	"fmt"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/board"
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

// viewKey is what a line says, for "unread": what hiding goes by (report.HideKey),
// plus what opening it shows (its note and contents, an Ask's options) and a
// Link's target; for a Fold, everything it holds. "" for lines that are
// never unread (text, headings).
func viewKey(n *report.Node) string {
	if n.Kind == report.KindDetails {
		var b strings.Builder
		b.WriteString(report.HideKey(n) + "|title=" + n.Text)
		var rec func(ns []*report.Node)
		rec = func(ns []*report.Node) {
			for _, c := range ns {
				b.WriteString(fmt.Sprintf("|%d:%s:%s:%s:%s:%s:%s", c.Kind, c.ID, c.Label, c.Text, c.Status, c.Reason, strings.Join(c.Code, "\n")))
				rec(c.Body)
				rec(c.Kids)
				rec(c.Subs)
			}
		}
		rec(n.Kids)
		return b.String()
	}
	k := report.HideKey(n)
	if k == "" {
		return ""
	}
	if n.Kind == report.KindPlain {
		return k
	}
	var b strings.Builder
	b.WriteString(k)
	b.WriteString("|note=" + n.Reason)
	b.WriteString("|" + n.Path + n.URL)
	var body func(ns []*report.Node)
	body = func(ns []*report.Node) {
		for _, c := range ns {
			b.WriteString("|" + c.Label + ":" + c.Text + strings.Join(c.Code, "\n"))
		}
	}
	body(n.Body)
	if n.Kind == report.KindQuestion {
		body(n.Kids)
	}
	return b.String()
}

// waiting says whether n wants the person's reaction and has none on what
// it says now.
func (a *app) waiting(n *report.Node) bool {
	if n.ID == "" {
		return false
	}
	e, ok := a.state.React[n.ID]
	answered := ok && e.Line == n.Sig()
	switch {
	case n.Kind == report.KindQuestion:
		return !answered
	case n.Ask == "y" && (n.Kind == report.KindTodo || n.Kind == report.KindDoing || n.Kind == report.KindLink):
		return !answered || (e.Value != "y" && e.Value != "n")
	}
	return false
}

// isNew says whether n is unread: the AI asked for it to be read
// (look="y") and the person has not looked at what it says now.
func (a *app) isNew(n *report.Node) bool {
	if n.Look != "y" || a.st == nil {
		return false
	}
	k := viewKey(n)
	return k != "" && !a.viewed[board.HashKey(k)]
}

// markViewed records that the person looked at n, and returns the hashes
// that were not recorded before.
func (a *app) markViewed(ns ...*report.Node) []string {
	if !a.canWrite() {
		return nil
	}
	var hs []string
	for _, n := range ns {
		if n == nil {
			continue
		}
		k := viewKey(n)
		if k == "" {
			continue
		}
		h := board.HashKey(k)
		if !a.viewed[h] {
			a.viewed[h] = true
			hs = append(hs, h)
		}
	}
	if err := a.st.AddViewed(hs); err == nil {
		a.viewedMod = modTime(a.st.ViewedPath())
	}
	return hs
}

const (
	markNone = iota
	markNew
	markWaiting
)

// markOfLine is the mark n gets in the gutter. A Fold gets the strongest
// mark of what it holds, open or not. Hidden lines get none.
func (a *app) markOfLine(pv *view, n *report.Node) int {
	if pv.isHidden(n) {
		return markNone
	}
	if n.Kind == report.KindDetails {
		m := markNone
		if a.isNew(n) {
			m = markNew
		}
		var rec func(ns []*report.Node)
		rec = func(ns []*report.Node) {
			for _, c := range ns {
				if m == markWaiting || pv.isHidden(c) {
					continue
				}
				m = max(m, a.markOfLine(pv, c))
				if c.Kind == report.KindSection {
					rec(c.Kids)
				}
			}
		}
		rec(n.Kids)
		return m
	}
	switch {
	case a.waiting(n):
		return markWaiting
	case a.isNew(n):
		return markNew
	}
	return markNone
}

// gutterMark puts the mark in its column of a drawn line (the cursor's
// too: the cursor has a column of its own).
func gutterMark(line string, m int) string {
	if m == markNone || len(line) < lgutter || line[markCol:lgutter] != strings.Repeat(" ", lgutter-markCol) {
		return line
	}
	color := green
	if m == markWaiting {
		color = yellow
	}
	return line[:markCol] + plainText("●", color).String() + line[markCol+1:]
}

// target is a line marked in the report, in the order of the page, with
// the Folds around it.
type marked struct {
	n     *report.Node
	m     int
	you   bool // an Item with wait="you" (not a mark: space goes there too)
	folds []*report.Node
}

// allLines is every line the cursor can stop at once Folds are open, in the
// order of the page, with its mark: what space walks through. Hidden lines
// are left out (unless H shows them), and so is what is inside an Ask.
func (a *app) allLines(pv *view) []marked {
	var out []marked
	var rec func(ns []*report.Node, folds []*report.Node)
	rec = func(ns []*report.Node, folds []*report.Node) {
		for _, n := range ns {
			if !pv.showHidden && (pv.isHidden(n) || (n.Kind == report.KindSection && pv.allHidden(n.Kids))) {
				continue
			}
			if pv.navigable(n) && n.Key != "" {
				m := markNone
				switch {
				case n.Kind != report.KindDetails:
					m = a.markOfLine(pv, n)
				case !pv.isHidden(n) && a.isNew(n):
					m = markNew // the Fold itself; what it holds counts on its own
				}
				you := n.Wait == "you" && !pv.isHidden(n)
				out = append(out, marked{n: n, m: m, you: you, folds: folds})
			}
			switch n.Kind {
			case report.KindSection:
				rec(n.Kids, folds)
			case report.KindDetails:
				rec(n.Kids, append(append([]*report.Node{}, folds...), n))
			}
		}
	}
	rec(pv.nodes, nil)
	return out
}

// counts is how many lines are waiting, and how many are unread (and not
// waiting), in the whole report.
func (a *app) counts(pv *view) (waiting, fresh int) {
	if pv == nil || a.st == nil {
		return 0, 0
	}
	for _, l := range a.allLines(pv) {
		switch l.m {
		case markWaiting:
			waiting++
		case markNew:
			fresh++
		}
	}
	return
}

// anyMarked says whether space has somewhere to go.
func (a *app) anyMarked(pv *view) bool {
	if pv == nil || a.st == nil {
		return false
	}
	for _, l := range a.allLines(pv) {
		if l.m != markNone || l.you {
			return true
		}
	}
	return false
}

// jump moves the cursor to the nearest marked line below it (waiting,
// wait="you" or unread, all alike), or if none, from the top. Folds on the
// way are opened. The person moved the cursor, but the line still waits
// settleDelay before x y n and options work on it.
func (a *app) jump(pv *view) {
	ls := a.allLines(pv)
	at := -1
	for i, l := range ls {
		if l.n.Key == pv.cur {
			at = i
		}
	}
	for d := 1; d <= len(ls); d++ {
		l := ls[(at+d+len(ls))%len(ls)]
		if l.m == markNone && !l.you {
			continue
		}
		for _, f := range l.folds {
			pv.open[f.Key] = true
		}
		pv.cur, pv.follow, pv.lock, pv.placed = l.n.Key, true, false, false
		pv.auto, pv.autoAt = l.n.Key, time.Time{}
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
	var ns []*report.Node
	report.WalkAll(pv.nodes, func(n *report.Node) { ns = append(ns, n) })
	if hs := a.markViewed(ns...); len(hs) > 0 {
		a.hist = append(a.hist, done{viewed: hs, desc: "A (すべて読んだことにする) を取り消しました"})
	}
	a.msg = "いまある行をすべて読んだことにしました"
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

// unreadIn is what makes n's green mark: n if unread, and for a Fold, the
// unread lines it holds (not hidden ones).
func (a *app) unreadIn(pv *view, n *report.Node) []*report.Node {
	if pv.isHidden(n) {
		return nil
	}
	var out []*report.Node
	if a.isNew(n) {
		out = append(out, n)
	}
	if n.Kind == report.KindDetails {
		var rec func(ns []*report.Node)
		rec = func(ns []*report.Node) {
			for _, c := range ns {
				if pv.isHidden(c) {
					continue
				}
				if c.Kind != report.KindSection && !a.waiting(c) && a.isNew(c) {
					out = append(out, c)
				}
				if c.Kind == report.KindSection || c.Kind == report.KindDetails {
					rec(c.Kids)
				}
			}
		}
		rec(n.Kids)
	}
	return out
}

// read marks the cursor's line as read (r): its green mark goes. u takes it
// back. Only the person says a line is read; moving the cursor, opening or
// showing it does not.
func (a *app) read(pv *view) {
	n := a.readTarget(pv)
	if n == nil {
		return
	}
	if hs := a.markViewed(a.unreadIn(pv, n)...); len(hs) > 0 {
		a.hist = append(a.hist, done{viewed: hs, desc: "r (読んだ) を取り消しました: " + report.PlainOf(n.Title())})
	}
}
