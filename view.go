package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	margin  = 2  // left and right gutter; the cursor mark sits in the left one
	maxText = 76 // width between the gutters, including the 2-cell mark field
	markW   = 2  // the mark field every line has (▸ ▾ → ? ○ · ~ ✓)

	msgChanged   = "報告が書き換わりました。もう一度押してください"
	msgMoved     = "報告が書き換わりました。カーソルの行を確かめてください"
	msgNoReport  = "AI はまだ報告を書いていません"
	msgNoBoard   = "このディレクトリと親に板 (.pitboard/) がありません。AI を動かしているディレクトリで開いてください"
	msgNoSession = "板はまだありません。AI が報告を書くと、ここに出ます"
)

// view is the report on screen.
type view struct {
	path     string
	mtime    time.Time
	err      string
	missing  bool // the report does not exist yet
	nodes    []*node
	probs    []Problem // what is wrong with the report
	readErr  string    // the last read failed (a bug); the previous screen is kept
	open     map[string]bool
	cut      map[string]bool // stops whose line did not fit and was cut with …
	cur      string          // key of the node under the cursor
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
	// hidden is the lines the person hid (by hideKey); showHidden draws them
	// faintly instead of leaving them out (H).
	hidden     map[string]bool
	showHidden bool
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
	// dwellKey is the cursor's line (and what it says) since dwellAt: resting
	// there for settleDelay marks it as looked at (see dwell).
	dwellKey string
	dwellAt  time.Time
}

// settleDelay is how long a line the viewer moved the cursor to must have
// been on screen before x y n and options work on it.
const settleDelay = 400 * time.Millisecond

// clock is the time (tests replace it).
var clock = time.Now

// settling says whether the cursor's line was put there by the viewer and
// has not been on screen for settleDelay yet.
func (pv *view) settling() bool {
	return pv.auto != "" && pv.auto == pv.cur && (pv.autoAt.IsZero() || clock().Sub(pv.autoAt) < settleDelay)
}

type input struct {
	edit  int    // the thought being rewritten (e), or 0 for a new one
	paste int    // where a paste began (rune index), -1 outside one
	id    string // the line the thought is about, if any
	title string // that line's title when writing began
	label string
	ed    edit // the text and the cursor in it
	etop  int  // the first row of the text in view, when it is taller than the editor
}

func newInput(id, title, label string) *input {
	return &input{id: id, title: title, label: label, ed: newEdit(""), paste: -1}
}

type app struct {
	t    *term
	base string
	st   *Store // nil until a session exists
	rep  *view
	msg  string // shown on the bottom line until the next key
	// msgLines is how many lines msg may take (0: one). Only what says why
	// sending failed and how to fix it takes two.
	msgLines int

	evMod, seenMod time.Time
	// viewed is the hashes of what the person has looked at (see marks.go).
	viewed    map[string]bool
	viewedMod time.Time
	state     *state
	unsent    int  // what the person has not sent yet (reactions, thoughts, hides)
	unread    int  // what was sent and the AI has not read yet
	waiter    bool // a 'pitboard wait' is waiting: sending reaches the AI

	in *input // the editor at the bottom, while writing
	// inAt is where the editor's input position is among the bottom lines
	// (line, column; 0-based), set when they are laid out.
	inAt [2]int
	help bool // the list of all keys
	// kbd is set when the terminal answered that it tells keys apart
	// (kitty keyboard protocol): shift+enter is then a key of its own.
	kbd  bool
	htop int // how far the list of keys is scrolled

	noSession bool // the session given with -s does not exist (yet)

	// dropBefore: keys read before this time are dropped (pressed while
	// sending).
	dropBefore time.Time

	// undo is the thought just taken back, while u can still undo it (the
	// very next key). tauto is the thought the viewer moved the cursor to
	// after that, and tautoAt when a frame first showed it (see settling).
	undo    int
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

	list      bool // the thoughts list instead of the report
	lcur      int  // thought number under the cursor in the list
	lopen     map[int]bool
	ltop      int
	lfollow   bool
	lcurShown bool
}

// runViewer opens the board of st, or (st nil) waits for the first session
// to appear under base and opens it.
func runViewer(base string, st *Store) error {
	t, err := openTerm()
	if err != nil {
		return err
	}
	defer t.close()
	a := &app{t: t, base: base, lopen: map[int]bool{}}
	a.setStore(st)
	if sig := a.loop(); sig != nil {
		// Stopped by a signal: the terminal is given back by the deferred
		// close, then the exit code says which signal it was.
		if s, ok := sig.(syscall.Signal); ok {
			return exitError(128 + int(s))
		}
		return exitError(1)
	}
	return nil
}

func (a *app) setStore(st *Store) {
	a.st = st
	path := ""
	a.noSession = false
	if st != nil {
		path = st.reportPath()
		if _, err := os.Stat(st.dir); err != nil {
			a.noSession = true
		}
	}
	rep := &view{path: path, open: map[string]bool{}, cut: map[string]bool{}}
	rep.load()
	a.rep = rep
	a.loadEvents()
}

func modTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

func (pv *view) load() {
	if pv.path == "" {
		pv.missing = true
		return
	}
	pv.mtime = modTime(pv.path)
	b, err := os.ReadFile(pv.path)
	if err != nil {
		if os.IsNotExist(err) {
			pv.missing, pv.err, pv.nodes, pv.probs = true, "", nil, nil
			return
		}
		pv.err = err.Error()
		return
	}
	pv.err, pv.missing = "", false
	src := strings.ReplaceAll(string(b), "\r\n", "\n")
	// Whatever goes wrong while reading, the viewer stays up and keeps what
	// it showed before.
	err = safely(func() error {
		nodes, probs, err := parseReport(src)
		if err != nil {
			return err
		}
		pv.nodes, pv.probs = nodes, probs
		return nil
	})
	pv.readErr = ""
	if err != nil {
		pv.readErr = "報告を読めませんでした (前の画面を出しています): " + err.Error()
	}
}

// safely runs f and turns a panic into an error.
func safely(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return f()
}

func (a *app) loadEvents() {
	if a.st == nil {
		a.state, a.unsent, a.unread = replayEvents(nil, 0), 0, 0
		a.viewed = map[string]bool{}
		return
	}
	a.loadViewed()
	a.waiter = a.st.Waiter() != 0
	a.evMod, a.seenMod = modTime(a.st.eventsPath()), modTime(a.st.seenPath())
	evs, seen := a.st.Events(), a.st.Seen()
	a.state = replayEvents(evs, seen)
	a.unsent, a.unread = unsent(evs), sentUnread(evs, seen)
	if a.rep != nil {
		a.rep.hidden = a.state.hidden
	}
}

func (a *app) loadViewed() {
	a.viewedMod = modTime(a.st.viewedPath())
	a.viewed = a.st.Viewed()
}

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
				if ss := sessions(a.base); len(ss) > 0 {
					a.setStore(newStore(a.base, ss[0]))
				}
				break
			}
			if a.noSession {
				if _, err := os.Stat(a.st.dir); err == nil {
					a.setStore(a.st)
				}
				break
			}
			if pv := a.top(); !modTime(pv.path).Equal(pv.mtime) {
				a.reload(pv)
			}
			if !modTime(a.st.eventsPath()).Equal(a.evMod) || !modTime(a.st.seenPath()).Equal(a.seenMod) {
				a.loadEvents()
			} else if !modTime(a.st.viewedPath()).Equal(a.viewedMod) {
				a.loadViewed() // another viewer of the same board
			}
			a.waiter = a.st.Waiter() != 0
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

// reload reads the file again and keeps the cursor on the same id. When the
// id is gone, the cursor goes to the nearest stop and reactions are locked
// until j/k, so nothing is ever recorded against a line the person did not
// point at.
func (a *app) reload(pv *view) {
	old := pv.nodeAt(pv.cur)
	pv.load()
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
		if now.part() != old.part() || (old.kind != kDetails && now.title() != old.title()) {
			pv.follow, pv.lock = true, true
		}
		return
	}
	pv.cur, pv.follow, pv.lock = a.nearest(pv, pv.curRow), true, true
}

// nearest is the key of the stop closest to screen row r.
func (a *app) nearest(pv *view, r int) string {
	best, bd := "", -1
	for i, row := range a.rows(pv) {
		if row.key == "" {
			continue
		}
		if d := abs(i - r); bd < 0 || d < bd {
			best, bd = row.key, d
		}
	}
	return best
}

