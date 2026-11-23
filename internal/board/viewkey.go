package board

import (
	"fmt"
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// ViewKey is what a line says, for "unread": what hiding goes by
// (report.HideKey), plus what opening it shows (its note and contents, an
// Ask's options) and a Link's target; for a Fold, everything it holds. ""
// for lines that are never unread (text, headings).
func ViewKey(n *report.Node) string {
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
