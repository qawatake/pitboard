package board

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Event is one line of events.jsonl.
//
//	react    id, value (an Ask's option key), option (its text), line (what
//	         the line was). Older viewers also wrote y / n on any line
//	         (no option): those are ignored (Legacy).
//	thought  text, and id when written on a line
//	settle   thought, text, by (the AI's note on where a thought went), and
//	         state: doing / hold / done (none: done). The latest counts.
//	hide     key (what the line says, see report.HideKey), title: the person no
//	         longer needs to see that line
//	unhide   key, title: show it again
//	send     the person sent what is above it: 'changes' delivers only
//	         what comes before the latest send
//	retract  thought: the person took the thought back
//	edit     thought, text: the person rewrote the thought (the latest
//	         edit's text is the thought's)
//	unretract  thought: they undid that (older viewers: u right after x)
//	undo     undo: the number of an event of the person's (react, thought,
//	         edit, retract, unretract, hide, unhide) to take back (u). Not
//	         sent yet, the two are as if neither happened; sent, the undo is
//	         new input for the AI (see resolveUndos).
type Event struct {
	N       int    `json:"n"`
	T       string `json:"t"`
	Op      string `json:"op"`
	ID      string `json:"id,omitempty"`
	Value   string `json:"value,omitempty"`
	Text    string `json:"text,omitempty"`
	Thought int    `json:"thought,omitempty"`
	By      string `json:"by,omitempty"`
	// Line is the reacted (or commented) line as the person saw it (see
	// report.Node.Sig). When the AI rewrites the line, the reaction is no
	// longer shown on it, and an Ask commented on is waiting again.
	Line string `json:"line,omitempty"`
	// Title is the line's title when the person reacted or began writing,
	// and Option the text of the chosen option: what the person saw, told to
	// the AI next to what the line says now.
	Title  string `json:"title,omitempty"`
	Option string `json:"option,omitempty"`
	// Key is what a hidden line says (see report.HideKey).
	Key string `json:"key,omitempty"`
	// State is how far the AI is with a thought (settle): doing, hold or
	// done ("" is done).
	State string `json:"state,omitempty"`
	// Undo is the event an undo takes back.
	Undo int `json:"undo,omitempty"`

	// of is, on an undo told to the AI, the op of the event it takes back
	// (never written).
	of string
}

// Legacy says whether e is a y / n reaction of an older viewer (y / n on
// an Item, Link, Notice or Fold, before an Ask was the one way to
// answer). It is not shown, not counted and not told; an answer with
// option key y or n records the option's text, so it is not one.
func (e Event) Legacy() bool {
	return e.Op == "react" && e.Option == "" && (e.Value == "y" || e.Value == "n")
}

