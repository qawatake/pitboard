package page

import (
	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Two marks keep the person from losing track of what wants them:
//
//	MarkWaiting  an Ask not answered yet, or an Item / Link with ask="y"
//	             without a y / n on what it says now
//	MarkNew      unread: the AI marked the line look="y", and the person
//	             has not looked at what it says now
//
// Only the person says a line is read: by reacting to it, writing on it,
// hiding it, Read or ReadAll (never by looking or opening). What a line says is
// its ViewKey; the keys looked at are kept, hashed, in the board's viewed
// file (never sent to the AI).

// Mark is the mark a line gets.
type Mark int

const (
	MarkNone Mark = iota
	MarkNew
	MarkWaiting
)

// ViewKey is what a line says, for "unread" (board.ViewKey).
func ViewKey(n *report.Node) string { return board.ViewKey(n) }

// Waiting says whether n wants the person's reaction and has none on what
// it says now.
func (b *Board) Waiting(n *report.Node) bool {
	if n.ID == "" {
		return false
	}
	e, ok := b.State.React[n.ID]
	answered := ok && e.Line == n.Sig()
	switch {
	case n.Kind == report.KindQuestion:
		return !answered
	case n.Ask == "y" && (n.Kind == report.KindTodo || n.Kind == report.KindDoing || n.Kind == report.KindLink):
		return !answered || (e.Value != "y" && e.Value != "n")
	}
	return false
}

// IsNew says whether n is unread: the AI asked for it to be read
// (look="y") and the person has not looked at what it says now.
func (b *Board) IsNew(n *report.Node) bool {
	if n.Look != "y" || b.Store == nil {
		return false
	}
	k := ViewKey(n)
	return k != "" && !b.Viewed[board.HashKey(k)]
}

// MarkViewed records that the person looked at ns, and returns the hashes
// that were not recorded before.
func (b *Board) MarkViewed(ns ...*report.Node) []string {
	if !b.CanWrite() {
		return nil
	}
	var hs []string
	for _, n := range ns {
		if n == nil {
			continue
		}
		k := ViewKey(n)
		if k == "" {
			continue
		}
		h := board.HashKey(k)
		if !b.Viewed[h] {
			b.Viewed[h] = true
			hs = append(hs, h)
		}
	}
	if err := b.Store.AddViewed(hs); err == nil {
		b.viewedMod = ModTime(b.Store.ViewedPath())
	}
	return hs
}

// MarkOf is the mark n gets. A Fold gets the strongest mark of what it
// holds, open or not. Hidden lines get none.
func (b *Board) MarkOf(n *report.Node) Mark {
	if b.IsHidden(n) {
		return MarkNone
	}
	if n.Kind == report.KindDetails {
		m := MarkNone
		if b.IsNew(n) {
			m = MarkNew
		}
		var rec func(ns []*report.Node)
		rec = func(ns []*report.Node) {
			for _, c := range ns {
				if m == MarkWaiting || b.IsHidden(c) {
					continue
				}
				m = max(m, b.MarkOf(c))
				if c.Kind == report.KindSection {
					rec(c.Kids)
				}
			}
		}
		rec(n.Kids)
		return m
	}
	switch {
	case b.Waiting(n):
		return MarkWaiting
	case b.IsNew(n):
		return MarkNew
	}
	return MarkNone
}

// UnreadIn is what makes n's MarkNew: n if unread, and for a Fold, the
// unread lines it holds (not hidden ones, nor ones waiting).
func (b *Board) UnreadIn(n *report.Node) []*report.Node {
	if b.IsHidden(n) {
		return nil
	}
	var out []*report.Node
	if b.IsNew(n) {
		out = append(out, n)
	}
	if n.Kind == report.KindDetails {
		var rec func(ns []*report.Node)
		rec = func(ns []*report.Node) {
			for _, c := range ns {
				if b.IsHidden(c) {
					continue
				}
				if c.Kind != report.KindSection && !b.Waiting(c) && b.IsNew(c) {
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

// Marked is a line the person can be taken to, in the order of the page,
// with its mark and the Folds around it.
type Marked struct {
	Node  *report.Node
	Mark  Mark
	You   bool // an Item with wait="you" (not a mark, but a place to go)
	Folds []*report.Node
}

// Lines is every line that can be marked or pointed at once Folds are
// open, in the order of the page (notices where they are written), with
// its mark: what "next marked line" walks through. Hidden lines are left
// out unless showHidden, and so is what is inside an Ask. A Fold's own
// mark here is only its own unread; what it holds counts on its own.
func (b *Board) Lines(showHidden bool) []Marked {
	var out []Marked
	var rec func(ns []*report.Node, folds []*report.Node)
	rec = func(ns []*report.Node, folds []*report.Node) {
		for _, n := range ns {
			if !showHidden && (b.IsHidden(n) || (n.Kind == report.KindSection && b.allHidden(n.Kids))) {
				continue
			}
			if Stops(n) && n.Key != "" {
				m := MarkNone
				switch {
				case n.Kind != report.KindDetails:
					m = b.MarkOf(n)
				case !b.IsHidden(n) && b.IsNew(n):
					m = MarkNew
				}
				you := n.Wait == "you" && !b.IsHidden(n)
				out = append(out, Marked{Node: n, Mark: m, You: you, Folds: folds})
			}
			switch n.Kind {
			case report.KindSection:
				rec(n.Kids, folds)
			case report.KindDetails:
				rec(n.Kids, append(append([]*report.Node{}, folds...), n))
			}
		}
	}
	rec(b.Report.Nodes, nil)
	return out
}

// allHidden says whether every one of ns is hidden (a Section whose lines
// are all hidden is not shown at all).
func (b *Board) allHidden(ns []*report.Node) bool {
	for _, n := range ns {
		if !b.IsHidden(n) {
			return false
		}
	}
	return len(ns) > 0
}

// Counts is how many lines are waiting, and how many are unread (and not
// waiting), in the whole report (with showHidden, also what hidden Folds
// hold).
func (b *Board) Counts(showHidden bool) (waiting, unread int) {
	if b.Store == nil {
		return 0, 0
	}
	for _, l := range b.Lines(showHidden) {
		switch l.Mark {
		case MarkWaiting:
			waiting++
		case MarkNew:
			unread++
		}
	}
	return
}

// AnyMarked says whether there is a marked line (or one waiting on the
// person) to go to.
func (b *Board) AnyMarked(showHidden bool) bool {
	if b.Store == nil {
		return false
	}
	for _, l := range b.Lines(showHidden) {
		if l.Mark != MarkNone || l.You {
			return true
		}
	}
	return false
}