// keepCursor makes the next frame scroll to the cursor's line if it is in
// view now: when the screen changes size, or the bottom takes more room,
// the line the person is on stays on screen.
func (a *app) keepCursor() {
	if pv := a.top(); pv != nil && pv.curShown {
		pv.follow = true
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ---- keys ----

func (a *app) handle(k key) (quit bool) {
	if k.name == "kbd" {
		a.kbd = true // the terminal's answer, not a key the person pressed
		return false
	}
	a.msg, a.msgLines = "", 0
	if k.name == "ctrl+c" {
		return true
	}
	if a.in == nil {
		k = imeFree(k)
	}
	// u right after taking a thought back undoes it; any other key ends
	// the chance.
	if undo := a.undo; undo != 0 {
		a.undo = 0
		if k.r == 'u' && !a.help && a.in == nil {
			a.unretract(undo)
			return false
		}
	}
	if a.help {
		a.handleHelp(k)
		return false
	}
	if a.in != nil {
		a.handleInput(k)
		return false
	}
	if k.r == '?' {
		a.help = true
		return false
	}
	if a.list {
		return a.handleList(k)
	}
	if a.inPane {
		return a.handlePane(k)
	}
	pv := a.top()
	switch {
	case k.r == 'q':
		return true
	case (k.r == 'j' || k.name == "down") && pv.offScreen() > 0,
		(k.r == 'k' || k.name == "up") && pv.offScreen() < 0:
		// The cursor's line is off the screen in this direction: bring it
		// back first, without moving it. The next j/k moves from there.
		pv.follow = true
	case k.r == 'j' || k.name == "down":
		if pv.step(1) {
			pv.lock, pv.placed = false, false
			break
		}
		if pv.atEnd && len(a.paneOrder()) > 0 {
			// Past the end of the page: into the thoughts under it. The
			// report's cursor stays where it is for coming back.
			a.inPane, a.pcur = true, a.paneOrder()[0].N
			break
		}
		pv.lock, pv.placed = false, false
		pv.top++ // past the last stop: scroll the page
	case k.r == ' ':
		a.jump(pv)
	case k.r == 'A':
		a.viewAll(pv)
	case k.r == 'k' || k.name == "up":
		pv.lock, pv.placed = false, false
		if !pv.step(-1) {
			pv.top = max(pv.top-1, 0)
		}
	case k.name == "pgup":
		pv.top = max(pv.top-max(a.t.h/2, 1), 0)
	case k.name == "pgdn":
		if !pv.atEnd {
			pv.top += max(a.t.h/2, 1) // draw keeps it within the page
		}
	case k.name == "tab":
		a.list, a.lfollow = true, true
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "コメントを書く")
	case k.r == 's':
		a.wake()
	case k.name == "enter" && pv.curShown:
		a.enter(pv)
	case k.r == 'x':
		a.hide(pv)
	case k.r == 'H':
		before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
		pv.showHidden = !pv.showHidden
		pv.moveFrom(before, i)
	case k.r != 0:
		a.react(pv, k.r)
	}
	return false
}

// imeFree makes the keys work with a Japanese input method left on, outside
// the editor: full-width letters and symbols (and the full-width space)
// become their ASCII keys, and ctrl+n / ctrl+p are down / up (keys that do
// not go through the input method).
func imeFree(k key) key {
	switch {
	case k.r >= 0xff01 && k.r <= 0xff5e:
		k.r -= 0xfee0
	case k.r == 0x3000:
		k.r = ' '
	case k.name == "ctrl+n":
		k.name = "down"
	case k.name == "ctrl+p":
		k.name = "up"
	}
	return k
}

// target is the line keys act on: the cursor's line, only while it is on
// screen and the page has not changed under it.
func (a *app) target(pv *view) *node {
	if !pv.curShown || pv.lock {
		return nil
	}
	n := pv.nodeAt(pv.cur)
	if n == nil || n.id == "" || n.kind == kDetails || !a.canWrite() {
		return nil
	}
	return n
}

// keysFor lists what the reaction keys do on n, in the order shown. An
// Option whose key the viewer uses itself (a check error) cannot be chosen.
func keysFor(n *node) (yn bool, choices []string, thought bool) {
	switch n.kind {
	case kTodo, kDoing, kDone, kLink:
		yn = true
	case kQuestion:
		for _, c := range n.kids {
			if c.kind == kChoice && len(c.label) == 1 && !strings.Contains(viewerKeys+"?", c.label) {
				choices = append(choices, c.label)
			}
		}
	}
	return yn, choices, true
}

func (a *app) react(pv *view, r rune) {
	n := a.target(pv)
	if n == nil {
		return
	}
	yn, choices, thought := keysFor(n)
	if n.kind == kQuestion && !pv.open[n.key] && contains(choices, string(r)) {
		// The options are not on screen: the key only opens the question.
		pv.open[n.key] = true
		return
	}
	if pv.placed {
		yn, choices = false, nil // the person has not pointed at this line yet
	}
	value := ""
	switch s := string(r); {
	case yn && (s == "y" || s == "n"):
		value = s
	case contains(choices, s):
		value = s
	case thought && s == "c":
		a.markViewed(n)
		a.in = newInput(n.id, n.title(), "「"+plainText(n.title(), "").capped(4*a.textWidth()).plain()+"」にコメント")
		return
	default:
		return
	}
	if pv.settling() {
		return // dropped, not kept for later
	}
	// Read the report again: the reaction is recorded only if the line is
	// still what the person is looking at.
	b, err := os.ReadFile(pv.path)
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	fresh, _, perr := parseReport(string(b))
	now := byID(fresh, n.id)
	if perr != nil || now == nil || now.sig() != n.sig() {
		a.reload(pv)
		if !pv.lock {
			a.msg = msgChanged
		}
		return
	}
	e := Event{Op: "react", ID: n.id, Value: value, Line: n.sig(), Title: n.title()}
	if ch := n.choice(value); n.kind == kQuestion && ch != nil {
		e.Option = plainOf(ch.text)
	}
	if _, err := a.st.Append(e); err != nil {
		a.msg = "記録できませんでした: " + err.Error()
	}
	a.markViewed(n)
	a.loadEvents()
}

// offScreen says where the cursor's line is when it is not on screen: -1
// above, 1 below, 0 on screen (or no cursor).
func (pv *view) offScreen() int {
	if pv.curShown || pv.curRow < 0 || pv.nodeAt(pv.cur) == nil {
		return 0
	}
	if pv.curRow < pv.top {
		return -1
	}
	return 1
}

// hideTarget is the line x acts on: the cursor's line under the same rules
// as reactions (on screen, pointed at by the person, page unchanged).
func (a *app) hideTarget(pv *view) *node {
	if !pv.curShown || pv.lock || pv.placed || !a.canWrite() {
		return nil
	}
	n := pv.nodeAt(pv.cur)
	if n == nil || hideKey(n) == "" {
		return nil
	}
	return n
}

// hide hides the cursor's line, or shows it again when it is hidden (and
// hidden lines are being shown). The report is not changed: the record says
// what the line says, and the viewer leaves out lines that say that.
func (a *app) hide(pv *view) {
	n := a.hideTarget(pv)
	if n == nil || pv.settling() {
		return
	}
	// As with y and n: read the report again, and record only if the line
	// is still what the screen shows.
	b, err := os.ReadFile(pv.path)
	if err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	fresh, _, perr := parseReport(string(b))
	var now *node
	walkAll(fresh, func(x *node) {
		if now == nil && x.key == n.key {
			now = x
		}
	})
	if perr != nil || now == nil || now.sig() != n.sig() || hideKey(now) != hideKey(n) {
		a.reload(pv)
		if !pv.lock {
			a.msg = msgChanged
		}
		return
	}
	op, msg := "hide", "隠しました (H で表示)"
	switch {
	case pv.isHidden(n):
		op, msg = "unhide", "表示に戻しました"
	case pv.showHidden:
		msg = "隠しました"
	}
	a.markViewed(n)
	before, i := pv.navKeys(), index(pv.navKeys(), pv.cur)
	if _, err := a.st.Append(Event{Op: op, Key: hideKey(n), Title: n.title()}); err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return
	}
	a.loadEvents()
	pv.moveFrom(before, i)
	a.msg = msg
}

// moveFrom keeps the cursor on a line still shown after lines were hidden:
// if its line went, it goes to the next line that is still there (or the
// one before), as j would.
func (pv *view) moveFrom(before []string, i int) {
	if pv.nodeAt(pv.cur) != nil || i < 0 {
		return
	}
	now := pv.navKeys()
	for _, k := range before[i+1:] {
		if index(now, k) >= 0 {
			pv.cur, pv.follow, pv.auto, pv.autoAt = k, true, k, time.Time{}
			return
		}
	}
	for j := i - 1; j >= 0; j-- {
		if k := before[j]; index(now, k) >= 0 {
			pv.cur, pv.follow, pv.auto, pv.autoAt = k, true, k, time.Time{}
			return
		}
	}
}

// isHidden says whether the person hid n.
func (pv *view) isHidden(n *node) bool {
	k := hideKey(n)
	return k != "" && pv.hidden[k]
}

// hiddenCount is how many lines of the report are hidden.
func (pv *view) hiddenCount() int {
	c := 0
	walkAll(pv.nodes, func(n *node) {
		if pv.isHidden(n) {
			c++
		}
	})
	return c
}

// settleCursor puts the cursor on the first stop when it is on none. The
// person did not choose that line, so it is marked placed.
func (pv *view) settleCursor() {
	if pv.nodeAt(pv.cur) == nil {
		pv.cur = ""
		pv.step(0)
		pv.placed = pv.cur != ""
	}
}

// navigable says whether the cursor stops at n: a Fold, a part with an id,
// or a plain "- " line (which can only be hidden; not a numbered one, and
// never a nested one).
func (pv *view) navigable(n *node) bool {
	return n.kind == kDetails || n.id != "" || (n.kind == kPlain && n.num == "")
}

// step moves the cursor d stops and reports whether it moved. When it did
// not (no stop in that direction), j/k scroll the page instead.
func (pv *view) step(d int) bool {
	keys := pv.navKeys()
	if len(keys) == 0 {
		pv.cur = ""
		return false
	}
	i := index(keys, pv.cur)
	if i < 0 {
		pv.cur, pv.follow = keys[0], true
		return true
	}
	j := min(max(i+d, 0), len(keys)-1)
	if j == i {
		return false
	}
	pv.cur, pv.follow = keys[j], true
	return true
}

func index(xs []string, x string) int {
	for i, y := range xs {
		if y == x {
			return i
		}
	}
	return -1
}

func (a *app) canOpen(pv *view, n *node) bool {
	if n.kind == kLink || n.kind == kChoice {
		return false
	}
	if n.kind == kDetails || n.kind == kQuestion || len(n.body) > 0 || n.reason != "" || pv.cut[n.key] {
		return true
	}
	return n.id != "" && len(a.state.on(n.id)) > 0
}

func (a *app) enter(pv *view) {
	n := pv.nodeAt(pv.cur)
	switch {
	case n == nil:
	case a.canOpen(pv, n):
		pv.open[pv.cur] = !pv.open[pv.cur]
		a.markViewed(n)
	}
}

// ---- the editor ----

