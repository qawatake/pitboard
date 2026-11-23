package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	margin  = 2  // left and right gutter; the cursor mark sits in the left one
	maxText = 76 // width between the gutters, including the 2-cell mark field
	markW   = 2  // the mark field every line has (▸ ▾ → ? ○ · ~ ✓)

	msgChanged  = "ページが変わったのでやり直してください"
	msgMoved    = "ページが変わりました。カーソルの行を確かめてください"
	msgNoReport = "AI はまだ報告を書いていません"
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
	id    string // the line the thought is about, if any
	title string // that line's title when writing began
	label string
	text  []rune
}

type app struct {
	t    *term
	base string
	st   *Store // nil until a session exists
	rep  *view
	msg  string // shown on the bottom line until the next key

	evMod, seenMod time.Time
	state          *state

	in   *input // the editor at the bottom, while writing
	help bool   // the list of all keys
	htop int    // how far the list of keys is scrolled

	noSession bool // the session given with -s does not exist (yet)

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
		a.state = replayEvents(nil, 0)
		return
	}
	a.evMod, a.seenMod = modTime(a.st.eventsPath()), modTime(a.st.seenPath())
	a.state = replayEvents(a.st.Events(), a.st.Seen())
	if a.rep != nil {
		a.rep.hidden = a.state.hidden
	}
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
			quit := false
			if err := safely(func() error { quit = a.handle(k); return nil }); err != nil {
				a.msg = "その操作でエラーが起きました: " + err.Error()
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
			}
		}
		a.render()
	}
}

