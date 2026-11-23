package page

import (
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

// 未送信 N counts inputs as the person sees them: one per line however
// often it was pressed, one per comment however often it was changed.
func TestPendingCount(t *testing.T) {
	evs := []board.Event{
		{N: 1, Op: "react", ID: "a", Value: "y"},
		{N: 2, Op: "react", ID: "a", Value: "y"},
		{N: 3, Op: "react", ID: "a", Value: "n"},
		{N: 4, Op: "hide", Key: "k"},
		{N: 5, Op: "thought", Text: "x"},
		{N: 6, Op: "edit", Thought: 5, Text: "y"},
		{N: 7, Op: "thought", ID: "a", Text: "z"},
		{N: 8, Op: "undo", ID: "b", Undo: 1},
	}
	if got := PendingCount(evs); got != 5 {
		t.Errorf("PendingCount = %d, want 5 (a, hide k, comment 5, comment 7, b)", got)
	}
}

func TestDiffColor(t *testing.T) {
	for l, want := range map[string]string{"@@ -1 +1 @@": ColorQuiet, "+x": ColorMove, "-x": ColorBad, " x": ""} {
		if got := DiffColor(l); got != want {
			t.Errorf("DiffColor(%q) = %q, want %q", l, got, want)
		}
	}
}

// Each refusal is one of the messages that say why a key did nothing, and
// an enum value the browser's gallery covers.
func TestRefusalsEnum(t *testing.T) {
	if len(Enums()["refusal"]) != len(Refusals) {
		t.Errorf("Enums refusal %v", Enums()["refusal"])
	}
}
