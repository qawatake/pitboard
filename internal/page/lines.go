package page

import (
	"strconv"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Stops says whether n is a line the person points at (and so can act
// on): a Fold, a part with an id, or a plain "- " line (which can only be
// hidden; not a numbered one, and never a nested one).
func Stops(n *report.Node) bool {
	return n.Kind == report.KindDetails || n.ID != "" || (n.Kind == report.KindPlain && n.Num == "")
}

// CanOpen says whether n opens to show more: a Fold, an Ask (its options),
// or a line with contents, a note or comments on it. A Link never opens.
func (b *Board) CanOpen(n *report.Node) bool {
	if n.Kind == report.KindLink || n.Kind == report.KindChoice {
		return false
	}
	if n.Kind == report.KindDetails || n.Kind == report.KindQuestion || len(n.Body) > 0 || n.Reason != "" {
		return true
	}
	id := report.Target(n)
	return id != "" && len(b.State.On(id)) > 0
}

// Actions is what the person can do on a line.
type Actions struct {
	YN      bool     // y / n
	Options []string // the keys of an Ask's options that can be chosen
	Comment bool     // write a comment on it
	Hide    bool     // hide it (or show it again when hidden)
	Read    bool     // say it is read (it has a MarkNew)
}

// ActionsOf is what the person can do on n as the board is now. Whether
// the person has pointed at the line, and whether it is on screen, are the
// screen's to add.
func (b *Board) ActionsOf(n *report.Node) Actions {
	var a Actions
	if report.Target(n) != "" && b.CanWrite() {
		a.YN, a.Options, a.Comment = KeysFor(n)
	}
	a.Hide = report.HideKey(n) != "" && b.CanWrite()
	a.Read = b.CanWrite() && b.MarkOf(n) == MarkNew
	return a
}

// KeysFor is what reactions n takes, whoever may give them: y / n, the
// keys of its options (an Option whose key a screen uses itself, a check
// error, cannot be chosen), and comments.
func KeysFor(n *report.Node) (yn bool, choices []string, comment bool) {
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

// Seen is a line as the person saw it. An action carries it, and is
// refused (ErrChanged) when the line now says something else: nothing is
// ever recorded against what the person did not see.
type Seen struct {
	Key     string // which line (report.Node.Key)
	Sig     string // what it says (report.Node.Sig)
	HideKey string // what hiding it goes by (report.HideKey)
}

// SeenOf is n as it is now.
func SeenOf(n *report.Node) Seen {
	return Seen{Key: n.Key, Sig: n.Sig(), HideKey: report.HideKey(n)}
}

// Quote is the thought an Item answers (re="N"), or nil when there is no
// such thought (taken back, a wrong number).
func (b *Board) Quote(n *report.Node) *board.Thought {
	if n.Re == "" || b.State == nil {
		return nil
	}
	for _, t := range b.State.Thoughts {
		if strconv.Itoa(t.N) == n.Re {
			return t
		}
	}
	return nil
}
