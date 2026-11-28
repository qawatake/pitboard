package page

import (
	"sort"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Pending is the comments in the order a list of them shows them (the
// terminal's comments screen, the browser's panel): those the AI has not
// finished yet (newest first), then those finished
// (most recently settled first).
func (b *Board) Pending() []*board.Thought {
	var open, done []*board.Thought
	for i := len(b.State.Thoughts) - 1; i >= 0; i-- {
		t := b.State.Thoughts[i]
		if !t.Finished() {
			open = append(open, t)
		} else {
			done = append(done, t)
		}
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].SettledN > done[j].SettledN })
	return append(open, done...)
}

// About is the title of the line comment t was written on: as it is now,
// "(報告にもうない行)" when it is gone, and the title it had when written
// if that differs. "" for a comment on no line.
func (b *Board) About(t *board.Thought) string {
	if t.ID == "" {
		return ""
	}
	about := "(報告にもうない行)"
	if n := report.ByID(b.Report.Nodes, t.ID); n != nil {
		about = n.Title()
	}
	if t.Title != "" && t.Title != about {
		about += " (書いたときは: " + t.Title + ")"
	}
	return about
}

// Jumps is where the person can go straight to: each notice (first, as
// they are shown above the page), each Section, each top-level Fold, in
// the order of the report, 25 at most (a viewer letters them a to z
// but g, which leaves goto mode). Hidden ones only with showHidden,
// and not a Section whose lines are all hidden.
func (b *Board) Jumps(showHidden bool) []*report.Node {
	var out []*report.Node
	for _, notices := range []bool{true, false} {
		for _, n := range b.Report.Nodes {
			if (n.Kind == report.KindNotice) != notices || len(out) == 25 {
				continue
			}
			if !showHidden && (b.IsHidden(n) || (n.Kind == report.KindSection && b.allHidden(n.Kids))) {
				continue
			}
			switch n.Kind {
			case report.KindNotice, report.KindSection, report.KindDetails:
				out = append(out, n)
			}
		}
	}
	return out
}
