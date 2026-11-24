package page

import (
	"errors"
	"os"
	"testing"
	"time"
)

// A rewrite with problems (Fixing) is not shown, so it is not the AI's
// answer: the turn stays the AI's until a good version comes.
func TestTurnWhileFixing(t *testing.T) {
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
	if b.Page(false).Fixing != MsgFixing {
		t.Fatal("not fixing")
	}
	if w := turn(b); w.Text != WordAITurn {
		t.Errorf("fixing: %+v", w)
	}
	write(t, b.Store, src)
	rewrite(t, b, time.Now().Add(2*time.Second))
	b.Changed()
	if w := turn(b); w.Text != WordYourTurn {
		t.Errorf("fixed: %+v", w)
	}
}

// The turn is on the page whatever the report is: missing or empty too.
func TestTurnWithoutReport(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		do   func()
		msg  string
		turn string
	}{
		{"empty", func() { write(t, b.Store, ""); rewrite(t, b, time.Now().Add(time.Second)) }, MsgEmpty, WordYourTurn},
		{"missing", func() { os.Remove(b.Store.ReportPath()) }, MsgNoReport, WordAITurn},
	} {
		c.do()
		b.Changed()
		p := b.Page(false)
		if p.Message != c.msg || len(p.Summary) == 0 || p.Summary[0].Text != c.turn {
			t.Errorf("%s: message %q summary %+v", c.name, p.Message, p.Summary)
		}
	}
}

// x on a line already hidden (hidden lines not shown) is refused in words
// of its own: the report did not change.
func TestHideAlreadyHidden(t *testing.T) {
	b := open(t, src)
	hook := find(b.Page(false).Things, "hook")
	if _, _, err := b.Hide(hook.Seen, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Hide(hook.Seen, false); !errors.Is(err, ErrAlreadyHidden) || err.Error() != MsgAlreadyHidden {
		t.Errorf("x again on a hidden line: %v", err)
	}
}
