package page

import (
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// Stops says whether n is a line the person points at: every line on
// screen. A row (report.Row: a Fold, a part with an id, a plain "- " line
// of the report) takes the keys of its kind; a line of text (report.Line:
// the title, a heading, a paragraph, a code block, a numbered or nested
// list line, a line of an opened body) takes only a comment. An Ask's
// options are its row's.
func Stops(n *report.Node) bool { return report.Row(n) || report.Line(n) }

// CanOpen says whether n opens to show more: a Fold, an Ask (its options),
// or a line with contents, a note or comments on it. A Link never opens,
// nor does a list line or a line of text (what is written on them shows
// at their right end).
func (b *Board) CanOpen(n *report.Node) bool {
	if n.Kind == report.KindLink || n.Kind == report.KindChoice || n.Kind == report.KindPlain || report.Line(n) {
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
		a.Options, a.Comment = KeysFor(n)
	}
	a.Hide = report.HideKey(n) != "" && b.CanWrite()
	a.Read = b.CanWrite() && b.MarkOf(n) == MarkNew
	return a
}

// KeysFor is what n takes, whoever may give it: the keys of its options
// when it is an Ask (an Option whose key a screen uses itself, a check
// error, cannot be chosen; answering an Ask is the one way to answer), and
// comments.
func KeysFor(n *report.Node) (choices []string, comment bool) {
	switch n.Kind {
	case report.KindQuestion:
		for _, c := range n.Kids {
			if c.Kind == report.KindChoice && len(c.Label) == 1 && !strings.Contains(report.ViewerKeys+"?", c.Label) {
				choices = append(choices, c.Label)
			}
		}
	}
	return choices, true
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