// render draws the screen; if drawing fails, it says so instead of
// taking the viewer down.
func (a *app) render() {
	if err := safely(func() error { a.draw(); return nil }); err != nil {
		a.t.draw([]string{"", "  描画できませんでした: " + clean(err.Error()), "", "  q 終了"})
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
	a.msg = ""
	if k.name == "ctrl+c" {
		return true
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
		pv.lock, pv.placed = false, false
		if !pv.step(1) {
			pv.top++ // past the last stop: scroll the page
		}
	case k.r == 'k' || k.name == "up":
		pv.lock, pv.placed = false, false
		if !pv.step(-1) {
			pv.top = max(pv.top-1, 0)
		}
	case k.name == "tab":
		a.list, a.lfollow = true, true
	case k.r == 'm' && a.canWrite():
		a.in = &input{label: "思いつきを書く"}
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
		a.in = &input{id: n.id, title: n.title(), label: "「" + plainText(n.title(), "").capped(4*a.textWidth()).plain() + "」に書く"}
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
	op, msg := "hide", "隠しました。H で隠した行を表示"
	switch {
	case pv.isHidden(n):
		op, msg = "unhide", "戻しました"
	case pv.showHidden:
		msg = "隠しました"
	}
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
// or a plain "- " line (which can only be hidden).
func (pv *view) navigable(n *node) bool {
	return n.kind == kDetails || n.id != "" || n.kind == kPlain
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
	if n.kind == kDetails || len(n.body) > 0 || n.reason != "" || pv.cut[n.key] {
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
	}
}

// ---- the editor ----

func (a *app) handleInput(k key) {
	switch k.name {
	case "esc":
		a.in = nil
	case "backspace":
		if n := len(a.in.text); n > 0 {
			a.in.text = a.in.text[:n-1]
		}
	case "ctrl+u":
		a.in.text = nil
	case "ctrl+e":
		a.external()
	case "newline":
		a.in.text = append(a.in.text, '\n')
	case "enter":
		a.send(string(a.in.text))
	case "":
		if k.r != 0 {
			a.in.text = append(a.in.text, k.r)
		}
	}
}

func (a *app) send(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		a.in = nil
		return
	}
	if _, err := a.st.Append(Event{Op: "thought", ID: a.in.id, Text: text, Title: a.in.title}); err != nil {
		a.msg = "送れませんでした: " + err.Error()
		return
	}
	a.in = nil
	a.loadEvents()
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
	fmt.Fprintf(f, "%s\n\n%s\n# %s: この線より上が送られます。空のまま閉じると送りません。\n", string(a.in.text), scissors, a.in.label)
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
		a.msg = "空だったので送りませんでした"
		a.in = nil
		return
	}
	a.send(text)
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
	case k.name == "enter" && len(ts) > 0 && a.lcurShown:
		a.lopen[ts[i].N] = !a.lopen[ts[i].N]
	case k.r == 'm' && a.canWrite():
		a.in = &input{label: "思いつきを書く"}
	}
	return false
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
		top, follow, shown = &a.ltop, &a.lfollow, &a.lcurShown
	} else {
		pv := a.top()
		pv.settleCursor()
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
	lines := make([]string, 0, a.t.h)
	for i := *top; i < len(rows) && len(lines) < h; i++ {
		lines = append(lines, rows[i].line)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	a.t.draw(append(lines, bottom...))
}

// bottomLines is everything under the scrolling part: the editor while
// writing, or the thoughts and the key hints. n is how many rows the
// scrolling part has (the list of keys scrolls only if it does not fit).
func (a *app) bottomLines(n int) []string {
	switch {
	case a.help:
		hs := []hint{{k: "q", d: "閉じる"}}
		if n > a.t.h-1 {
			hs = []hint{{k: "j k", d: "送る"}, {k: "q", d: "閉じる"}}
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
		{"j / k", "上下に動く。止まる行がその先になければ 1 行送る。カーソルの行が画面の外にあれば、まずそこまで戻す"},
		{"enter", "開く・閉じる"},
		{"y", "よし (項目・リンク)"},
		{"n", "だめ (項目・リンク)"},
		{"a b …", "問いに答える (選択肢のキー)"},
		{"c", "この行に思いつきを書く"},
		{"m", "思いつきを書く"},
		{"x", "この行を隠す (隠した行の上では戻す)"},
		{"H", "隠した行を表示する / しない"},
		{"tab", "思いつきの一覧 / 報告に戻る"},
		{"?", "このキーの一覧"},
		{"q", "終了 (一覧からは報告に戻る)"},
		{"", ""},
		{"書いているとき", ""},
		{"enter", "送る"},
		{"ctrl+j", "改行"},
		{"ctrl+e", "$VISUAL / $EDITOR / vim で続きを書く"},
		{"esc", "やめる"},
	} {
		if k[0] == "" {
			rows = append(rows, row{})
			continue
		}
		// The description wraps in its own column, to the right of the key.
		const col = 10
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
	short string
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
	if !a.list {
		right = a.hiddenCount()
	}
	say := func(msg string) []string {
		var out []string
		for i, l := range plainText(msg, magenta).wrap(a.room(0, right)) {
			if i == 0 {
				out = append(out, a.compose(false, 0, nil, l, right))
			} else {
				out = append(out, a.compose(false, 0, nil, l, nil))
			}
		}
		return out
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
				keys = append(keys, hint{k: "enter", d: "閉じる"})
			} else {
				keys = append(keys, hint{k: "enter", d: "開く"})
			}
		}
		keys = append(keys, hint{k: "tab", d: "報告へ"})
		return a.oneLine(keys, nil)
	}
	pv := a.top()
	n := pv.nodeAt(pv.cur)
	if n == nil || !pv.curShown {
		if pv.err == "" && !pv.missing {
			keys = append(keys, hint{k: "j k", d: "送る"})
		}
	} else {
		// The keys for this line come first: they are the last to be left
		// out when the line is narrow.
		switch {
		case a.canOpen(pv, n) && pv.open[pv.cur]:
			keys = append(keys, hint{k: "enter", d: "閉じる"})
		case a.canOpen(pv, n):
			keys = append(keys, hint{k: "enter", d: "開く"})
		}
		if t := a.target(pv); t != nil {
			yn, choices, thought := keysFor(t)
			if pv.placed {
				// Placed by the viewer, not by the person: say how to react.
				if t.kind == kQuestion {
					keys = append(keys, hint{k: "j k", d: "動かすと答えられる", short: "→ 答える"})
				} else if yn {
					keys = append(keys, hint{k: "j k", d: "動かすと y n が効く", short: "→ y n"})
				}
				yn, choices = false, nil
			}
			if yn {
				keys = append(keys, hint{k: "y", d: "よし"}, hint{k: "n", d: "だめ"})
			}
			if len(choices) > 0 {
				keys = append(keys, hint{k: strings.Join(choices, " "), d: "答える"})
			}
			if thought {
				keys = append(keys, hint{k: "c", d: "書く"})
			}
		}
		if pv.placed && a.target(pv) == nil && hideKey(n) != "" && !pv.lock && a.canWrite() {
			// A plain line or a Fold the viewer put the cursor on.
			keys = append(keys, hint{k: "j k", d: "動かすと x が効く", short: "→ x"})
		}
		if a.hideTarget(pv) != nil {
			if pv.isHidden(n) {
				keys = append(keys, hint{k: "x", d: "戻す"})
			} else {
				keys = append(keys, hint{k: "x", d: "隠す"})
			}
		}
	}
	// m tab q H work everywhere: the list of keys (?) tells them.
	return a.oneLine(keys, right)
}

// oneLine is the hints: always one line, the keys for this line (at most
// five) and "? キー" last. When the line is narrow, right goes first, then
// keys from the end; "? キー" and a hint with a short form always stay.
func (a *app) oneLine(keys []hint, right text) []string {
	q := hint{k: "?", d: "キー"}
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
	for n := min(len(keys), 5); n >= 0; n-- {
		var ks []hint
		for i, k := range keys {
			if i < n || k.short != "" {
				ks = append(ks, k)
			}
		}
		for _, cand := range [][]hint{ks, shorten(ks)} {
			cand = append(cand, q)
			for _, r := range []text{right, nil} { // right goes before any key
				if keyHints(cand).width() <= a.room(0, r) {
					return []string{a.compose(false, 0, nil, keyHints(cand), r)}
				}
			}
		}
	}
	// Too narrow for both: what the person must know to react (the short
	// form of a kept hint) comes before "? キー".
	var must []hint
	for _, k := range shorten(keys) {
		if k.short != "" {
			must = append(must, k)
		}
	}
	if len(must) > 0 && keyHints(must).width() <= a.room(0, nil) {
		return []string{a.compose(false, 0, nil, keyHints(must), nil)}
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
		if t.settled == "" {
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
	n := min(paneMax, len(order))
	if a.t.h < paneMinH {
		n = 0
	}
	for n > 0 && avail-(n+3) < minBodyRows { // + heading and the blank above and below
		n--
	}
	unread := 0
	for _, t := range a.state.thoughts {
		if t.N > a.state.seen {
			unread++
		}
	}
	// Quiet: the only thing that draws the eye is what the AI has not read.
	var right text
	if unread > 0 {
		right = plainText(fmt.Sprintf("未読 %d", unread), yellow)
	}
	head := a.compose(false, 0, nil, plainText("思いつき", bold+blue), right)
	if n == 0 {
		return []string{head}
	}
	out := []string{"", head}
	w := a.textWidth()
	for _, t := range order[:n] {
		word, color := a.state.statusOf(t)
		base := ""
		if t.settled != "" {
			base = dim
		}
		body := plainText(strings.Join(strings.Fields(t.Text), " "), base).capped(2 * w)
		out = append(out, a.compose(false, 2, nil, body, plainText(word, color)))
	}
	return append(out, "")
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
	w := a.textWidth() - markW
	var lns []text
	for _, l := range strings.Split(string(a.in.text), "\n") {
		lns = append(lns, plainText(l, "").wrap(w)...)
	}
	if limit := max(a.t.h/3, 3); len(lns) > limit {
		lns = lns[len(lns)-limit:]
	}
	hints := a.packKeys([]hint{{k: "enter", d: "送る"}, {k: "ctrl+j", d: "改行"}, {k: "ctrl+e", d: "エディタ"}, {k: "esc", d: "やめる"}}, nil, false)
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
	for i, l := range lns {
		if i == len(lns)-1 {
			l = append(l, cell{r: ' ', st: rev})
		}
		out = append(out, a.compose(false, 0, nil, l, nil))
	}
	return append(out, hints...)
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

// right is what goes on the right end of a line: the status the AI wrote,
// the person's reaction (while the line is unchanged), and thoughts on it.
func (a *app) right(pv *view, n *node) text {
	var st text
	switch {
	case n.status == "":
	case strings.HasPrefix(n.status, "あなた待ち"):
		st = plainText(n.status, magenta)
	default:
		st = plainText(n.status, yellow)
	}
	if n.id == "" {
		return st
	}
	var re, th text
	if e, ok := a.state.react[n.id]; ok && e.Line == n.sig() {
		switch e.Value {
		case "y":
			re = plainText("y", bold+green)
		case "n":
			re = plainText("n", bold+red)
		default:
			re = plainText(e.Value, bold+cyan)
		}
	}
	if c := len(a.state.on(n.id)); c > 0 {
		th = plainText(fmt.Sprintf("✎%d", c), dim)
	}
	return join(st, re, th)
}

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
	case a.noSession:
		return append(rows, a.para(0, plainText(fmt.Sprintf("セッション %s はまだありません", a.st.session), dim))...)
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
	pv.walk(func(n *node) {
		sel := n.key != "" && n.key == pv.cur
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
			rows = append(rows, a.para(ind, inline(n.text, ""))...)
			rows = append(rows, row{})
		case kCode:
			blank()
			rows = append(rows, a.code(n, ind+2)...)
			rows = append(rows, row{})
		case kChoice:
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
			if n.kind == kDetails && n.depth == 0 {
				blank()
			}
			if n.kind == kQuestion {
				chosen = ""
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
			// A line the cursor cannot stop at cannot be opened either, so
			// what it holds is always shown.
			shown := (can && open) || (!nav && n.kind != kDetails)
			wrap := whole || (can && open)
			mark := markOf(n.kind, can, open)
			if hidden {
				mark = plainText("⌀", dim)
			}
			lns := a.line(sel, ind, mark, body, right, wrap)
			if nav {
				lns[0].key = n.key
			}
			rows = append(rows, lns...)
			if n.kind == kLink {
				rows = append(rows, a.linkTarget(n, ind)...)
			}
			if shown {
				rows = append(rows, a.contents(pv, n, ind)...)
			}
		}
	})
	return rows
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
	if n.reason != "" {
		rows = append(rows, a.para(ind, inline(n.reason, magenta))...)
	}
	for _, b := range n.body {
		switch b.kind {
		case kPara:
			rows = append(rows, a.para(ind, inline(b.text, dim))...)
		case kCode:
			rows = append(rows, a.code(b, ind+markW)...)
		default: // list lines (in Markdown, maybe with a checkbox)
			rows = append(rows, a.line(false, ind+markW, markOf(b.kind, false, false), inline(b.text, dim), nil, true)...)
		}
	}
	if n.id != "" {
		for _, t := range a.state.on(n.id) {
			word, color := a.state.statusOf(t)
			for i, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
				if i == 0 {
					rows = append(rows, a.line(false, ind+markW, plainText("✎", dim), plainText(l, ""), plainText(word, color), true)...)
				} else {
					rows = append(rows, a.line(false, ind+markW, nil, plainText(l, ""), nil, true)...)
				}
			}
			if t.Title != "" && t.Title != n.title() {
				rows = append(rows, a.line(false, ind+markW, nil, plainText("(書いたときの題: "+t.Title+")", dim).capped(8*a.textWidth()), nil, true)...)
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
	rows := []row{{}, {line: a.compose(false, 0, nil, plainText("思いつき", bold+blue), nil)}}
	ts := a.listed()
	if len(ts) == 0 {
		return append(rows, a.para(0, plainText("まだありません。m で書けます", dim))...)
	}
	cur := a.listCur()
	var nodes []*node
	if a.rep != nil {
		nodes = a.rep.nodes
	}
	for _, t := range ts {
		sel := t == cur
		open := a.lopen[t.N]
		word, color := a.state.statusOf(t)
		first, _, _ := strings.Cut(strings.TrimSpace(t.Text), "\n")
		body := append(plainText(fmt.Sprintf("%d  ", t.N), dim), plainText(first, "")...)
		mark := plainText("▸", "")
		if open {
			mark = plainText("▾", "")
		}
		// A long destination is cut, so the thought keeps its room.
		st := plainText(word, color).capped(2 * a.textWidth()).trunc(max(a.textWidth()/2, 8))
		rows = append(rows, a.line(sel, 0, mark, body, st, false)...)
		if open {
			if t.ID != "" {
				about := "(報告にもうない行)"
				if n := byID(nodes, t.ID); n != nil {
					about = n.title()
				}
				if t.Title != "" && t.Title != about {
					about += " (書いたときの題: " + t.Title + ")"
				}
				rows = append(rows, a.para(0, plainText("行: "+about, dim))...)
			}
			for _, l := range strings.Split(strings.TrimSpace(t.Text), "\n") {
				rows = append(rows, a.para(0, plainText(l, ""))...)
			}
			switch {
			case t.settled != "":
				rows = append(rows, a.para(0, plainText("行き先: "+t.settled, yellow))...)
			case t.N > a.state.seen:
				rows = append(rows, a.para(0, plainText("まだ AI に届いていません", dim))...)
			default:
				rows = append(rows, a.para(0, plainText("AI は受け取りました。行き先はまだです", dim))...)
			}
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
			if n.kind == kQuestion || n.kind == kSection || (n.kind == kDetails && pv.open[n.key]) {
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