func (a *app) handleInput(k key) {
	e, w := &a.in.ed, a.editWidth()
	switch k.name {
	case "esc":
		a.in = nil
	case "left", "ctrl+b":
		e.Left()
	case "right", "ctrl+f":
		e.Right()
	case "up", "ctrl+p":
		e.Up(w)
	case "down", "ctrl+n":
		e.Down(w)
	case "home", "ctrl+a":
		e.Home()
	case "end", "ctrl+e":
		e.End()
	case "alt+left", "alt+b":
		e.WordLeft()
	case "alt+right", "alt+f":
		e.WordRight()
	case "backspace":
		e.Backspace()
	case "delete", "ctrl+d":
		e.Delete()
	case "ctrl+w", "alt+backspace":
		e.KillWordLeft()
	case "ctrl+k":
		e.KillToEnd()
	case "ctrl+u":
		e.KillToStart()
	case "pgup", "pgdn":
		// The report above still scrolls while writing.
		pv, d := a.top(), max(a.t.h/3, 1)
		if k.name == "pgup" {
			pv.top = max(pv.top-d, 0)
		} else {
			pv.top += d
		}
	case "ctrl+g":
		a.external()
	case "newline":
		e.Insert('\n')
	case "enter":
		a.write(e.String())
	case "ctrl+s":
		// Nothing: sending is s, once written.
	case "ctrl+v":
		a.pasteImage()
	case "paste-start":
		a.in.paste = e.pos
	case "paste-end":
		if start := a.in.paste; start >= 0 {
			a.in.paste = -1
			a.pastedImage(start)
		}
	case "":
		if k.r != 0 {
			e.Insert(k.r)
		}
	}
}

// editWidth is the width the editor's text is laid out in.
func (a *app) editWidth() int { return a.textWidth() - markW }

// write records what is in the editor as a thought and closes the editor
// (empty text closes it without recording). It reports false when the
// thought could not be recorded; the editor then stays open.
func (a *app) write(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		a.in = nil // to take a thought back, x
		return true
	}
	e := Event{Op: "thought", ID: a.in.id, Text: text, Title: a.in.title}
	if n := a.in.edit; n != 0 {
		if t := a.thoughtN(n); t == nil || strings.TrimSpace(t.Text) == text {
			a.in = nil // gone, or not changed: nothing to record
			return true
		}
		e = Event{Op: "edit", Thought: n, Text: text}
	}
	if _, err := a.st.Append(e); err != nil {
		a.msg = "記録できませんでした: " + err.Error()
		return false
	}
	a.in = nil
	a.loadEvents()
	return true
}

// thoughtN is the thought numbered n on the board, or nil.
func (a *app) thoughtN(n int) *thought {
	for _, t := range a.state.thoughts {
		if t.N == n {
			return t
		}
	}
	return nil
}

// rewrite opens the editor on thought n's text (e in the pane or the list).
func (a *app) rewrite(n int) {
	t := a.thoughtN(n)
	if t == nil || !a.canWrite() {
		return
	}
	a.in = &input{edit: n, label: fmt.Sprintf("コメント %d を直す", n), ed: newEdit(strings.TrimSpace(t.Text)), paste: -1}
}

// wake types a line into the AI's terminal telling it that the board has
// something it has not read. What was sent is not marked as delivered: that
// happens when the AI runs 'changes'.
func (a *app) wake() {
	if !a.canWrite() {
		return
	}
	a.loadEvents()
	n := a.unsent
	if n == 0 {
		a.msg = "未送信のものはありません"
		return
	}
	// The mark comes first: what is above it is what the AI gets from
	// 'changes', whether or not it can be woken now.
	if _, err := a.st.Append(Event{Op: "send"}); err != nil {
		a.msg = "送れませんでした: " + err.Error()
		return
	}
	a.loadEvents()
	// A 'pitboard wait' waiting for this send wakes the AI by ending: no
	// terminal is typed into.
	if a.st.Waiter() != 0 {
		a.msg = "送りました"
		return
	}
	// Waking may take seconds: say so, and drop the keys pressed meanwhile
	// (they were pressed at a screen that said nothing yet, and would wipe
	// out the result).
	a.msg = "送っています…"
	a.render()
	err := deliver(a.st.session, wakeMessage(a.st, n))
	a.dropBefore = time.Now()
	if errors.Is(err, errNoTarget) {
		a.msg, a.msgLines = "送りました。ただ AI が待っていないので、チャットで一言伝えてください。\n"+err.Error(), 2
		return
	}
	if err != nil {
		a.msg, a.msgLines = "送りました。ただ AI を起こせなかったので、チャットで一言伝えてください。\n"+err.Error(), 2
		return
	}
	a.msg = "AI に送りました"
}

const scissors = "# ------------------------ >8 ------------------------"

func editorCommand() []string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if f := strings.Fields(os.Getenv(v)); len(f) > 0 {
			return f
		}
	}
	return []string{"vim"}
}

// external continues the text in the person's editor. What is above the
// scissors line is sent; leaving it empty sends nothing.
func (a *app) external() {
	f, err := os.CreateTemp("", "pitboard-*.md")
	if err != nil {
		a.msg = err.Error()
		return
	}
	defer os.Remove(f.Name())
	fmt.Fprintf(f, "%s\n\n%s\n# %s: この線より上を板に置きます。空のまま閉じると何もしません。\n", a.in.ed.String(), scissors, a.in.label)
	f.Close()
	cmd := editorCommand()
	if err := a.t.run(exec.Command(cmd[0], append(cmd[1:], f.Name())...)); err != nil {
		a.msg = cmd[0] + ": " + err.Error()
		return
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		a.msg = err.Error()
		return
	}
	text, _, _ := strings.Cut(string(b), scissors)
	if strings.TrimSpace(text) == "" {
		a.msg = "空だったので書きませんでした"
		a.in = nil
		return
	}
	a.write(text)
}

// ---- the thoughts list ----

// listed is the thoughts, newest first.
func (a *app) listed() []*thought {
	ts := a.state.thoughts
	out := make([]*thought, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		out = append(out, ts[i])
	}
	return out
}

func (a *app) handleList(k key) (quit bool) {
	ts := a.listed()
	i := 0
	for j, t := range ts {
		if t.N == a.lcur {
			i = j
		}
	}
	switch {
	case k.name == "tab" || k.r == 'q' || k.name == "esc":
		a.list = false
	case k.r == 'j' || k.name == "down":
		if i+1 < len(ts) {
			a.lcur, a.lfollow = ts[i+1].N, true
		} else {
			a.ltop++
		}
	case k.r == 'k' || k.name == "up":
		if i > 0 && len(ts) > 0 {
			a.lcur, a.lfollow = ts[i-1].N, true
		} else {
			a.ltop = max(a.ltop-1, 0)
		}
	case k.name == "pgup":
		a.ltop = max(a.ltop-max(a.t.h/2, 1), 0)
	case k.name == "pgdn":
		a.ltop += max(a.t.h/2, 1)
	case k.name == "enter" && len(ts) > 0 && a.lcurShown:
		a.lopen[ts[i].N] = !a.lopen[ts[i].N]
	case k.r == 'x' && len(ts) > 0 && a.lcurShown:
		a.retract(ts, ts[i].N, func(n int) { a.lcur, a.lfollow = n, true })
	case k.r == 'e' && len(ts) > 0 && a.lcurShown:
		a.rewrite(ts[i].N)
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "コメントを書く")
	case k.r == 's':
		a.wake()
	}
	return false
}

// handlePane is the keys while the cursor is in the thoughts pane: j k move
// (k on the first goes back to the report), enter opens a thought. Nothing
// is recorded on a thought; m s tab q work as on the report.
func (a *app) handlePane(k key) (quit bool) {
	ts := a.paneOrder()
	i := -1
	for j, t := range ts {
		if t.N == a.pcur {
			i = j
		}
	}
	if i < 0 {
		if len(ts) == 0 {
			a.inPane = false
			return false
		}
		i, a.pcur = 0, ts[0].N
	}
	switch {
	case k.r == 'q':
		return true
	case k.r == ' ':
		a.inPane, a.ptop = false, 0
		a.jump(a.top())
	case k.r == 'A':
		a.viewAll(a.top())
	case k.r == 'j' || k.name == "down":
		if i+1 < len(ts) {
			a.pcur = ts[i+1].N
		}
	case k.r == 'k' || k.name == "up":
		if i == 0 {
			// Back to the report's last stop: the person pressed k to get
			// there, so it works at once.
			a.inPane, a.ptop = false, 0
			a.top().follow = true
			break
		}
		a.pcur = ts[i-1].N
	case k.name == "enter":
		if a.popen == nil {
			a.popen = map[int]bool{}
		}
		a.popen[a.pcur] = !a.popen[a.pcur]
	case k.r == 'e':
		a.rewrite(a.pcur)
	case k.r == 'x':
		a.retract(ts, a.pcur, func(n int) { a.pcur = n })
		if len(a.paneOrder()) == 0 {
			a.inPane, a.ptop = false, 0
			a.top().follow = true
		}
	case k.name == "tab":
		a.list, a.lfollow = true, true
	case k.r == 'm' && a.canWrite():
		a.in = newInput("", "", "コメントを書く")
	case k.r == 's':
		a.wake()
	}
	return false
}

// retract takes thought n back (x on a thought in the pane or the list).
// The cursor goes to the next thought in order (or the one before), which
// then waits settleDelay before x works on it, so x pressed twice does not
// take back a thought the person has not seen. u as the very next key
// undoes it.
func (a *app) retract(order []*thought, n int, move func(n int)) {
	if !a.canWrite() || a.tsettling(n) {
		return
	}
	if _, err := a.st.Append(Event{Op: "retract", Thought: n}); err != nil {
		a.msg = "取り消せませんでした: " + err.Error()
		return
	}
	a.loadEvents()
	for j, t := range order {
		if t.N != n {
			continue
		}
		next := -1
		switch {
		case j+1 < len(order):
			next = order[j+1].N
		case j > 0:
			next = order[j-1].N
		}
		if next >= 0 {
			move(next)
			a.tauto, a.tautoAt = next, time.Time{}
		}
	}
	a.undo = n
	a.msg = "取り消しました (u で元に戻す)"
}

