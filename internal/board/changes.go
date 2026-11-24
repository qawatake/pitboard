package board

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// Unsent counts what the person has not sent yet: what the next send would
// deliver (a thought taken back before it was sent is not counted).
func Unsent(evs []Event) int { return len(UnsentEvents(evs)) }

// UnsentEvents is what the person has done since the latest send, as it
// would be told to the AI.
func UnsentEvents(evs []Event) []Event { return deliverable(evs, lastSend(evs), noCut) }

// SentUnread counts what was sent but the AI has not read yet.
func SentUnread(evs []Event, seen int) int { return len(deliverable(evs, seen, lastSend(evs))) }

// Changes returns the person's actions the AI has not been given yet, one
// line each with context from the report, and remembers that they were given.
func (s *Store) Changes() ([]string, error) {
	if _, err := os.Stat(s.EventsPath()); os.IsNotExist(err) {
		return nil, nil // the person has done nothing yet
	}
	var out []string
	err := s.locked(func(_ *os.File, evs []Event) error {
		seen := s.Seen()
		var nodes []*report.Node
		if b, err := os.ReadFile(s.ReportPath()); err == nil {
			nodes, _, _ = report.Parse(string(b))
		}
		// Only what the person has sent (what is before the latest send)
		// is delivered; what comes after waits for the next send.
		last := lastSend(evs)
		if last <= seen {
			return nil
		}
		out = compact(deliverable(evs, seen, last), nodes, s.imagePath)
		if last == seen {
			return nil
		}
		return writeFile(s.SeenPath(), []byte(strconv.Itoa(last)+"\n"))
	})
	return out, err
}

// Pending is the events changes would deliver next, without delivering
// them.
func (s *Store) Pending() []Event {
	evs := s.Events()
	return deliverable(evs, s.Seen(), lastSend(evs))
}

// lastSend is the number of the latest send (0: none yet).
func lastSend(evs []Event) int {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Op == "send" {
			return evs[i].N
		}
	}
	return 0
}

// noCut is a cut past every event.
const noCut = int(^uint(0) >> 1)

// deliverable is what of the events after seen and before cut is told to
// the AI (settle, the AI's own, and send are not; events from cut on are
// not considered at all). A thought taken back before it was delivered is not
// told at all, nor is its taking back: the AI never knew it. For a thought
// already delivered, what is told is whether it now stands or is taken back,
// if that differs from what the AI was last told: one retract or unretract
// (carrying the thought's text), where the last of them is.
func deliverable(all []Event, seen, cut int) []Event {
	var evs []Event
	for _, e := range dropLegacy(all) {
		if e.N < cut {
			evs = append(evs, e)
		}
	}
	evs = resolveUndos(evs, seen)
	thoughts := map[int]Event{}
	final := map[int]bool{}  // taken back, in the end
	told := map[int]bool{}   // taken back, as the AI was last told
	lastOp := map[int]int{}  // the number of the last retract / unretract
	text := map[int]string{} // the thought's text, as last edited
	lastEdit := map[int]int{}
	known := map[int]string{} // the thought's text as the AI last got it
	for _, e := range evs {
		switch e.Op {
		case "edit":
			text[e.Thought] = e.Text
			if e.N <= seen {
				known[e.Thought] = e.Text
			}
			if e.N > seen {
				lastEdit[e.Thought] = e.N
			}
		case "thought":
			thoughts[e.N] = e
			if _, ok := known[e.N]; !ok {
				known[e.N] = e.Text
			}
		case "retract", "unretract":
			final[e.Thought] = e.Op == "retract"
			if e.N <= seen {
				told[e.Thought] = final[e.Thought]
			} else {
				lastOp[e.Thought] = e.N
			}
		}
	}
	var out []Event
	for _, e := range evs {
		if e.N <= seen {
			continue
		}
		switch e.Op {
		case "settle", "send":
		case "thought":
			// Not delivered yet: what is told is its text as edited, and
			// the edits are not told.
			if t, ok := text[e.N]; ok {
				e.Text = t
			}
			if !final[e.N] {
				out = append(out, e)
			}
		case "edit":
			// A delivered thought rewritten: its latest text, once.
			t, ok := thoughts[e.Thought]
			if !ok || t.N > seen || final[e.Thought] || lastEdit[e.Thought] != e.N {
				continue
			}
			out = append(out, e)
		case "retract", "unretract":
			t, ok := thoughts[e.Thought]
			if !ok || t.N > seen || lastOp[e.Thought] != e.N || final[e.Thought] == told[e.Thought] {
				continue
			}
			e.Op, e.Text = "unretract", known[e.Thought]
			if final[e.Thought] {
				e.Op = "retract"
			}
			out = append(out, e)
		default:
			out = append(out, e)
		}
	}
	return out
}

// Lengths titles are cut to in what 'changes' prints.
const (
	hideTitleMax    = 20
	thoughtTitleMax = 30
)

