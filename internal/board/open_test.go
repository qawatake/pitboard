package board

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOpenLines(t *testing.T) {
	st := testStore(t)
	t0 := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	prev := now
	now = func() time.Time { return t0.Add(3 * time.Hour) }
	t.Cleanup(func() { now = prev })
	at := func(d time.Duration) string { return t0.Add(d).Format(time.RFC3339) }
	if st.OpenLines() != nil {
		t.Error("open lines on an empty board")
	}
	for i := range 4 {
		st.Append(Event{Op: "thought", Text: fmt.Sprintf("コメント %d", i+1)})
	}
	st.Append(Event{Op: "thought", Text: strings.Repeat("長い", 30)})
	st.Append(Event{Op: "settle", Thought: 2, Text: "やっています", State: "doing", T: at(2*time.Hour + 48*time.Minute)})
	st.Append(Event{Op: "settle", Thought: 1, Text: "あとで", State: "hold", T: at(0)})
	st.Append(Event{Op: "settle", Thought: 3, Text: "済んだ", T: at(time.Hour)})
	st.Append(Event{Op: "settle", Thought: 4, Text: "x", State: "doing", T: at(time.Hour)})
	st.Append(Event{Op: "retract", Thought: 4})
	st.Append(Event{Op: "settle", Thought: 5, Text: "x", State: "doing", T: at(time.Hour)})
	got := strings.Join(st.OpenLines(), "\n")
	want := `open (settle --done N "what happened" when finished):
  hold  1 (3h): コメント 1
  doing 5 (2h): ` + strings.Repeat("長い", 14) + "長…" + `
  doing 2 (12m): コメント 2`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// Rewritten: its state is gone, and so is it from the list.
	st.Append(Event{Op: "edit", Thought: 5, Text: "書き直し"})
	if got := strings.Join(st.OpenLines(), "\n"); strings.Contains(got, " 5 ") {
		t.Errorf("rewritten still open:\n%s", got)
	}
	// More than 10: the oldest 10, and how many more.
	for i := range 12 {
		e, _ := st.Append(Event{Op: "thought", Text: "多い"})
		st.Append(Event{Op: "settle", Thought: e.N, Text: "x", State: "doing", T: at(time.Duration(i) * time.Minute)})
	}
	ls := st.OpenLines()
	if len(ls) != 12 || ls[11] != "  … and 4 more" || !strings.Contains(ls[1], "hold  1 ") {
		t.Errorf("%d lines:\n%s", len(ls), strings.Join(ls, "\n"))
	}
}
