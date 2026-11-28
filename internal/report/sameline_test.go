package report

import (
	"strings"
	"testing"
)

// Two lines with the same text in the same row have targets of their own:
// the first keeps the plain form, the next ones say which they are.
func TestSameLinesTargets(t *testing.T) {
	nodes, _, err := Parse("<Item id=\"a\">項目\n  - 同じ\n  - 同じ\n  別\n</Item>\n\n- 同じ\n")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	WalkDeep(nodes, func(n *Node) {
		if tg := Target(n); tg != "" {
			got = append(got, tg)
		}
	})
	want := []string{"a", "Line|a\n同じ", "Line#2|a\n同じ", "Line|a\n別", "Line|\n同じ"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q want %q", got, want)
	}
	for _, w := range want {
		if n := ByID(nodes, w); n == nil || Target(n) != w {
			t.Errorf("ByID(%q) = %v", w, n)
		}
	}
}
