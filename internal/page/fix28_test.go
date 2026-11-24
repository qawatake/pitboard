package page

import (
	"os"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// A report that fails check, or is missing, is the AI's turn on every
// board, open before or opened now, whatever else holds: after the AI
// answered, after it settled everything, and before anything was sent.
func TestTurnBrokenOrMissing(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	rewrite(t, b, time.Now().Add(-time.Hour))
	b.Changed()
	check := func(name, want string) {
		t.Helper()
		b.Changed()
		if w := turn(b); w.Text != want {
			t.Errorf("%s, open: %+v", name, w)
		}
		if w := turn(Open(b.Store)); w.Text != want {
			t.Errorf("%s, opened now: %+v", name, w)
		}
	}
	at := time.Now().Add(-time.Hour)
	step := func(src string) {
		t.Helper()
		at = at.Add(time.Second)
		write(t, b.Store, src)
		rewrite(t, b, at)
	}

	step(broken())
	check("broken, never sent", WordAITurn)
	step(src)
	check("good, never sent", WordYourTurn)

	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	check("sent", WordAITurn)
	at = time.Now()
	step(src)
	check("answered", WordYourTurn)
	step(broken())
	check("answered, then broken", WordAITurn)
	step(src)
	check("fixed", WordYourTurn)

	n := b.State.Thoughts[0].N
	if _, err := b.Store.Append(board.Event{Op: "settle", Thought: n, Text: "x", State: "done", By: "ai"}); err != nil {
		t.Fatal(err)
	}
	check("all settled", WordYourTurn)
	step(broken())
	check("all settled, then broken", WordAITurn)

	os.Remove(b.Store.ReportPath())
	check("missing", WordAITurn)
	step("")
	check("empty", WordYourTurn)
}
