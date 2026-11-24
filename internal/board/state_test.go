package board

import (
	"strings"
	"testing"
)

func TestLatestReactionWins(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "unit", Value: "a", Option: "A"})
	st.Append(Event{Op: "react", ID: "unit", Value: "b", Option: "B"})
	s := Replay(st.Events(), 0)
	if s.React["unit"].Value != "b" {
		t.Errorf("answer = %q", s.React["unit"].Value)
	}
	if len(st.Events()) != 2 {
		t.Error("both answers must stay in the record")
	}
}

// An older viewer's y / n (no option) replays as nothing, and so does its
// undo; an answer with option key y is an answer.
func TestLegacyYNIgnored(t *testing.T) {
	st := testStore(t)
	r, _ := st.Append(Event{Op: "react", ID: "hook", Value: "y"})
	st.Append(Event{Op: "react", ID: "link", Value: "n"})
	st.Append(Event{Op: "undo", Undo: r.N, ID: "hook"})
	st.Append(Event{Op: "react", ID: "q", Value: "y", Option: "入れる"})
	sendAll(st)
	s := Replay(st.Events(), 0)
	if _, ok := s.React["hook"]; ok || len(s.Undoing) != 0 {
		t.Errorf("legacy y replayed: %+v %+v", s.React, s.Undoing)
	}
	if _, ok := s.React["link"]; ok {
		t.Error("legacy n replayed")
	}
	if s.React["q"].Value != "y" {
		t.Errorf("answer y: %+v", s.React)
	}
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "answer q = y (入れる; question: gone)" {
		t.Errorf("changes %q", lines)
	}
}