// unretract undoes retract (u right after x) and puts the cursor back on
// the thought.
func (a *app) unretract(n int) {
	if _, err := a.st.Append(Event{Op: "unretract", Thought: n}); err != nil {
		a.msg = "元に戻せませんでした: " + err.Error()
		return
	}
	a.loadEvents()
	if a.list {
		a.lcur, a.lfollow = n, true
	} else {
		a.inPane, a.pcur = true, n
	}
	a.tauto = 0
	a.msg = "元に戻しました"
}

// tsettling says whether the thought under the cursor was put there by the
// viewer (after x) and has not been on screen for settleDelay yet.
func (a *app) tsettling(n int) bool {
	return a.tauto != 0 && a.tauto == n && (a.tautoAt.IsZero() || clock().Sub(a.tautoAt) < settleDelay)
}

// canWrite says whether thoughts can be recorded: the session must exist
// (the viewer never creates one).
func (a *app) canWrite() bool { return a.st != nil && !a.noSession }

// ---- rendering ----

type row struct {
	line  string
	key   string // set on the first line of a stop
	isCur bool
}

func (a *app) textWidth() int {
	return max(min(maxText, a.t.w-2*margin), 10)
}

func (a *app) draw() {
	var rows []row
	var cur *int
	var top *int
	var follow *bool
	var shown *bool
	if a.list {
		rows = a.listRows()
		a.top().dwellKey = "" // the report is not on screen
		top, follow, shown = &a.ltop, &a.lfollow, &a.lcurShown
	} else {
		pv := a.top()
		pv.settleCursor()
		if a.help {
			pv.dwellKey = "" // the list of keys covers the line
		} else {
			a.dwell(pv)
		}
		if a.in != nil && a.in.id != "" && a.in.id == pv.cur {
			// While writing about the cursor's line, keep it in view however
			// tall the editor grows.
			pv.follow = true
		}
		rows = a.rows(pv)
		top, follow, shown, cur = &pv.top, &pv.follow, &pv.curShown, &pv.curRow
	}
	if a.help {
		rows = a.helpRows()
		top, follow, shown, cur = &a.htop, new(bool), new(bool), nil
	}
	ci := -1
	for i, r := range rows {
		if r.isCur {
			ci = i
		}
	}
	// The bottom lines say what works on the cursor's line, which depends on
	// whether the cursor is on screen, which depends on how many lines the
	// bottom takes. Lay out with the last guess, then once more if it was
	// wrong.
	top0, follow0 := *top, *follow
	var bottom []string
	for range 2 {
		guess := len(bottom)
		if guess == 0 {
			guess = len(a.bottomLines(len(rows)))
		}
		h := max(a.t.h-guess, 1)
		*top, *follow = top0, follow0
		if ci >= 0 && *follow {
			*follow = false
			if ci < *top+1 {
				*top = max(ci-1, 0)
			}
			if ci >= *top+h-1 {
				*top = ci - h + 2
			}
		}
		*top = max(min(*top, len(rows)-h), 0)
		*shown = ci >= *top && ci < *top+h
		bottom = a.bottomLines(len(rows))
		if len(bottom) == guess {
			break
		}
	}
	if cur != nil {
		*cur = ci
		if pv := a.top(); pv.auto != "" {
			switch {
			case pv.cur != pv.auto:
				pv.auto = "" // the person moved on
			case pv.curShown && pv.autoAt.IsZero():
				pv.autoAt = clock() // the line is on screen from now
			}
		}
	}
	h := max(a.t.h-len(bottom), 1)
	if cur != nil {
		a.top().atEnd = *top+h >= len(rows)
	}
	if a.tauto != 0 {
		on, shown := 0, true
		switch {
		case a.list:
			on, shown = a.lcur, a.lcurShown
		case a.inPane:
			on = a.pcur
		}
		switch {
		case on != a.tauto:
			a.tauto = 0 // the person moved on
		case shown && a.tautoAt.IsZero() && !a.help:
			a.tautoAt = clock() // on screen from now
		}
	}
	lines := make([]string, 0, a.t.h)
	for i := *top; i < len(rows) && len(lines) < h; i++ {
		lines = append(lines, rows[i].line)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if mark := a.sendMark(); !a.help && len(mark) > 0 && len(lines) > 0 {
		// What the person has not sent, and what the AI has not read yet,
		// on the top line however far the page is scrolled.
		lines[0] = a.overlayRight(lines[0], mark)
	}
	var at *[2]int
	if a.in != nil && !a.help {
		// The terminal's own cursor goes where the person writes, so an
		// input method shows what is being composed there.
		if r := h + a.inAt[0]; r < a.t.h {
			at = &[2]int{r + 1, a.inAt[1] + 1}
		}
	}
	a.t.draw(append(lines, bottom...), at)
}

// sendMark is the top right: "送信済 N" (sent, not read by the AI yet;
// faint) and "● 未送信 N" (not sent; yellow).
func (a *app) sendMark() text {
	var t text
	add := func(s, color string) {
		if len(t) > 0 {
			t = append(t, plainText("  ", "")...)
		}
		t = append(t, plainText(s, color)...)
	}
	if w, f := a.counts(a.top()); w > 0 || f > 0 {
		if w > 0 {
			add(fmt.Sprintf("反応待ち %d", w), yellow)
		}
		if f > 0 {
			add(fmt.Sprintf("新着 %d", f), green)
		}
	}
	if a.waiter {
		add("AI 待機中", dim)
	}
	if a.unread > 0 {
		add(fmt.Sprintf("送信済 %d", a.unread), dim)
	}
	if a.unsent > 0 {
		add(fmt.Sprintf("● 未送信 %d", a.unsent), yellow)
	}
	return t
}

// overlayRight puts right at the right end of an already drawn line, where
// compose puts a line's right part. A line too long for both is cut with ….
func (a *app) overlayRight(line string, right text) string {
	end := margin + a.textWidth() // the column right ends at
	avail := end - right.width() - 2
	if w := visibleWidth(line); w <= avail {
		return line + strings.Repeat(" ", max(end-right.width()-w, 2)) + right.String()
	}
	cut := cutVisible(line, max(avail-1, 0))
	return cut + "…" + strings.Repeat(" ", max(end-right.width()-visibleWidth(cut)-1, 2)) + right.String()
}

// visibleWidth is how many cells a drawn line takes (escape sequences take
// none).
func visibleWidth(s string) int {
	w := 0
	forVisible(s, func(_ int, r rune) bool { w += runeWidth(r); return true })
	return w
}

// cutVisible keeps the first w cells of a drawn line, and ends any style
// or hyperlink it was in.
func cutVisible(s string, w int) string {
	n, end := 0, 0
	forVisible(s, func(i int, r rune) bool {
		if n+runeWidth(r) > w {
			return false
		}
		n += runeWidth(r)
		end = i + len(string(r))
		return true
	})
	// Keep the escape sequences right after the last kept rune too (they
	// may close a style), then reset everything.
	return s[:end] + rst + "\x1b]8;;\x1b\\"
}

// forVisible calls f with each rune of s that is drawn (not part of an
// escape sequence) and its byte offset, until f returns false.
func forVisible(s string, f func(i int, r rune) bool) {
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) {
			switch s[i+1] {
			case '[': // CSI: up to a final byte
				j := i + 2
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				i = j + 1
				continue
			case ']': // OSC: up to ESC \ or BEL
				j := i + 2
				for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
					j++
				}
				if j < len(s) && s[j] == 0x1b {
					j++
				}
				i = j + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !f(i, r) {
			return
		}
		i += size
	}
}

// bottomLines is everything under the scrolling part: the editor while
// writing, or the thoughts and the key hints. n is how many rows the
// scrolling part has (the list of keys scrolls only if it does not fit).
func (a *app) bottomLines(n int) []string {
	switch {
	case a.help:
		hs := []hint{{k: "q", d: "閉じる"}}
		if n > a.t.h-1 {
			hs = []hint{{k: "j k ↑↓", d: "スクロール"}, {k: "q", d: "閉じる"}}
		}
		return a.packKeys(hs, nil, false)
	case a.in != nil:
		return a.editorLines()
	}
	hints := a.bottom()
	if a.list {
		return hints
	}
	return append(a.pane(a.t.h-len(hints)), hints...)
}

