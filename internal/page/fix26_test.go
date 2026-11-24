package page

import (
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// Settling every thought the latest send carried gives the turn back,
// without a rewrite; a send of answers alone waits for the rewrite.
func TestTurnSettledSend(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	rewrite(t, b, time.Now().Add(-time.Hour))
	b.Changed()
	settle := func(n int, state string) {
		t.Helper()
		if _, err := b.Store.Append(board.Event{Op: "settle", Thought: n, Text: "x", State: state, By: "ai"}); err != nil {
			t.Fatal(err)
		}
		b.Changed()
	}
	send := func() {
		t.Helper()
		if _, err := b.Send(nil); err != nil {
			t.Fatal(err)
		}
	}

	for _, text := range []string{"一つ目", "二つ目"} {
		if _, err := b.Comment("", "", text); err != nil {
			t.Fatal(err)
		}
	}
	send()
	n1, n2 := b.State.Thoughts[0].N, b.State.Thoughts[1].N
	settle(n1, "done")
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("one of two settled: %+v", w)
	}
	settle(n2, "hold")
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("all settled: %+v", w)
	}

	// A later send carries only its own thoughts: the old ones, settled,
	// do not count.
	if _, err := b.Comment("", "", "三つ目"); err != nil {
		t.Fatal(err)
	}
	send()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("new thought sent: %+v", w)
	}
	settle(b.State.Thoughts[2].N, "done")
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("new thought settled: %+v", w)
	}

	// Editing a settled thought and sending makes it unsettled again.
	if _, err := b.Store.Append(board.Event{Op: "edit", Thought: n1, Text: "一つ目 (直し)"}); err != nil {
		t.Fatal(err)
	}
	b.Changed()
	send()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("edited and sent: %+v", w)
	}
	settle(n1, "done")
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("edit settled: %+v", w)
	}

	// A send with no thought (an answer): only the rewrite ends it.
	if _, err := b.Store.Append(board.Event{Op: "react", ID: "unit", Value: "a"}); err != nil {
		t.Fatal(err)
	}
	b.Changed()
	send()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("answer sent: %+v", w)
	}
	rewrite(t, b, time.Now().Add(time.Second))
	b.Changed()
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("answer, rewritten: %+v", w)
	}
}