// dropLegacy is evs without the y / n reactions of older viewers, and the
// undos of them.
func dropLegacy(evs []Event) []Event {
	gone := map[int]bool{}
	for _, e := range evs {
		if e.Legacy() {
			gone[e.N] = true
		}
	}
	if len(gone) == 0 {
		return evs
	}
	out := make([]Event, 0, len(evs))
	for _, e := range evs {
		if gone[e.N] || (e.Op == "undo" && gone[e.Undo]) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// undoable says whether u can take back an event with op.
func undoable(op string) bool {
	switch op {
	case "react", "thought", "edit", "retract", "unretract", "hide", "unhide":
		return true
	}
	return false
}

// undoneBy maps each event taken back to the undo that took it back.
func undoneBy(evs []Event) map[int]Event {
	out := map[int]Event{}
	for _, e := range evs {
		if e.Op == "undo" && e.Undo > 0 && e.Undo < e.N {
			if _, ok := out[e.Undo]; !ok {
				out[e.Undo] = e
			}
		}
	}
	return out
}

// resolveUndos is evs (all before the cut) with the undos worked out for
// telling the AI, who was told what is up to seen. An event taken back that
// the AI was not told is dropped with its undo: the AI never knew it. One
// the AI was told turns its undo into what now stands: a thought taken
// back is a retract, a retract taken back an unretract (and the other way
// round), an edit taken back an edit back to the text before; a reaction,
// hide or unhide stays an undo (told as "retract ...").
func resolveUndos(evs []Event, seen int) []Event {
	undone := undoneBy(evs)
	if len(undone) == 0 {
		return evs
	}
	byN := map[int]Event{}
	for _, e := range evs {
		byN[e.N] = e
	}
	var out []Event
	for _, e := range evs {
		if _, ok := undone[e.N]; ok && e.N > seen {
			continue
		}
		if e.Op != "undo" {
			out = append(out, e)
			continue
		}
		t, ok := byN[e.Undo]
		if !ok || t.N > seen || undone[t.N].N != e.N || !undoable(t.Op) {
			continue
		}
		switch t.Op {
		case "thought":
			out = append(out, Event{N: e.N, T: e.T, Op: "retract", Thought: t.N})
		case "retract":
			out = append(out, Event{N: e.N, T: e.T, Op: "unretract", Thought: t.Thought})
		case "unretract":
			out = append(out, Event{N: e.N, T: e.T, Op: "retract", Thought: t.Thought})
		case "edit":
			out = append(out, Event{N: e.N, T: e.T, Op: "edit", Thought: t.Thought, Text: textBefore(evs, undone, t)})
		default:
			u := t
			u.N, u.T, u.Op, u.Undo, u.of = e.N, e.T, "undo", t.N, t.Op
			out = append(out, u)
		}
	}
	return out
}

// textBefore is the text thought edit.Thought had before edit (edits taken
// back left out).
func textBefore(evs []Event, undone map[int]Event, edit Event) string {
	text := ""
	for _, e := range evs {
		if e.N >= edit.N {
			break
		}
		switch _, gone := undone[e.N]; {
		case e.Op == "thought" && e.N == edit.Thought:
			text = e.Text
		case e.Op == "edit" && e.Thought == edit.Thought && !gone:
			text = e.Text
		}
	}
	return text
}

func parseEvents(data []byte) []Event {
	var evs []Event
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.N > 0 {
			evs = append(evs, e)
		}
	}
	return evs
}

func (s *Store) Events() []Event {
	b, err := os.ReadFile(s.EventsPath())
	if err != nil {
		return nil
	}
	return parseEvents(b)
}

// locked runs f with events.jsonl open and locked (flock), so that the viewer
// and the AI's commands never interleave or reuse a number.
func (s *Store) locked(f func(file *os.File, evs []Event) error) error {
	if _, err := os.Stat(s.Dir); err != nil {
		return fmt.Errorf("no session %q yet", s.Session)
	}
	file, err := os.OpenFile(s.EventsPath(), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	return f(file, parseEvents(data))
}

// Append numbers e and adds it to events.jsonl.
func (s *Store) Append(e Event) (Event, error) {
	err := s.locked(func(file *os.File, evs []Event) error {
		if e.Op == "settle" || e.Op == "edit" {
			if t := findThought(evs, e.Thought); t == nil {
				return fmt.Errorf("no thought %d", e.Thought)
			}
		}
		e.N = 1
		if len(evs) > 0 {
			e.N = evs[len(evs)-1].N + 1
		}
		if e.T == "" {
			e.T = time.Now().Format(time.RFC3339)
		}
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		_, err = file.Write(append(line, '\n'))
		return err
	})
	return e, err
}

func findThought(evs []Event, n int) *Event {
	for i := range evs {
		if evs[i].Op == "thought" && evs[i].N == n {
			return &evs[i]
		}
	}
	return nil
}

func (s *Store) Seen() int {
	b, err := os.ReadFile(s.SeenPath())
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

// writeFile replaces path through a temporary file in the same directory and
// a rename.
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // no-op after a successful rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
