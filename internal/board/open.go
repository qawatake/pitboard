package board

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// The AI says it is on a comment (settle --doing) or has put it aside
// (--hold), and may forget to say when it is done. So the numbers of what is
// still doing or hold end what 'changes' and 'wait' print, every time,
// without relying on the AI to remember. One short line, to spare its
// context; 'open' lists them.

// openMax is how many numbers the line gives (the oldest).
const openMax = 20

// now is the time (tests replace it).
var now = time.Now

// openThought is a comment still doing or hold, and since when.
type openThought struct {
	*Thought
	since time.Time
	ok    bool // since is known
}

// openThoughts is the comments whose state is still doing or hold, oldest
// first (by when they got that state). A comment taken back, or rewritten
// (its state is then gone), is not one.
func (s *Store) openThoughts() []openThought {
	evs := s.Events()
	st := Replay(evs, s.Seen())
	at := map[int]string{}
	for _, e := range evs {
		at[e.N] = e.T
	}
	var open []openThought
	for _, t := range st.Thoughts {
		if t.SettledState == "doing" || t.SettledState == "hold" {
			tt, err := time.Parse(time.RFC3339, at[t.SettledN])
			open = append(open, openThought{t, tt, err == nil})
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if a, b := open[i].since, open[j].since; !a.Equal(b) {
			return a.Before(b)
		}
		return open[i].SettledN < open[j].SettledN
	})
	return open
}

// OpenLine is the one line that ends 'changes' and 'wait': the numbers,
// oldest first, how long the oldest has waited, and where the details are.
// Empty when nothing is open.
func (s *Store) OpenLine() string {
	open := s.openThoughts()
	if len(open) == 0 {
		return ""
	}
	var ns []string
	for i, t := range open {
		if i == openMax {
			ns = append(ns, "…")
			break
		}
		ns = append(ns, fmt.Sprint(t.N))
	}
	oldest := ""
	if open[0].ok {
		oldest = " (oldest " + ago(now().Sub(open[0].since)) + ")"
	}
	return fmt.Sprintf("open: %s%s. Details: pitboard -s %s open", strings.Join(ns, " "), oldest, ShortSession(s))
}

// OpenLines is the list 'open' prints: one comment a line, oldest first,
// with its state, how long it has had it and the head of its text. Nil when
// nothing is open.
func (s *Store) OpenLines() []string {
	var out []string
	for _, t := range s.openThoughts() {
		when := ""
		if t.ok {
			when = " (" + ago(now().Sub(t.since)) + ")"
		}
		out = append(out, fmt.Sprintf("%s %d%s: %s", t.SettledState, t.N, when, CutTitle(strings.TrimSpace(t.Text), 30)))
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

// NoteOpen prints OpenLine, if anything is open.
func (s *Store) NoteOpen(w io.Writer) {
	if l := s.OpenLine(); l != "" {
		fmt.Fprintln(w, l)
	}
}

// ListOpen prints OpenLines ('open').
func (s *Store) ListOpen(w io.Writer) {
	for _, l := range s.OpenLines() {
		fmt.Fprintln(w, l)
	}
}