// helpRows is the list of every key.
func (a *app) helpRows() []row {
	rows := []row{{}, {line: a.compose(false, 0, nil, plainText("キー", bold+blue), nil)}, {}}
	for _, k := range [][2]string{
		{"j k ↑↓", "上下に動く (ctrl+n / ctrl+p も)。止まる行がその先になければ 1 行スクロール。カーソルの行が画面の外にあれば、まずそこまで戻す"},
		{"pgup pgdn", "半画面ずつスクロール"},
		{"", "報告の終わりからさらに j で、報告の下のコメントへ (欄が広がる)。コメントの 1 件目で k で報告へ"},
		{"", "日本語入力のままでも、↑↓ ctrl+n ctrl+p と全角の英字 (ｊ ｙ など) が効く"},
		{"space", "次の反応待ち (黄の ●) へ。無ければ次の新着 (緑の ●) へ。折りたたみは開く"},
		{"A", "いま報告にある行をすべて見たことにする (緑の ● を消す)"},
		{"enter", "開く・閉じる (コメントの欄でも)"},
		{"x (コメント)", "コメントの欄と一覧では、そのコメントを取り消す。直後の u で元に戻す"},
		{"e (コメント)", "コメントの欄と一覧で、そのコメントを直す (enter で置き換える。esc でやめる)"},
		{"y", "よし (項目・リンク)"},
		{"n", "だめ (項目・リンク)"},
		{"a b …", "問いに答える (選択肢のキー。問いを開いているときだけ。閉じていれば開く)"},
		{"c", "この行にコメントを書く"},
		{"m", "コメントを書く (どこにカーソルがあっても)"},
		{"s", "AI に送る: 未送信のものをまとめて送る (AI が pitboard wait で待っていればそれが起こす。いなければ AI の端末に 1 行打ち込んで起こす)"},
		{"x", "この行を隠す (隠した行の上では表示に戻す)"},
		{"H", "隠した行を表示する / しない"},
		{"tab", "コメントの一覧 / 報告へ"},
		{"?", "このキーの一覧"},
		{"q", "終了 (一覧からは報告へ)"},
		{"", ""},
		{"書いているとき", ""},
		{"enter", "板に置く (s で送るまで AI には届かない)"},
		{"shift+enter", "改行 (対応していない端末では ctrl+j。alt+enter も)"},
		{"← → ctrl+b f", "1 字左 / 右"},
		{"↑ ↓ ctrl+p n", "上 / 下の行へ (折り返した見た目の行で)"},
		{"ctrl+a e", "行頭 / 行末 (home / end も)"},
		{"alt+b f", "語単位で左 / 右 (alt+← → も)"},
		{"backspace", "左の 1 字を消す (ctrl+h も)"},
		{"ctrl+d", "右の 1 字を消す (delete も)"},
		{"ctrl+w", "左の 1 語を消す (alt+backspace も)"},
		{"ctrl+k / u", "カーソルから行末まで / 行頭からカーソルまで消す"},
		{"pgup pgdn", "上の報告をスクロール"},
		{"ctrl+v", "クリップボードの画像を貼る ([Image #N] が入る。画像ファイルのパスを貼っても同じ)"},
		{"ctrl+g", "$VISUAL / $EDITOR / vim で続きを書く"},
		{"esc", "やめる"},
	} {
		if k[0] == "" && k[1] == "" {
			rows = append(rows, row{})
			continue
		}
		// The description wraps in its own column, to the right of the key.
		const col = 14
		label := plainText(k[0], bold)
		label = append(label, plainText(strings.Repeat(" ", max(col-label.width(), 1)), "")...)
		for i, l := range plainText(k[1], dim).wrap(a.room(col, nil)) {
			if i == 0 {
				rows = append(rows, row{line: a.compose(false, 0, nil, append(label, l...), nil)})
			} else {
				rows = append(rows, row{line: a.compose(false, col, nil, l, nil)})
			}
		}
	}
	return rows
}

// handleHelp scrolls the list of keys; q, esc and ? close it.
func (a *app) handleHelp(k key) {
	switch {
	case k.r == 'q' || k.r == '?' || k.name == "esc":
		a.help, a.htop = false, 0
	case k.r == 'j' || k.name == "down":
		a.htop++ // draw keeps it within the list
	case k.r == 'k' || k.name == "up":
		a.htop = max(a.htop-1, 0)
	}
}

// hint is one key in the hints at the bottom. A hint with a short form is
// never left out: when the line is narrow, the short form is shown instead.
type hint struct {
	k, d  string
	alt   string // a shorter label tried before hints are left out
	short string
	rank  int // when it is left out on a narrow line (see rankX)
}

func keyHints(hs []hint) text {
	var t text
	for i, h := range hs {
		if i > 0 {
			t = append(t, plainText("  ", "")...)
		}
		t = append(t, plainText(h.k, bold)...)
		t = append(t, plainText(" "+h.d, dim)...)
	}
	return t
}

// bottom is the bottom line(s): a message, or the keys that work on this
// line, and on the right how many lines are hidden.
func (a *app) bottom() []string {
	var right text
	switch {
	case a.st == nil || a.noSession:
		// Waiting for a board: say where the viewer is looking.
		// A long path keeps its end (the directory's own name).
		p, room := homeTilde(a.base), a.textWidth()-markW-len("? キー")-4
		if width(p) > room {
			p = "…" + tail(p, max(room-1, 4))
		}
		right = plainText(p, dim)
	case !a.list:
		right = a.hiddenCount()
	}
	// A message is one line, cut with … when long (the hints are always
	// one line too).
	say := func(msg string) []string {
		one := func(s string) text { return plainText(strings.Join(strings.Fields(s), " "), magenta) }
		t := one(msg)
		r := right
		if t.width() > a.room(0, r) {
			r = nil // the message before the count of hidden lines
		}
		head, rest, two := strings.Cut(msg, "\n")
		if a.msgLines < 2 || !two || t.width() <= a.room(0, r) {
			return []string{a.compose(false, 0, nil, t, r)}
		}
		// Two lines: what happened, and why (with how to fix it); each is
		// cut with … if need be.
		return []string{a.compose(false, 0, nil, one(head), nil), a.compose(false, 0, nil, one(rest), nil)}
	}
	switch {
	case a.msg != "":
		return say(a.msg)
	case !a.list && a.top().lock:
		return say(msgMoved)
	}
	var keys []hint
	if a.list {
		if t := a.listCur(); t != nil && a.lcurShown {
			if a.lopen[t.N] {
				keys = append(keys, hint{k: "enter", d: "閉じる", rank: rankEnter})
			} else {
				keys = append(keys, hint{k: "enter", d: "開く", rank: rankEnter})
			}
		}
		if len(keys) > 0 && a.canWrite() {
			keys = append(keys, hint{k: "e", d: "直す", rank: rankN}, hint{k: "x", d: "取り消す", rank: rankX})
		}
		if len(keys) == 0 {
			keys = append(keys, hint{k: "tab", d: "報告へ", rank: rankX})
		}
		return a.oneLine(keys, nil)
	}
	if a.inPane {
		if a.popen[a.pcur] {
			keys = append(keys, hint{k: "enter", d: "閉じる", rank: rankEnter})
		} else {
			keys = append(keys, hint{k: "enter", d: "開く", rank: rankEnter})
		}
		if a.canWrite() {
			keys = append(keys, hint{k: "e", d: "直す", rank: rankN}, hint{k: "x", d: "取り消す", rank: rankX})
		}
		return a.oneLine(keys, nil)
	}
	pv := a.top()
	n := pv.nodeAt(pv.cur)
	if n == nil || !pv.curShown {
		if pv.err == "" && !pv.missing {
			keys = append(keys, hint{k: "j k ↑↓", d: "スクロール", rank: rankEnter})
		}
		return a.oneLine(keys, right)
	}
	var enter, answer, yn, write, hide []hint
	switch {
	case a.canOpen(pv, n) && pv.open[pv.cur]:
		enter = []hint{{k: "enter", d: "閉じる", rank: rankEnter}}
	case a.canOpen(pv, n):
		enter = []hint{{k: "enter", d: "開く", rank: rankEnter}}
	}
	ask := n.kind == kQuestion
	askOpen := ask && pv.open[pv.cur]
	t := a.target(pv)
	if t != nil {
		y, choices, thought := keysFor(t)
		if !askOpen {
			choices = nil // the options of a closed Ask are not on screen
		}
		if pv.placed {
			// Placed by the viewer, not by the person: say how to react.
			switch {
			case askOpen:
				answer = []hint{{k: "j k ↑↓", d: "動かすと答えられる", short: "→ 答える"}}
			case y:
				answer = []hint{{k: "j k ↑↓", d: "動かすと y n が効く", short: "→ y n"}}
			}
			y, choices = false, nil
		}
		if y {
			yn = []hint{{k: "y", d: "よし", rank: rankY}, {k: "n", d: "だめ", rank: rankN}}
		}
		if len(choices) > 0 {
			answer = []hint{{k: strings.Join(choices, " "), d: "答える", rank: rankY}}
		}
		if thought {
			write = []hint{{k: "c", d: "行にコメント", alt: "行に書く", rank: rankC}}
		}
	}
	if pv.placed && len(answer) == 0 && hideKey(n) != "" && !pv.lock && a.canWrite() {
		// A plain line, a Fold or a closed Ask the viewer put the cursor on.
		answer = []hint{{k: "j k ↑↓", d: "動かすと x が効く", short: "→ x"}}
	}
	if a.hideTarget(pv) != nil {
		if pv.isHidden(n) {
			hide = []hint{{k: "x", d: "表示に戻す", rank: rankX}}
		} else {
			hide = []hint{{k: "x", d: "隠す", rank: rankX}}
		}
	}
	// The keys for this line, in the order shown; an open Ask leads with
	// its options.
	if askOpen {
		keys = concat(answer, enter, write, hide)
	} else {
		keys = concat(enter, answer, yn, write, hide)
	}
	// m tab q H work everywhere: the list of keys (?) tells them; m is also
	// shown, since writing works wherever the cursor is.
	return a.oneLine(keys, right)
}

func concat(hs ...[]hint) []hint {
	var out []hint
	for _, h := range hs {
		out = append(out, h...)
	}
	return out
}

// The order in which hints are left out when the line is narrow: lowest
// first. A hint with rank 0 (one with a short form, m, ?) is kept.
const (
	rankX = iota + 1
	rankEnter
	rankN
	rankY
	rankC
)

