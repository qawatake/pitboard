package page

import (
	"os"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Board is a session's report and record, as a screen shows them, and what
// the person did on this screen since it opened (for Undo).
type Board struct {
	Store *board.Store // nil until a session exists
	// NoSession: the session given does not exist (yet). Nothing is
	// recorded then: a screen never makes a session.
	NoSession bool
	Report    *Report
	State     *board.State
	// Viewed is the hashes of what the person has looked at (see marks.go).
	Viewed map[string]bool
	Unsent int  // what the person has not sent yet, as they see it (PendingCount)
	Unread int  // what was sent and the AI has not read yet
	Waiter bool // a 'pitboard wait' is waiting: sending reaches the AI
	// Hist is what the person did since the screen opened, oldest first:
	// what Undo takes back, newest first.
	Hist []Done

	evMod, seenMod, viewedMod time.Time
}

// Open is the board of st (nil: no session yet).
func Open(st *board.Store) *Board {
	b := &Board{Store: st}
	path := ""
	if st != nil {
		path = st.ReportPath()
		if _, err := os.Stat(st.Dir); err != nil {
			b.NoSession = true
		}
	}
	b.Report = NewReport(path)
	b.Load()
	return b
}

// Load reads the record again.
func (b *Board) Load() {
	if b.Store == nil {
		b.State, b.Unsent, b.Unread = board.Replay(nil, 0), 0, 0
		b.Viewed = map[string]bool{}
		return
	}
	b.LoadViewed()
	b.Waiter = b.Store.Waiter() != 0
	b.evMod, b.seenMod = ModTime(b.Store.EventsPath()), ModTime(b.Store.SeenPath())
	evs, seen := b.Store.Events(), b.Store.Seen()
	b.State = board.Replay(evs, seen)
	b.Unsent, b.Unread = PendingCount(board.UnsentEvents(evs)), board.SentUnread(evs, seen)
}

// LoadViewed reads again what the person has looked at (another screen of
// the same board may have added to it).
func (b *Board) LoadViewed() {
	b.viewedMod = ModTime(b.Store.ViewedPath())
	b.Viewed = b.Store.Viewed()
}

// Poll reads again what changed on disk since it was read: the record (it
// reports true then), or else what was looked at; and whether an AI is
// waiting. The report is the screen's to reload (Report.Stale), since it
// moves the screen's cursor.
func (b *Board) Poll() (events bool) {
	if b.Store == nil || b.NoSession {
		return false
	}
	if !ModTime(b.Store.EventsPath()).Equal(b.evMod) || !ModTime(b.Store.SeenPath()).Equal(b.seenMod) {
		b.Load()
		events = true
	} else if !ModTime(b.Store.ViewedPath()).Equal(b.viewedMod) {
		b.LoadViewed()
	}
	b.Waiter = b.Store.Waiter() != 0
	return events
}

// CanWrite says whether the person's actions can be recorded: the session
// must exist.
func (b *Board) CanWrite() bool { return b.Store != nil && !b.NoSession }

// Thought is the thought numbered n on the board (not taken back), or nil.
func (b *Board) Thought(n int) *board.Thought {
	for _, t := range b.State.Thoughts {
		if t.N == n {
			return t
		}
	}
	return nil
}

// IsHidden says whether the person hid n.
func (b *Board) IsHidden(n *report.Node) bool {
	k := report.HideKey(n)
	return k != "" && b.State.Hidden[k]
}

// HiddenCount is how many lines of the report are hidden.
func (b *Board) HiddenCount() int {
	c := 0
	report.WalkAll(b.Report.Nodes, func(n *report.Node) {
		if b.IsHidden(n) {
			c++
		}
	})
	return c
}
