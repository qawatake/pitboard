package board

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// The AI says it is on a comment (settle --doing) or has put it aside
// (--hold), and may forget to say when it is done. So what is still doing
// or hold is shown at the end of what 'changes', 'wait' and 'settle' print,
// every time, without relying on the AI to remember.

// openMax is how many open comments are listed (the oldest).
const openMax = 10

// now is the time (tests replace it).
var now = time.Now

// OpenLines is the comments whose state is still doing or hold, oldest
// first, under a line saying what to do with them; nil when there are none.
// A comment taken back, or rewritten (its state is then gone), is not one.
func (s *Store) OpenLines() []string {
	evs := s.Events()
	st := Replay(evs, s.Seen())
	at := map[int]string{}
	for _, e := range evs {
		at[e.N] = e.T
	}
	var open []*Thought
	for _, t := range st.Thoughts {
		if t.SettledState == "doing" || t.SettledState == "hold" {
			open = append(open, t)
		}
	}
	if len(open) == 0 {
		return nil
	}
	// Oldest first: by when it got its state.
	since := func(t *Thought) time.Time {
		tt, _ := time.Parse(time.RFC3339, at[t.SettledN])
		return tt
	}
	sort.SliceStable(open, func(i, j int) bool {
		if a, b := since(open[i]), since(open[j]); !a.Equal(b) {
			return a.Before(b)
		}
		return open[i].SettledN < open[j].SettledN
	})
	out := []string{`open (settle --done N "what happened" when finished):`}
	for i, t := range open {
		if i == openMax {
			out = append(out, fmt.Sprintf("  … and %d more", len(open)-openMax))
			break
		}
		when := ""
		if tt, err := time.Parse(time.RFC3339, at[t.SettledN]); err == nil {
			when = " (" + ago(now().Sub(tt)) + ")"
		}
		out = append(out, fmt.Sprintf("  %-5s %d%s: %s", t.SettledState, t.N, when, CutTitle(strings.TrimSpace(t.Text), 30)))
	}
	return out
}

// ago is a duration in one unit: 40s, 12m, 3h, 2d.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", max(int(d/time.Second), 0))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
}

// NoteOpen prints OpenLines, if there are any.
func (s *Store) NoteOpen(w io.Writer) {
	for _, l := range s.OpenLines() {
		fmt.Fprintln(w, l)
	}
}