// oneLine is the hints: always one line, the keys for this line, then
// "m 書く" and "? キー". When the line is narrow, right goes first, then
// keys by rank (x, enter, n, y, c); "m 書く", "? キー" and a hint with a
// short form stay to the end.
func (a *app) oneLine(keys []hint, right text) []string {
	q := hint{k: "?", d: "キー"}
	var tail []hint
	if a.canWrite() && a.unsent > 0 {
		tail = append(tail, hint{k: "s", d: "送る"}) // kept to the end, as m and ?
	}
	if a.canWrite() {
		tail = append(tail, hint{k: "m", d: "書く"})
	}
	tail = append(tail, q)
	if !a.list && !a.inPane {
		if w, f := a.counts(a.top()); w > 0 || f > 0 {
			// Where to go next: before "m 書く", dropped first when narrow.
			keys = append(append([]hint{}, keys...), hint{k: "space", d: "次へ", rank: rankX})
		}
	}
	shorten := func(ks []hint) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.short != "" {
				k.d = k.short
			}
			out[i] = k
		}
		return out
	}
	alt := func(ks []hint) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.alt != "" {
				k.d = k.alt
			}
			out[i] = k
		}
		return out
	}
	fits := func(ks []hint, r text) bool { return keyHints(ks).width() <= a.room(0, r) }
	for lvl := 0; lvl <= rankC; lvl++ {
		var ks []hint
		for _, k := range keys {
			if k.short != "" || k.rank == 0 || k.rank > lvl {
				ks = append(ks, k)
			}
		}
		for _, cand := range [][]hint{ks, alt(ks), shorten(alt(ks))} {
			cand = append(cand, tail...)
			for _, r := range []text{right, nil} { // right goes before any key
				if fits(cand, r) {
					return []string{a.compose(false, 0, nil, keyHints(cand), r)}
				}
			}
		}
	}
	// Too narrow for all: what the person must know to react (the short
	// form of a kept hint) comes first, then "? キー".
	var must []hint
	for _, k := range shorten(keys) {
		if k.short != "" {
			must = append(must, k)
		}
	}
	for _, cand := range [][]hint{append(append([]hint{}, must...), q), must} {
		if len(must) > 0 && fits(cand, nil) {
			return []string{a.compose(false, 0, nil, keyHints(cand), nil)}
		}
	}
	return []string{a.compose(false, 0, nil, keyHints([]hint{q}), nil)}
}

// packKeys lays the key hints out on one line beside right, or on two. Hints
// are never cut: when even two lines are too few, the keys that work
// everywhere (they come last) are left out first, the keys for this line
// stay, and "? キー" at the end shows them all.
func (a *app) packKeys(keys []hint, right text, more bool) []string {
	// lay fills the first line, then the second, in order. right goes on
	// line rl (1 or 2).
	lay := func(ks []hint, rl int) ([]string, bool) {
		var rights [2]text
		rights[rl-1] = right
		var ls [2][]hint
		li := 0
		for _, k := range ks {
			for li < 2 && keyHints(append(ls[li], k)).width() > a.room(0, rights[li]) {
				li++
			}
			if li == 2 {
				return nil, false
			}
			ls[li] = append(ls[li], k)
		}
		if len(ls[0]) == 0 && len(ks) > 0 {
			return nil, false // the first line would hold nothing but right
		}
		out := []string{a.compose(false, 0, nil, keyHints(ls[0]), rights[0])}
		if len(ls[1]) > 0 || len(rights[1]) > 0 {
			out = append(out, a.compose(false, 0, nil, keyHints(ls[1]), rights[1]))
		}
		return out, true
	}
	try := func(ks []hint) ([]string, bool) {
		if out, ok := lay(ks, 1); ok {
			return out, true
		}
		if len(right) > 0 {
			return lay(ks, 2)
		}
		return nil, false
	}
	shorten := func(ks []hint) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.short != "" {
				k.d = k.short
			}
			out[i] = k
		}
		return out
	}
	if out, ok := try(keys); ok {
		return out
	}
	if more {
		// Keys are dropped from the end, so the ones for this line go last;
		// a hint with a short form is never dropped.
		for n := len(keys); n >= 0; n-- {
			var ks []hint
			for i, k := range keys {
				if i < n || k.short != "" {
					ks = append(ks, k)
				}
			}
			ks = append(ks, hint{k: "?", d: "キー"})
			if out, ok := try(ks); ok {
				return out
			}
			if out, ok := try(shorten(ks)); ok {
				return out
			}
		}
	}
	// Too narrow for anything: cut, as a last resort.
	return []string{a.compose(false, 0, nil, keyHints(shorten(keys)), right)}
}

// ---- the thoughts at the bottom of the report ----

// paneOrder is the thoughts the pane shows first: those still waiting for a
// destination (newest first), then those given one (most recently first).
func (a *app) paneOrder() []*thought {
	var open, done []*thought
	for i := len(a.state.thoughts) - 1; i >= 0; i-- {
		t := a.state.thoughts[i]
		if !t.finished() {
			open = append(open, t)
		} else {
			done = append(done, t)
		}
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].settledN > done[j].settledN })
	return append(open, done...)
}

const (
	paneMax     = 3  // thoughts shown in the pane
	paneMinH    = 20 // below this screen height, only the pane's heading
	minBodyRows = 6  // the report keeps at least this many rows
)

// pane is the thoughts, always in view under the report: a heading with
// counts, and the first few. avail is the height left above the hints.
func (a *app) pane(avail int) []string {
	order := a.paneOrder()
	if len(order) == 0 {
		return nil
	}
	if a.inPane {
		return a.openPane(order)
	}
	n := min(paneMax, len(order))
	if a.t.h < paneMinH {
		n = 0
	}
	for n > 0 && avail-(n+3) < minBodyRows { // + heading and the blank above and below
		n--
	}
	// Quiet: how many are not sent is said at the top right (未送信), and
	// how to send them in the hints (s 送る).
	head := a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)
	if n == 0 {
		return []string{head}
	}
	out := []string{"", head}
	w := a.textWidth()
	for _, t := range order[:n] {
		word, color := a.state.statusOf(t)
		base := ""
		if t.finished() {
			base = dim
		}
		body := withImages(strings.Join(strings.Fields(t.Text), " "), base).capped(2 * w)
		out = append(out, a.compose(false, 2, nil, body, plainText(word, color)))
	}
	return append(out, "")
}

// openPane is the pane while the cursor is in it: up to half the screen,
// every thought in the pane's order, scrolled to keep the cursor's in view.
func (a *app) openPane(order []*thought) []string {
	head := a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)
	var rows []row
	for _, t := range order {
		word, color := a.state.statusOf(t)
		base := ""
		if t.finished() {
			base = dim
		}
		sel := t.N == a.pcur
		mark := plainText("▸", "")
		if a.popen[t.N] {
			mark = plainText("▾", "")
		}
		body := withImages(strings.Join(strings.Fields(t.Text), " "), base).capped(2 * a.textWidth())
		r := a.line(sel, 0, mark, body, plainText(word, color), false)
		r[0].key = fmt.Sprint(t.N) // the first row of a thought
		rows = append(rows, r...)
		if a.popen[t.N] {
			rows = append(rows, a.thoughtDetails(t, markW)...)
		}
	}
	room := max(a.t.h/2-3, 1) // less the heading and the blank above and below
	// Keep the cursor's thought in view, and its opened lines as far as
	// they fit.
	ci, end := 0, len(rows)-1
	for i, r := range rows {
		if r.isCur {
			ci, end = i, len(rows)-1
		} else if r.key != "" && i > ci && end == len(rows)-1 && rows[ci].isCur {
			end = i - 1
		}
	}
	end = min(end, ci+room-1)
	if ci < a.ptop {
		a.ptop = ci
	}
	if end >= a.ptop+room {
		a.ptop = min(end-room+1, ci)
	}
	a.ptop = max(min(a.ptop, len(rows)-room), 0)
	out := []string{"", head}
	for i := a.ptop; i < len(rows) && i < a.ptop+room; i++ {
		out = append(out, rows[i].line)
	}
	return append(out, "")
}

// headed is a line with a faint head word ("理由: ", "対応: ") and the
// rest in the normal color.
func headed(head, s string) text {
	return append(plainText(head, dim), plainText(s, "")...)
}

// thoughtDetails is what an opened thought shows: the line it was written
// on (and its title then), the whole text, and where it went.
func (a *app) thoughtDetails(t *thought, ind int) []row {
	var rows []row
	var nodes []*node
	if a.rep != nil {
		nodes = a.rep.nodes
	}
	if t.ID != "" {
		about := "(報告にもうない行)"
		if n := byID(nodes, t.ID); n != nil {
			about = n.title()
		}
		if t.Title != "" && t.Title != about {
			about += " (書いたときは: " + t.Title + ")"
		}
		rows = append(rows, a.para(ind, headed("行: ", about))...)
	}
	for _, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
		rows = append(rows, a.para(ind, withImages(l, ""))...)
	}
	switch {
	case t.sstate != "":
		rows = append(rows, a.para(ind, headed(settleHead(t.sstate), t.settled))...)
	case t.lastN > a.state.sent:
		rows = append(rows, a.para(ind, plainText("まだ送っていません (s で送る)", dim))...)
	case t.lastN > a.state.seen:
		rows = append(rows, a.para(ind, plainText("送りました。AI はまだ読んでいません", dim))...)
	default:
		rows = append(rows, a.para(ind, plainText("AI は読みました。対応はまだです", dim))...)
	}
	return rows
}

// settleHead is the head word of what the AI wrote on a comment.
func settleHead(state string) string {
	switch state {
	case "doing":
		return "対応中: "
	case "hold":
		return "保留: "
	}
	return "対応: "
}

// homeTilde writes a path under the home directory with ~.
func homeTilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return p
}

// hiddenCount is what the hints say on the right about hidden lines.
func (a *app) hiddenCount() text {
	pv := a.top()
	if pv == nil {
		return nil
	}
	c := pv.hiddenCount()
	switch {
	case c == 0:
		return nil
	case pv.showHidden:
		return plainText(fmt.Sprintf("隠した行 %d (表示中)", c), dim)
	}
	return plainText(fmt.Sprintf("隠した行 %d", c), dim)
}

