package page

import (
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// Row links: the AI may write [text](#id) in a line's title or contents to
// point at another row of the report (an answer that says the work is
// queued links to that work). check refuses a target no row has; one that
// went since is drawn as plain text.
//
// In goto mode (g) the links of the cursor's line are numbered 1 to 9, in
// the order they appear (title, then contents): the digit goes there,
// opening the Folds around it. Every jump g makes (a letter or a digit)
// remembers where it came from; - in goto mode goes back.

// MaxLinks is how many links of a line get a digit.
const MaxLinks = 9

// Link is a row link of a line, and the digit g follows it with.
type Link struct {
	Text   string
	Target string
	Num    int
}

// Known says whether a row with id target is in the report.
func (b *Board) Known(target string) bool {
	return target != "" && report.ByID(b.Report.Nodes, target) != nil
}

// LinksOf is n's row links whose target is in the report, in order (its
// title, then its contents), one per target (a second link to the same
// row is the same link), the first MaxLinks numbered. A link to n itself
// is left out.
func (b *Board) LinksOf(n *report.Node) []Link {
	if n == nil {
		return nil
	}
	var out []Link
	seen := map[string]bool{}
	add := func(text string) {
		for _, l := range report.RowLinks(text) {
			if b.Known(l.Target) && l.Target != n.ID && !seen[l.Target] {
				seen[l.Target] = true
				out = append(out, Link{Text: report.PlainOf(l.Text), Target: l.Target})
			}
		}
	}
	add(n.Text)
	var walk func(ns []*report.Node)
	walk = func(ns []*report.Node) {
		for _, c := range ns {
			add(c.Text)
			walk(c.Subs)
		}
	}
	walk(n.Body)
	for i := range out {
		if i < MaxLinks {
			out[i].Num = i + 1
		}
	}
	return out
}

// LinkNums is, by target, the digit of each of n's links (for drawing it
// before the link while g waits).
func (b *Board) LinkNums(n *report.Node) map[string]int {
	out := map[string]int{}
	for _, l := range b.LinksOf(n) {
		if l.Num > 0 {
			out[l.Target] = l.Num
		}
	}
	return out
}

// LinkIndex is which of links digit r follows (-1: none).
func LinkIndex(r rune, links []Link) int {
	for i, l := range links {
		if l.Num > 0 && r == rune('0'+l.Num) {
			return i
		}
	}
	return -1
}

// Follow is what going to a row takes: the row's key (the cursor's
// line) and the Folds around it to open, outermost first.
type Follow struct {
	Key  string
	Open []string
	// Hidden: the row (or a Fold around it) is hidden by the person; it is
	// on screen only while hidden lines are shown.
	Hidden bool
}

// Follow is how to go to the row with id target (false: no such row).
func (b *Board) Follow(target string) (Follow, bool) {
	var f Follow
	var path []*report.Node
	var find func(ns []*report.Node) bool
	find = func(ns []*report.Node) bool {
		for _, n := range ns {
			if n.ID == target && Stops(n) {
				f.Key = n.Key
				for _, a := range path {
					if a.Kind == report.KindDetails {
						f.Open = append(f.Open, a.Key)
					}
					f.Hidden = f.Hidden || b.IsHidden(a)
				}
				f.Hidden = f.Hidden || b.IsHidden(n)
				return true
			}
			path = append(path, n)
			if find(n.Kids) {
				return true
			}
			path = path[:len(path)-1]
		}
		return false
	}
	if target == "" || !find(b.Report.Nodes) {
		return Follow{}, false
	}
	return f, true
}

// LinkHintMax is how many characters of the target's title the g 1 hint
// keeps.
const LinkHintMax = 16

// Words of row links.
const (
	MsgLinkHidden = "隠した行です。H で表示すると行けます"
	WordBack      = "戻る"
	// Why enter on a comment (the terminal's comments screen) does not go
	// to its line: it is on none, or the line is gone from the report.
	MsgGoNoLine = "宛先なしのコメントです。行はありません"
	MsgGoGone   = "その行は報告にもうありません"
)

// LinkHintWord is the hint for a line with exactly one link: where g 1
// goes (the target's title, cut to max cells' worth of characters).
func (b *Board) LinkHintWord(l Link, max int) string {
	t := l.Text
	if n := report.ByID(b.Report.Nodes, l.Target); n != nil {
		t = report.PlainOf(n.Title())
	}
	r := []rune(strings.Join(strings.Fields(t), " "))
	if max > 1 && len(r) > max {
		r = append(r[:max-1], '…')
	}
	return "→ " + string(r)
}
