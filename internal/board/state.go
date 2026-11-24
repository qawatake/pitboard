package board

// ---- what the events add up to, for the viewer ----

type Thought struct {
	Event
	Settled      string // the AI's note, if any
	SettledN     int    // the number of that settle event
	SettledState string // the AI's state for it: "" (none yet), doing, hold, done
	retracted    bool   // the person took it back (not shown)
	LastN        int    // the number of the thought or its latest edit
}

// State is the events replayed: the latest reaction per id, thoughts, and
// the lines the person hid.
type State struct {
	React    map[string]Event
	Thoughts []*Thought      // oldest first
	Seen     int             // what the AI has read ('changes')
	Sent     int             // the latest send: what the person has sent
	Hidden   map[string]bool // report.HideKey of each hidden line
	// Undoing is the lines (by id, and by report.HideKey) with a reaction, hide or
	// unhide that was sent and is now taken back, the taking back not sent
	// yet.
	Undoing, UndoingKey map[string]bool
}

// Replay adds up the events. What was taken back (undo) is as if it
// never happened.
func Replay(evs []Event, seen int) *State {
	evs = dropLegacy(evs)
	st := &State{React: map[string]Event{}, Seen: seen, Sent: lastSend(evs), Hidden: map[string]bool{},
		Undoing: map[string]bool{}, UndoingKey: map[string]bool{}}
	byN := map[int]*Thought{}
	undone := undoneBy(evs)
	for _, e := range evs {
		if _, ok := undone[e.N]; ok {
			// Sent, its taking back not yet: the line says so.
			if u := undone[e.N]; e.N <= st.Sent && u.N > st.Sent {
				switch e.Op {
				case "react":
					st.Undoing[e.ID] = true
				case "hide", "unhide":
					st.UndoingKey[e.Key] = true
				}
			}
			continue
		}
		switch e.Op {
		case "undo":
			// Taking back a sent change to a thought is new input on it.
			if t := byN[e.Undo]; t != nil && e.Undo <= st.Sent {
				t.LastN = max(t.LastN, e.N)
			}
			for _, x := range evs {
				if x.N == e.Undo && x.Thought != 0 && x.N <= st.Sent {
					if t := byN[x.Thought]; t != nil {
						t.LastN = max(t.LastN, e.N)
					}
				}
			}
		case "react":
			st.React[e.ID] = e
		case "thought":
			t := &Thought{Event: e, LastN: e.N}
			st.Thoughts = append(st.Thoughts, t)
			byN[e.N] = t
		case "settle":
			if t := byN[e.Thought]; t != nil {
				t.Settled, t.SettledN, t.SettledState = e.Text, e.N, e.State
				if t.SettledState == "" {
					t.SettledState = "done"
				}
			}
		case "retract", "unretract":
			if t := byN[e.Thought]; t != nil {
				t.retracted = e.Op == "retract"
			}
		case "edit":
			// Rewritten: the AI's settle was for the old text.
			if t := byN[e.Thought]; t != nil {
				t.Text, t.LastN = e.Text, e.N
				t.Settled, t.SettledN, t.SettledState = "", 0, ""
			}
		case "hide":
			st.Hidden[e.Key] = true
		case "unhide":
			delete(st.Hidden, e.Key)
		}
	}
	// A thought taken back is gone from the board.
	kept := st.Thoughts[:0]
	for _, t := range st.Thoughts {
		if !t.retracted {
			kept = append(kept, t)
		}
	}
	st.Thoughts = kept
	return st
}

func (st *State) On(id string) []*Thought {
	var out []*Thought
	for _, t := range st.Thoughts {
		if t.ID == id {
			out = append(out, t)
		}
	}
	return out
}

// Finished says whether the AI is done with t.
func (t *Thought) Finished() bool { return t.SettledState == "done" }

// Status is how far a comment has got: not sent, sent (not read by the AI
// yet), read, then as the AI settled it: being handled, on hold, handled.
// The viewer puts it in words (its statusWord).
type Status int

const (
	StatusUnsent Status = iota
	StatusSent
	StatusRead
	StatusDoing
	StatusHold
	StatusDone
)

// StatusOf is a comment's status: as the AI settled it, or else how far
// it was sent.
func (st *State) StatusOf(t *Thought) Status {
	if t.SettledState != "" {
		return settleStatus(t.SettledState)
	}
	return st.SentStatus(t.LastN)
}

// SentStatus is the status of something sent: sent until the AI reads it,
// then read (or unsent when it was not sent yet).
func (st *State) SentStatus(n int) Status {
	switch {
	case n > st.Sent:
		return StatusUnsent
	case n > st.Seen:
		return StatusSent
	}
	return StatusRead
}

func settleStatus(state string) Status {
	switch state {
	case "doing":
		return StatusDoing
	case "hold":
		return StatusHold
	}
	return StatusDone
}