// editorLines is the editor at the bottom: a title saying what is being
// written, then the text, growing up to a third of the screen.
func (a *app) editorLines() []string {
	e, w := &a.in.ed, a.editWidth()
	rs := e.rows(w)
	crow, ccol := e.Cursor(w)
	// Taller than the editor: scrolled to keep the cursor's row in view.
	limit := max(a.t.h/3, 3)
	top := &a.in.etop
	if crow < *top {
		*top = crow
	}
	if crow >= *top+limit {
		*top = crow - limit + 1
	}
	*top = max(min(*top, len(rs)-limit), 0)
	hints := a.editorHints()
	// If the line being written about changed or went away meanwhile, say so
	// (sending still works: what was written is never lost).
	var changed text
	if a.in.id != "" {
		if n := byID(a.rep.nodes, a.in.id); n == nil || n.title() != a.in.title {
			changed = plainText("この行は書き換わりました", magenta)
		}
	}
	out := []string{""}
	for i, l := range plainText(a.in.label, cyan+bold).wrap(a.room(0, changed)) {
		if i == 0 {
			out = append(out, a.compose(false, 0, nil, l, changed))
		} else {
			out = append(out, a.compose(false, 0, nil, l, nil))
		}
	}
	for i := *top; i < len(rs) && i < *top+limit; i++ {
		if i == crow {
			// The input position, past the gutter and the mark field (the
			// terminal's cursor is put there).
			a.inAt = [2]int{len(out), margin + markW + ccol}
		}
		l := withImages(string(e.text[rs[i].start:rs[i].end]), "")
		out = append(out, a.compose(false, 0, nil, l, nil))
	}
	return append(out, hints...)
}

// editorHints is the editor's one line of hints. When narrow, they are
// left out from the end: ctrl+g, then ctrl+j; how to write, send and stop
// always stay.
func (a *app) editorHints() []string {
	nl := "ctrl+j"
	if a.kbd {
		nl = "shift+enter" // the terminal tells it from enter
	}
	all := []hint{{k: "enter", d: "置く"}, {k: "esc", d: "やめる"}, {k: nl, d: "改行", rank: 2}, {k: "ctrl+g", d: "エディタ", rank: 1}}
	var ks []hint
	for lvl := 0; lvl <= 2; lvl++ {
		ks = ks[:0]
		for _, k := range all {
			if k.rank == 0 || k.rank > lvl {
				ks = append(ks, k)
			}
		}
		if keyHints(ks).width() <= a.room(0, nil) {
			break
		}
	}
	return []string{a.compose(false, 0, nil, keyHints(ks), nil)}
}

// compose lays out one screen line: the left gutter (holding the cursor
// mark), indent, the 2-cell mark field, the content, and an optional
// right-aligned part (status, reaction, thoughts).
func (a *app) compose(selected bool, indent int, mark, body, right text) string {
	w := a.textWidth()
	gutter := strings.Repeat(" ", margin)
	if selected {
		gutter = ">" + strings.Repeat(" ", margin-1)
	}
	if len(right) > 0 {
		right = right.trunc(max(w-indent-markW-8, 4))
	}
	body = body.trunc(a.room(indent, right))
	s := gutter + strings.Repeat(" ", indent) + mark.String() + strings.Repeat(" ", max(markW-mark.width(), 0)) + body.String()
	if len(right) > 0 {
		gap := w - indent - markW - body.width() - right.width()
		s += strings.Repeat(" ", max(gap, 1)) + right.String()
	}
	return s
}

// room is the width left for the content of a line.
func (a *app) room(indent int, right text) int {
	r := a.textWidth() - indent - markW
	if len(right) > 0 {
		r -= right.width() + 2
	}
	return max(r, 4)
}

func (a *app) para(indent int, t text) []row {
	var out []row
	for _, ln := range t.wrap(a.room(indent, nil)) {
		out = append(out, row{line: a.compose(false, indent, nil, ln, nil)})
	}
	return out
}

// line draws one part: its first row carries the mark and the right part;
// when wrap is set, the rest of the text follows on rows indented to it.
func (a *app) line(sel bool, indent int, mark, body, right text, wrap bool) []row {
	if !wrap {
		return []row{{line: a.compose(sel, indent, mark, body, right), isCur: sel}}
	}
	lns := body.wrap(a.room(indent, right))
	out := []row{{line: a.compose(sel, indent, mark, lns[0], right), isCur: sel}}
	for _, ln := range lns[1:] {
		out = append(out, row{line: a.compose(false, indent, nil, ln, nil)})
	}
	return out
}

func join(parts ...text) text {
	var out text
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, plainText("  ", "")...)
		}
		out = append(out, p...)
	}
	return out
}

// right is what goes on the right end of a line: the state of the
// person's latest input on it (a reaction, or a comment), then the status
// the AI wrote. Colors mean: yellow needs the person (not sent, wait="you"),
// green moves (a doing Item's status), faint waits or is done.
func (a *app) right(pv *view, n *node) text {
	var st text
	switch {
	case n.status == "":
	case n.wait == "you":
		st = plainText(n.status, yellow)
	case n.kind == kDoing:
		st = plainText(n.status, green)
	default:
		st = plainText(n.status, dim)
	}
	if n.id == "" {
		return st
	}
	// The reaction, while the line is what it was reacted to.
	var re text
	e, reacted := a.state.react[n.id]
	reacted = reacted && e.Line == n.sig()
	if reacted {
		switch e.Value {
		case "y":
			re = plainText("y", bold+green)
		case "n":
			re = plainText("n", bold+red)
		default:
			re = plainText(e.Value, bold+cyan)
		}
	}
	// The state of the latest input: the reaction's, or the latest
	// comment's (with how many comments there are, when more than one).
	cs := a.state.on(n.id)
	var last *thought
	for _, t := range cs {
		if last == nil || t.lastN > last.lastN {
			last = t
		}
	}
	var word text
	switch {
	case reacted && (last == nil || e.N > last.lastN):
		w, color := a.state.sentWord(e.N), dim
		if w == "未送信" {
			color = yellow
		}
		word = plainText(w, color)
	case last != nil:
		w, color := a.state.statusOf(last)
		word = plainText(w, color)
		if len(cs) > 1 {
			word = append(plainText(fmt.Sprintf("%d ", len(cs)), dim), word...)
		}
	}
	mine := func(re text) text {
		switch {
		case len(re) == 0:
			return word
		case len(word) == 0:
			return re
		}
		return append(append(append(text{}, re...), plainText(" ", "")...), word...)
	}
	if n.kind == kQuestion && len(re) > 0 && !pv.open[n.key] {
		// A closed question shows the head of the chosen option too, if
		// the line has room for it.
		if ch := n.choice(re.plain()); ch != nil {
			head := append(append(text{}, re...), plainText(" ", "")...)
			head = append(head, plainText(plainOf(ch.text), dim).trunc(answerHead)...)
			ind := n.depth * 2
			if long := join(mine(head), st); inline(n.text, "").width() <= a.room(ind, long) {
				return long
			}
		}
	}
	return join(mine(re), st)
}

// answerHead is how much of the chosen option a closed question shows.
const answerHead = 12

// markOf is the mark at the head of a line. Done items always show ✓;
// otherwise opening wins over the kind.
func markOf(k kind, canOpen, open bool) text {
	switch {
	case k == kLink:
		return plainText("→", blue)
	case k == kDone:
		return plainText("✓", dim)
	case k == kQuestion:
		return plainText("?", yellow) // even when a long question opens
	case canOpen && open:
		return plainText("▾", "")
	case canOpen:
		return plainText("▸", "")
	case k == kDoing:
		return plainText("~", "")
	case k == kTodo:
		return plainText("○", "")
	case k == kPlain:
		return plainText("·", dim)
	}
	return nil
}

