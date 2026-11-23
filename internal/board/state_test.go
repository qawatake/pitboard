package board

import "testing"

func TestLatestReactionWins(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y"})
	st.Append(Event{Op: "react", ID: "hook", Value: "n"})
	s := Replay(st.Events(), 0)
	if s.React["hook"].Value != "n" {
		t.Errorf("reaction = %q", s.React["hook"].Value)
	}
	if len(st.Events()) != 2 {
		t.Error("both reactions must stay in the record")
	}
}
