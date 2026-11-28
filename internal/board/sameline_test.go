package board

import (
	"os"
	"strings"
	"testing"
)

// Two lines saying the same in the same row each take their own comments:
// the second is named as such, and is gone when it is gone.
func TestThoughtOnASecondSameLine(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Item id=\"a\">項目\n  - 同じ\n  - 同じ\n</Item>\n"), 0o644)
	st.Append(Event{Op: "thought", ID: "Line#2|a\n同じ", Title: "同じ", Text: "2 つ目に"})
	if got, want := changes(t, st), `thought 1 on line "同じ" (#2 with this text) in a: 2 つ目に`; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Item id=\"a\">項目\n  - 同じ\n</Item>\n"), 0o644)
	st.Append(Event{Op: "thought", ID: "Line#2|a\n同じ", Title: "同じ", Text: "また"})
	if got, want := changes(t, st), `on line "同じ" (#2 with this text) in a (now gone): また`; !strings.HasSuffix(got, want) {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
