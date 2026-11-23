package viewer

import (
	"os"
	"syscall"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

const (
	margin = 2 // the right gutter
	// The left gutter: the cursor's > in column 0, the marks (●) in column
	// markCol, so neither moves or hides the other.
	lgutter = 4
	markCol = 2
	maxText = 76 // width between the gutters, including the 2-cell mark field
	markW   = 2  // the mark field every line has (▸ ▾ → ? · ✓ ! ⌀)
)

// view is the report on screen.
type view struct {
	*page.Report
	open     map[string]bool
	cur      string         // key of the node under the cursor
	ends     map[string]int // where each stop's rows end (its contents too), from rows
	top      int
	follow   bool // scroll the cursor into view on the next render
	curRow   int
	curShown bool
	// lock is set when a reload could not find the cursor's line: the cursor
	// went to the nearest stop, and nothing is recorded until j/k.
	lock bool
	// placed is set when the viewer put the cursor on a line by itself (at
	// start, or when stops appear on a page that had none). The person has
	// not pointed at that line, so y, n and options wait for j/k.
	placed bool
	// showHidden draws the lines the person hid faintly instead of leaving
	// them out (H).
	showHidden bool
	// b is the board the report is on: what is hidden, marked, ...
	b *page.Board
	// auto is the line the viewer moved the cursor to by itself (after x or
	// H took the cursor's line away), and autoAt when a frame first showed
	// the cursor there. Until settleDelay has passed since, x y n and options
	// are dropped: a key pressed twice, or pressed before the screen showed
	// the new line, must not act on a line the person has not seen.
	auto   string
	autoAt time.Time
	// atEnd says whether the last frame showed the end of the page: j past
	// the last stop then goes into the thoughts pane.
	atEnd bool
	// viewH is how many rows of the page the last frame showed.
	viewH int
}

// settleDelay is how long a line the viewer moved the cursor to must have
// been on screen before x y n and options work on it.
const settleDelay = 400 * time.Millisecond

// clock is the time (tests replace it).
var clock = time.Now

type input struct {
	edit  int    // the thought being rewritten (e), or 0 for a new one
	paste int    // where a paste began (rune index), -1 outside one
	id    string // the line the thought is about, if any
	title string // that line's title when writing began
	label string
	ed    edit // the text and the cursor in it
	etop  int  // the first row of the text in view, when it is taller than the editor
	// at is the report line the editor goes under when it is not about a
	// line (m: the cursor's line when it opened).
	at string
	// Laid out by drawEditing: how far the editor is indented (to the
	// title of the line it goes under), and whether it shows its label.
	ind       int
	showLabel bool
}

func newInput(id, title, label string) *input {
	return &input{id: id, title: title, label: label, ed: newEdit(""), paste: -1}
}

type app struct {
	t    *term
	base string
	st   *board.Store // nil until a session exists
	rep  *view
	msg  string // shown on the bottom line until the next key
	// msgLines is how many lines msg may take (0: one). Only what says why
	// sending failed and how to fix it takes two.
	msgLines int

	// b is the board as any screen shows it: the record, the marks, what
	// the person can do (internal/page). Its Report is rep's.
	b *page.Board

	in *input // the editor at the bottom, while writing
	// inAt is where the editor's input position is among the bottom lines
	// (line, column; 0-based), set when they are laid out.
	inAt [2]int
	help bool // the list of all keys
	// jumping is set while g waits for the letter of where to go; jumpTo
	// is the places, lettered when g was pressed.
	jumping bool
	jumpTo  []*report.Node
	// kbd is set when the terminal answered that it tells keys apart
	// (kitty keyboard protocol): shift+enter is then a key of its own.
	kbd  bool
	htop int // how far the list of keys is scrolled

	// unpinned is the notices opened too tall to pin above the page: they
	// head the page instead (see notice.go).
	unpinned map[string]bool

	// dropBefore: keys read before this time are dropped (pressed while
	// sending).
	dropBefore time.Time

	// tauto is the thought the viewer moved the cursor to after x, and
	// tautoAt when a frame first showed it (see settling). (What u takes
	// back is a.b.Hist.)
	tauto   int
	tautoAt time.Time

	// inPane is set while the cursor is in the thoughts pane under the
	// report (reached with j from the report's end): the pane grows to half
	// the screen. pcur is the thought under the cursor, popen the thoughts
	// opened there, ptop how far the pane is scrolled.
	inPane bool
	pcur   int
	popen  map[int]bool
	ptop   int

	list      bool        // the thoughts list instead of the report
	lcur      int         // thought number under the cursor in the list
	lends     map[int]int // where each thought's rows end in the list, from listRows
	lopen     map[int]bool
	ltop      int
	lfollow   bool
	lcurShown bool
}

// Run opens the board of st, or (st nil) waits for the first session
// to appear under base and opens it.
// When a signal stops it, exit is the code to end the program with.
func Run(base string, st *board.Store) (exit int, err error) {
	t, err := openTerm()
	if err != nil {
		return 0, err
	}
	defer t.close()
	a := &app{t: t, base: base, lopen: map[int]bool{}}
	a.setStore(st)
	if sig := a.loop(); sig != nil {
		// Stopped by a signal: the terminal is given back by the deferred
		// close, then the exit code says which signal it was.
		if s, ok := sig.(syscall.Signal); ok {
			return 128 + int(s), nil
		}
		return 1, nil
	}
	return 0, nil
}

func (a *app) setStore(st *board.Store) {
	a.st = st
	a.b = page.Open(st)
	a.rep = &view{Report: a.b.Report, b: a.b, open: map[string]bool{}}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// safely runs f and turns a panic into an error.
func safely(f func() error) error { return page.Safely(f) }

// loadEvents reads the record again.
func (a *app) loadEvents() { a.b.Load() }

func (a *app) top() *view { return a.rep }

// loop runs the viewer until q, or until a signal stops it (returned).
func (a *app) loop() os.Signal {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	a.render()
	for {
		select {
		case sig := <-a.t.stop:
			return sig
		case k, ok := <-a.t.keys:
			if !ok {
				return nil
			}
			if !k.at.IsZero() && k.at.Before(a.dropBefore) {
				continue
			}
			quit := false
			if err := safely(func() error { quit = a.handle(k); return nil }); err != nil {
				a.msg = "できませんでした: " + err.Error()
			}
			if quit {
				return nil
			}
		case <-a.t.winch:
			a.t.resize()
			a.keepCursor()
		case <-tick.C:
			if a.st == nil {
				if ss := board.Sessions(a.base); len(ss) > 0 {
					a.setStore(board.Open(a.base, ss[0]))
				}
				break
			}
			if a.b.NoSession {
				if _, err := os.Stat(a.st.Dir); err == nil {
					a.setStore(a.st)
				}
				break
			}
			a.tickReload()
			a.b.Poll()
		}
		a.render()
	}
}

// render draws the screen; if drawing fails, it says so instead of
// taking the viewer down.
func (a *app) render() {
	if err := safely(func() error { a.draw(); return nil }); err != nil {
		a.t.draw([]string{"", "  描画できませんでした: " + clean(err.Error()), "", "  q 終了"}, nil)
	}
}

// ---- reloading ----

// tickReload reads the report again when it was rewritten. A rewrite
// while g waits leaves it: the letters were given to the report as it was.
func (a *app) tickReload() {
	pv := a.top()
	if !pv.Stale() {
		return
	}
	a.reload(pv)
	if a.jumping {
		a.stopJump()
		a.msg = msgMoved
	}
}

// reload reads the file again and keeps the cursor on the same id. When the
// id is gone, the cursor goes to the nearest stop and reactions are locked
// until j/k, so nothing is ever recorded against a line the person did not
// point at.
func (a *app) reload(pv *view) {
	old := pv.nodeAt(pv.cur)
	pv.Load()
	if old == nil {
		return
	}
	// Lines added or removed above must not carry the cursor's line off the
	// screen: if it was in view, the screen follows it.
	if pv.curShown {
		pv.follow = true
	}
	if now := pv.nodeAt(pv.cur); now != nil {
		// The same id now naming something else (another title, or another
		// kind of part) is a change the person must look at before reacting.
		// A Fold's title is only a label, and status, note and state are not
		// what the line is.
		if now.Part() != old.Part() || (old.Kind != report.KindDetails && now.Title() != old.Title()) {
			pv.follow, pv.lock = true, true
		}
		return
	}
	pv.cur, pv.follow, pv.lock = a.nearest(pv, pv.curRow), true, true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
