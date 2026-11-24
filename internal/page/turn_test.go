package page

import (
	"os"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

func turn(b *Board) Word { return b.Summary(false)[0] }

// rewrite stands for the AI writing report.md at time at.
func rewrite(t *testing.T, b *Board, at time.Time) {
	t.Helper()
	if err := os.Chtimes(b.Store.ReportPath(), at, at); err != nil {
		t.Fatal(err)
	}
}

// Whose turn it is: the AI's from a send until the report is rewritten,
// and while a comment is doing; the person's otherwise. Both flip live
// (Changed).
func TestTurn(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	rewrite(t, b, time.Now().Add(-time.Hour))
	b.Changed()
	if w := turn(b); w != (Word{WordYourTurn, ToneNormal}) {
		t.Fatalf("never sent: %+v", w)
	}
	for _, w := range b.Summary(false) {
		if w.Text == WordAITurn {
			t.Errorf("both turns: %+v", b.Summary(false))
		}
	}

	// 1. A send: the AI's turn, read or not, until the report is rewritten.
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("written, not sent: %+v", w)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	if w := turn(b); w != (Word{WordAITurn, ToneQuiet}) {
		t.Errorf("sent: %+v", w)
	}
	for _, w := range b.Summary(false) {
		if w.Text == "送信済 1" {
			t.Errorf("送信済 is gone: %+v", b.Summary(false))
		}
	}
	b.Store.Changes()
	b.Changed()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("read, not answered: %+v", w)
	}
	rewrite(t, b, time.Now())
	if !b.Changed() || turn(b).Text != WordYourTurn {
		t.Errorf("rewritten: %+v", turn(b))
	}

	// 2. A comment settled as doing: the AI's turn, report rewritten or
	// not, until it is settled otherwise.
	n := b.State.Thoughts[0].N
	settle := func(state string) {
		t.Helper()
		if _, err := b.Store.Append(board.Event{Op: "settle", Thought: n, Text: "x", State: state, By: "ai"}); err != nil {
			t.Fatal(err)
		}
		if !b.Changed() {
			t.Error("settle not seen")
		}
	}
	settle("doing")
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("doing: %+v", w)
	}
	rewrite(t, b, time.Now())
	b.Changed()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("doing, rewritten: %+v", w)
	}

	// Both: a send while doing. Doing ends, the send still waits.
	if _, err := b.Comment("", "", "もう一つ"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	settle("done")
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("done, a send not answered: %+v", w)
	}
	rewrite(t, b, time.Now())
	b.Changed()
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("done and rewritten: %+v", w)
	}
	// Doing again: the report's rewrite does not end it.
	settle("doing")
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("doing again: %+v", w)
	}
	settle("hold")
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("hold: %+v", w)
	}
}