// compact is what 'changes' prints for evs: one line per event (with the
// images a thought holds under it), except that a reaction pressed again on
// the same line is told once (the last), and hides and unhides are told on
// one line each, at the end.
func compact(evs []Event, nodes []*report.Node, image func(n int) string) []string {
	lastReact := map[string]int{}
	for _, e := range evs {
		if e.Op == "react" {
			lastReact[e.ID] = e.N
		}
	}
	var out, hides, unhides []string
	for _, e := range evs {
		switch {
		case e.Op == "react" && lastReact[e.ID] != e.N:
			continue
		case e.Op == "hide":
			hides = append(hides, CutTitle(e.Title, hideTitleMax))
			continue
		case e.Op == "unhide":
			unhides = append(unhides, CutTitle(e.Title, hideTitleMax))
			continue
		}
		line := describe(e, nodes)
		if line == "" {
			continue
		}
		out = append(out, line)
		if e.Op == "thought" || e.Op == "edit" {
			for _, n := range imagesIn(e.Text) {
				if p := image(n); p != "" {
					out = append(out, fmt.Sprintf("  image %d: %s", n, p))
				}
			}
		}
	}
	if len(hides) > 0 {
		out = append(out, fmt.Sprintf("hide (%d): %s", len(hides), strings.Join(hides, " / ")))
	}
	if len(unhides) > 0 {
		out = append(out, fmt.Sprintf("unhide (%d): %s", len(unhides), strings.Join(unhides, " / ")))
	}
	return out
}

// CutTitle is a title on one line, cut to max characters with ….
func CutTitle(s string, max int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return string(r)
}

// describe is how an event is told to the AI: with the title of the line as
// the person saw it, and what it says now if that differs.
func describe(e Event, nodes []*report.Node) string {
	now := report.ByID(nodes, e.ID)
	seen := e.Title
	if seen == "" && now != nil {
		seen = now.Title() // recorded before titles were kept
	}
	ctx := seen
	switch {
	case now == nil && seen == "":
		ctx = "gone"
	case now == nil:
		ctx += " → now gone"
	case now.Title() != seen:
		ctx += " → now: " + now.Title()
	}
	switch e.Op {
	case "react":
		opt := e.Option
		if opt == "" && now != nil {
			if ch := now.Choice(e.Value); ch != nil {
				opt = report.PlainOf(ch.Text)
			}
		}
		if ctx == seen {
			return fmt.Sprintf("answer %s = %s (%s)", e.ID, e.Value, opt)
		}
		return fmt.Sprintf("answer %s = %s (%s; question: %s)", e.ID, e.Value, opt, ctx)
	case "thought":
		text := strings.ReplaceAll(strings.TrimSpace(e.Text), "\n", "\n  ")
		if e.ID == "" {
			return fmt.Sprintf("thought %d: %s", e.N, text)
		}
		// The line's titles, cut: the thought is what matters here.
		ctx := CutTitle(seen, thoughtTitleMax)
		switch {
		case now == nil && seen == "":
			ctx = "gone"
		case now == nil:
			ctx += " → now gone"
		case now.Title() != seen:
			ctx += " → now: " + CutTitle(now.Title(), thoughtTitleMax)
		}
		if title, ok := report.FoldTitle(e.ID); ok {
			// A Fold without an id: named by its title, which is what finds it.
			where := fmt.Sprintf("fold %q", CutTitle(report.PlainOf(title), thoughtTitleMax))
			if now == nil {
				where += " (now gone)"
			}
			return fmt.Sprintf("thought %d on %s: %s", e.N, where, text)
		}
		return fmt.Sprintf("thought %d on %s (%s): %s", e.N, e.ID, ctx, text)
	case "hide", "unhide":
		return fmt.Sprintf("%s: %s", e.Op, strings.ReplaceAll(e.Title, "\n", " "))
	case "edit":
		text := strings.ReplaceAll(strings.TrimSpace(e.Text), "\n", "\n  ")
		return fmt.Sprintf("edit thought %d: %s", e.Thought, text)
	case "undo":
		t := e
		t.Op = e.of
		if s := describe(t, nodes); s != "" {
			return "retract " + s
		}
		return ""
	case "retract", "unretract":
		head := []rune(strings.Join(strings.Fields(e.Text), " "))
		if len(head) > 40 {
			head = append(head[:39], '…')
		}
		return fmt.Sprintf("%s thought %d: %s", e.Op, e.Thought, string(head))
	}
	return "" // settle is written by the AI itself
}

// Settle records where thought n went (done).
func (s *Store) Settle(n int, text string) error {
	return s.SettleAll([]Settlement{{n, "done", text}})
}

// Settlement is one thought's settle: its number, state (doing, hold or
// done) and text.
type Settlement struct {
	N     int
	State string
	Text  string
}

// SettleAll records each settlement: all of them, or (when a number is not
// a thought, or a done one has no text) none.
func (s *Store) SettleAll(ss []Settlement) error {
	evs := s.Events()
	for i := range ss {
		x := &ss[i]
		x.Text = strings.TrimSpace(x.Text)
		if x.State == "" {
			x.State = "done"
		}
		if x.Text == "" && x.State == "done" {
			return fmt.Errorf("settle %d needs a text", x.N)
		}
		if findThought(evs, x.N) == nil {
			return fmt.Errorf("no thought %d", x.N)
		}
	}
	for _, x := range ss {
		e := Event{Op: "settle", Thought: x.N, Text: x.Text, By: "ai"}
		if x.State != "done" {
			e.State = x.State
		}
		if _, err := s.Append(e); err != nil {
			return err
		}
	}
	return nil
}
