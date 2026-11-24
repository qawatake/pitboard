package page

import (
	"testing"
	"time"
)

// A version that fails check is never a rewrite since the send: a board
// opened fresh on it says AI の番, as the board open before does.
func TestTurnFreshOnBrokenReport(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	rewrite(t, b, time.Now().Add(-time.Hour))
	b.Changed()
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	write(t, b.Store, broken())
	rewrite(t, b, time.Now().Add(time.Second))
	b.Changed()
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("open before: %+v", w)
	}
	fresh := Open(b.Store)
	if w := turn(fresh); w.Text != WordAITurn {
		t.Errorf("opened fresh: %+v", w)
	}
	write(t, b.Store, src)
	rewrite(t, b, time.Now().Add(2*time.Second))
	if w := turn(Open(b.Store)); w.Text != WordYourTurn {
		t.Errorf("fixed, opened fresh: %+v", w)
	}
}