func (a *app) rows(pv *view) []row {
	var rows []row
	blank := func() {
		if len(rows) > 0 && rows[len(rows)-1].line != "" {
			rows = append(rows, row{})
		}
	}
	rows = append(rows, row{}) // top padding
	switch {
	case a.st == nil && !isDir(filepath.Join(a.base, ".pitboard")):
		return append(rows, a.para(0, plainText(msgNoBoard, dim))...)
	case a.st == nil:
		return append(rows, a.para(0, plainText(msgNoSession, dim))...)
	case a.noSession:
		return append(rows, a.para(0, plainText(fmt.Sprintf("板 %s はまだありません", a.st.session), dim))...)
	case pv.missing:
		return append(rows, a.para(0, plainText(msgNoReport, dim))...)
	case pv.err != "":
		rows = append(rows, a.para(0, plainText("読めませんでした: "+pv.err, magenta))...)
		blank()
	}
	if pv.readErr != "" {
		rows = append(rows, a.para(0, plainText(pv.readErr, magenta+bold))...)
		blank()
	}
	if len(pv.probs) > 0 {
		rows = append(rows, a.para(0, plainText("報告に書き方の誤りがあります (読めた所までを出しています)", magenta+bold))...)
		for _, pr := range pv.probs {
			rows = append(rows, a.para(0, plainText(fmt.Sprintf("%d 行目: %s", pr.Line, pr.Msg), magenta))...)
		}
		blank()
	}
	pv.cut = map[string]bool{}
	chosen := "" // the answered Option of the Ask being drawn
	var curAsk *node
	pv.walk(func(n *node) {
		sel := n.key != "" && n.key == pv.cur && !a.inPane
		ind := n.depth * 2
		open := pv.open[n.key]
		switch n.kind {
		case kTitle:
			rows = append(rows, a.para(0, inline(n.text, bold))...)
			rows = append(rows, row{})
		case kSection:
			blank()
			rows = append(rows, a.para(0, inline(n.text, bold+blue))...)
		case kPara:
			blank()
			rows = append(rows, a.para(ind, paraText(n, ""))...)
			rows = append(rows, row{})
		case kCode:
			blank()
			rows = append(rows, a.code(n, ind+2)...)
			rows = append(rows, row{})
		case kChoice:
			defer func() {
				// After the last option of an open Ask, a blank line.
				if curAsk != nil && len(curAsk.kids) > 0 && curAsk.kids[len(curAsk.kids)-1] == n {
					blank()
				}
			}()
			label := plainText(n.label+"  ", dim)
			var mark text
			if chosen != "" && n.label == chosen {
				mark = plainText("●", bold+cyan)
			}
			lns := inline(n.text, "").wrap(a.room(ind, nil) - label.width())
			rows = append(rows, row{line: a.compose(false, ind, mark, append(label, lns[0]...), nil)})
			for _, ln := range lns[1:] {
				rows = append(rows, row{line: a.compose(false, ind+label.width(), nil, ln, nil)})
			}
		default: // todo, done, doing, question, plain, link, details
			if (n.kind == kDetails && n.depth == 0) || (n.kind == kPlain && n.gap) {
				blank()
			}
			if n.kind == kQuestion {
				chosen, curAsk = "", n
				if e, ok := a.state.react[n.id]; ok && n.id != "" && e.Line == n.sig() {
					chosen = e.Value
				}
			}
			base := ""
			switch n.kind {
			case kDone:
				base = dim
			case kLink:
				base = blue
			}
			hidden := pv.isHidden(n) // drawn only while H shows hidden lines
			if hidden {
				base = dim
			}
			body, right := inline(n.text, base).capped(8*a.textWidth()), a.right(pv, n)
			nav := pv.navigable(n)
			// A stop is kept to one line and cut with …; a cut stop opens
			// to its full text. Other lines, links and plain lines are
			// wrapped so nothing is lost.
			whole := !nav || n.kind == kLink || n.kind == kPlain
			if !whole && body.width() > a.room(ind, right) {
				pv.cut[n.key] = true
			}
			can := nav && a.canOpen(pv, n)
			if can && open {
				// Open: the title in bold, so it stands apart from what it
				// holds (in the normal weight).
				body = inline(n.text, base+bold).capped(8 * a.textWidth())
			}
			// A line the cursor cannot stop at cannot be opened either, so
			// what it holds is always shown.
			shown := (can && open) || (!nav && n.kind != kDetails)
			wrap := whole || (can && open)
			mark := markOf(n.kind, can, open)
			if hidden {
				mark = plainText("⌀", dim)
			}
			var lns []row
			if n.kind == kPlain {
				if !hidden {
					mark = nil // the list's own mark
				}
				lns = a.listItem(sel, ind, n, base, mark, right)
			} else {
				lns = a.line(sel, ind, mark, body, right, wrap)
			}
			if nav {
				lns[0].key = n.key
				lns[0].line = gutterMark(lns[0].line, a.markOfLine(pv, n))
			}
			rows = append(rows, lns...)
			if n.kind == kLink {
				rows = append(rows, a.linkTarget(n, ind)...)
			}
			if shown {
				// What it holds, 2 past the title's text (4 past its mark),
				// and a blank line after it when opened.
				c := a.contents(pv, n, ind+2)
				rows = append(rows, c...)
				if can && open && len(c) > 0 {
					blank()
				}
			}
		}
	})
	return rows
}

// paraText is a paragraph as drawn: a small heading is bold in the color of
// section headings.
func paraText(n *node, base string) text {
	if n.head {
		return inline(n.text, base+bold+blue)
	}
	return inline(n.text, base)
}

// listMark is the mark of a list line: its number, or · - · by level.
func listMark(n *node) text {
	switch {
	case n.num != "":
		return plainText(n.num+" ", "")
	case n.level == 1:
		return plainText("-", dim)
	}
	return plainText("·", dim)
}

// listItem draws a list line and the lines nested under it. Its text
// wraps to where it starts (past a number wider than the mark field), and
// each level is indented 2 more. mark replaces the list's own mark if set.
func (a *app) listItem(sel bool, ind int, n *node, base string, mark, right text) []row {
	if len(mark) == 0 {
		mark = listMark(n)
	}
	extra := max(mark.width()-markW, 0)
	lns := inline(n.text, base).wrap(a.room(ind, right) - extra)
	out := []row{{line: a.compose(sel, ind, mark, lns[0], right), isCur: sel}}
	for _, ln := range lns[1:] {
		out = append(out, row{line: a.compose(false, ind+extra, nil, ln, nil)})
	}
	for _, s := range n.subs {
		out = append(out, a.listItem(false, ind+markW+extra, s, base, nil, nil)...)
	}
	return out
}

// code draws a fenced block. Long lines wrap; a diff is colored.
func (a *app) code(n *node, ind int) []row {
	var rows []row
	for _, ln := range n.code {
		st := cyan
		if n.lang == "diff" {
			switch {
			case strings.HasPrefix(ln, "@@"):
				st = cyan
			case strings.HasPrefix(ln, "+"):
				st = green
			case strings.HasPrefix(ln, "-"):
				st = red
			default:
				st = ""
			}
		}
		for _, w := range plainText(ln, st).wrap(a.room(ind, nil)) {
			rows = append(rows, row{line: a.compose(false, ind, nil, w, nil)})
		}
	}
	return rows
}

// contents is what an opened item shows under itself: the reason of its
// status, its indented lines, and the thoughts written on it.
func (a *app) contents(pv *view, n *node, ind int) []row {
	var rows []row
	// What an item holds is read, so it is in the normal color; only a done
	// item's is faint, like the item.
	base := ""
	if n.kind == kDone {
		base = dim
	}
	if n.reason != "" {
		rows = append(rows, a.para(ind, append(plainText("理由: ", dim), inline(n.reason, base)...))...)
	}
	for _, b := range n.body {
		// A blank line in the text stays one; a small heading always has
		// one above it, unless it comes first.
		if len(rows) > 0 && (b.gap || b.head) {
			rows = append(rows, row{})
		}
		switch b.kind {
		case kPara:
			rows = append(rows, a.para(ind, paraText(b, base))...)
		case kCode:
			rows = append(rows, a.code(b, ind+markW)...)
		default: // list lines
			rows = append(rows, a.listItem(false, ind+markW, b, base, nil, nil)...)
		}
	}
	if n.id != "" {
		for _, t := range a.state.on(n.id) {
			word, color := a.state.statusOf(t)
			for i, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
				if i == 0 {
					rows = append(rows, a.line(false, ind+markW, plainText("✎", dim), withImages(l, ""), plainText(word, color), true)...)
				} else {
					rows = append(rows, a.line(false, ind+markW, nil, withImages(l, ""), nil, true)...)
				}
			}
			if t.Title != "" && t.Title != n.title() {
				rows = append(rows, a.line(false, ind+markW, nil, plainText("(書いたときは: "+t.Title+")", dim).capped(8*a.textWidth()), nil, true)...)
			}
			if t.sstate != "" {
				rows = append(rows, a.line(false, ind+markW, nil, headed(settleHead(t.sstate), t.settled).capped(8*a.textWidth()), nil, true)...)
			}
		}
	}
	return rows
}

func (a *app) listCur() *thought {
	ts := a.listed()
	for _, t := range ts {
		if t.N == a.lcur {
			return t
		}
	}
	if len(ts) > 0 {
		a.lcur = ts[0].N
		return ts[0]
	}
	return nil
}

func (a *app) listRows() []row {
	rows := []row{{}, {line: a.compose(false, 0, nil, plainText("コメント", bold+blue), nil)}}
	ts := a.listed()
	if len(ts) == 0 {
		return append(rows, a.para(0, plainText("コメントはまだありません。m で書けます", dim))...)
	}
	cur := a.listCur()
	for _, t := range ts {
		sel := t == cur
		open := a.lopen[t.N]
		word, color := a.state.statusOf(t)
		first, _, _ := strings.Cut(strings.TrimSpace(t.Text), "\n")
		body := append(plainText(fmt.Sprintf("%d  ", t.N), dim), withImages(first, "")...)
		mark := plainText("▸", "")
		if open {
			mark = plainText("▾", "")
		}
		// A long destination is cut, so the thought keeps its room.
		st := plainText(word, color).capped(2 * a.textWidth()).trunc(max(a.textWidth()/2, 8))
		rows = append(rows, a.line(sel, 0, mark, body, st, false)...)
		if open {
			rows = append(rows, a.thoughtDetails(t, 0)...)
			rows = append(rows, row{})
		}
	}
	return rows
}

// ---- walking the visible tree ----

// walk visits the nodes that are on screen.
func (pv *view) walk(f func(n *node)) {
	var rec func(ns []*node)
	rec = func(ns []*node) {
		for _, n := range ns {
			if !pv.showHidden && (pv.isHidden(n) || (n.kind == kSection && pv.allHidden(n.kids))) {
				continue
			}
			f(n)
			if n.kind == kSection || ((n.kind == kDetails || n.kind == kQuestion) && pv.open[n.key]) {
				rec(n.kids)
			}
		}
	}
	rec(pv.nodes)
}

// allHidden says whether every one of ns is hidden (a Section whose lines
// are all hidden is not drawn at all).
func (pv *view) allHidden(ns []*node) bool {
	for _, n := range ns {
		if !pv.isHidden(n) {
			return false
		}
	}
	return len(ns) > 0
}

func (pv *view) navKeys() []string {
	var keys []string
	pv.walk(func(n *node) {
		if pv.navigable(n) && n.key != "" {
			keys = append(keys, n.key)
		}
	})
	return keys
}

func (pv *view) nodeAt(key string) *node {
	if key == "" {
		return nil
	}
	var found *node
	pv.walk(func(n *node) {
		if n.key == key && pv.navigable(n) {
			found = n
		}
	})
	return found
}

// linkTarget is the line under a Link: its path as written, or its URL
// (underlined, and a hyperlink where the terminal supports it). It is
// always shown and wrapped, never cut, so it can be copied.
func (a *app) linkTarget(n *node, ind int) []row {
	t := plainText(n.path, dim)
	if n.url != "" {
		t = plainText(n.url, dim+ul).hyperlink(n.url)
	}
	var rows []row
	for _, l := range t.wrap(a.room(ind, nil)) {
		rows = append(rows, row{line: a.compose(false, ind, nil, l, nil)})
	}
	return rows
}
