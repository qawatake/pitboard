package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// A board is per session, in .pitboard/<session>/ under the directory the
// commands run in (base):
//
//	report.md     written by the AI only (with its usual file edits)
//	events.jsonl  written by the viewer (and `pitboard settle`), append-only
//	seen          the last event number `pitboard changes` has delivered
type Store struct {
	base    string // the directory holding .pitboard/; Link paths are relative to it
	session string
	dir     string
}

func newStore(base, session string) *Store {
	return &Store{base: base, session: session, dir: filepath.Join(base, ".pitboard", session)}
}

var reSession = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sessions lists the boards under base, most recently updated first.
func sessions(base string) []string {
	ents, err := os.ReadDir(filepath.Join(base, ".pitboard"))
	if err != nil {
		return nil
	}
	type s struct {
		name string
		t    time.Time
	}
	var ss []s
	for _, e := range ents {
		if !e.IsDir() || !reSession.MatchString(e.Name()) {
			continue
		}
		st := newStore(base, e.Name())
		t := modTime(st.dir)
		for _, p := range []string{st.reportPath(), st.eventsPath(), st.seenPath()} {
			if m := modTime(p); m.After(t) {
				t = m
			}
		}
		ss = append(ss, s{e.Name(), t})
	}
	sort.SliceStable(ss, func(i, j int) bool { return ss[i].t.After(ss[j].t) })
	out := make([]string, len(ss))
	for i, x := range ss {
		out[i] = x.name
	}
	return out
}

// resolveSession turns what was given (-s or $PITBOARD_SESSION) into a
// session name: an existing session by its name or a unique prefix, or a new
// name.
func resolveSession(base, given string) (string, error) {
	if !reSession.MatchString(given) {
		return "", fmt.Errorf("session %q: use A-Z a-z 0-9 _ -", given)
	}
	var hits []string
	for _, s := range sessions(base) {
		if s == given {
			return s, nil
		}
		if strings.HasPrefix(s, given) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 0:
		return given, nil // a new session
	case 1:
		return hits[0], nil
	}
	sort.Strings(hits)
	return "", fmt.Errorf("session %q matches %s", given, strings.Join(hits, ", "))
}

// ancestors is dir and the directories above it, nearest first. Under the
// home directory the search stops at home: a board is never looked for
// above it.
func ancestors(dir string) []string {
	dir = filepath.Clean(dir)
	home, _ := os.UserHomeDir()
	if home != "" {
		home = filepath.Clean(home)
	}
	var out []string
	for {
		out = append(out, dir)
		if dir == home {
			return out
		}
		up := filepath.Dir(dir)
		if up == dir {
			return out
		}
		dir = up
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// findSession looks for the board of the session given, from dir upwards:
// the first directory with .pitboard/<session>/ wins. With prefix, an
// existing session may be given by a unique prefix (checked level by level).
// ok is false when there is no such board anywhere.
func findSession(dir, given string, prefix bool) (base, name string, ok bool, err error) {
	if !reSession.MatchString(given) {
		return "", "", false, fmt.Errorf("session %q: use A-Z a-z 0-9 _ -", given)
	}
	for _, d := range ancestors(dir) {
		name := given
		if prefix {
			if name, err = resolveSession(d, given); err != nil {
				return "", "", false, err
			}
		}
		if isDir(newStore(d, name).dir) {
			return d, name, true, nil
		}
	}
	return "", "", false, nil
}

// boardRoot is where boards live for commands run in dir: the nearest
// directory (from dir upwards) that has a .pitboard/ directory, or dir.
func boardRoot(dir string) string {
	for _, d := range ancestors(dir) {
		if isDir(filepath.Join(d, ".pitboard")) {
			return d
		}
	}
	return dir
}

func (s *Store) reportPath() string { return filepath.Join(s.dir, "report.md") }
func (s *Store) eventsPath() string { return filepath.Join(s.dir, "events.jsonl") }
func (s *Store) seenPath() string   { return filepath.Join(s.dir, "seen") }

// Event is one line of events.jsonl.
//
//	react    id, value (y / n / a choice letter), line (what the line was)
//	thought  text, and id when written on a line
//	settle   thought, text, by (the AI's note on where a thought went), and
//	         state: doing / hold / done (none: done). The latest counts.
//	hide     key (what the line says, see hideKey), title: the person no
//	         longer needs to see that line
//	unhide   key, title: show it again
//	send     the person sent what is above it: 'changes' delivers only
//	         what comes before the latest send
//	retract  thought: the person took the thought back
//	edit     thought, text: the person rewrote the thought (the latest
//	         edit's text is the thought's)
//	unretract  thought: they undid that (u right after x)
type Event struct {
	N       int    `json:"n"`
	T       string `json:"t"`
	Op      string `json:"op"`
	ID      string `json:"id,omitempty"`
	Value   string `json:"value,omitempty"`
	Text    string `json:"text,omitempty"`
	Thought int    `json:"thought,omitempty"`
	By      string `json:"by,omitempty"`
	// Line is the reacted line as the person saw it (see node.sig). When the
	// AI rewrites the line, the reaction is no longer shown on it.
	Line string `json:"line,omitempty"`
	// Title is the line's title when the person reacted or began writing,
	// and Option the text of the chosen option: what the person saw, told to
	// the AI next to what the line says now.
	Title  string `json:"title,omitempty"`
	Option string `json:"option,omitempty"`
	// Key is what a hidden line says (see hideKey).
	Key string `json:"key,omitempty"`
	// State is how far the AI is with a thought (settle): doing, hold or
	// done ("" is done).
	State string `json:"state,omitempty"`
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
	b, err := os.ReadFile(s.eventsPath())
	if err != nil {
		return nil
	}
	return parseEvents(b)
}

// locked runs f with events.jsonl open and locked (flock), so that the viewer
// and the AI's commands never interleave or reuse a number.
func (s *Store) locked(f func(file *os.File, evs []Event) error) error {
	if _, err := os.Stat(s.dir); err != nil {
		return fmt.Errorf("no session %q yet", s.session)
	}
	file, err := os.OpenFile(s.eventsPath(), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
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
	b, err := os.ReadFile(s.seenPath())
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

// Changes returns the person's actions the AI has not been given yet, one
// line each with context from the report, and remembers that they were given.
func (s *Store) Changes() ([]string, error) {
	if _, err := os.Stat(s.eventsPath()); os.IsNotExist(err) {
		return nil, nil // the person has done nothing yet
	}
	var out []string
	err := s.locked(func(_ *os.File, evs []Event) error {
		seen := s.Seen()
		var nodes []*node
		if b, err := os.ReadFile(s.reportPath()); err == nil {
			nodes, _, _ = parseReport(string(b))
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
		return writeFile(s.seenPath(), []byte(strconv.Itoa(last)+"\n"))
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
	for _, e := range all {
		if e.N < cut {
			evs = append(evs, e)
		}
	}
	thoughts := map[int]Event{}
	final := map[int]bool{}  // taken back, in the end
	told := map[int]bool{}   // taken back, as the AI was last told
	lastOp := map[int]int{}  // the number of the last retract / unretract
	text := map[int]string{} // the thought's text, as last edited
	lastEdit := map[int]int{}
	for _, e := range evs {
		switch e.Op {
		case "edit":
			text[e.Thought] = e.Text
			if e.N > seen {
				lastEdit[e.Thought] = e.N
			}
		case "thought":
			thoughts[e.N] = e
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
			e.Op, e.Text = "unretract", t.Text
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
func compact(evs []Event, nodes []*node, image func(n int) string) []string {
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
			hides = append(hides, cutTitle(e.Title, hideTitleMax))
			continue
		case e.Op == "unhide":
			unhides = append(unhides, cutTitle(e.Title, hideTitleMax))
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

// cutTitle is a title on one line, cut to max characters with ….
func cutTitle(s string, max int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return string(r)
}

// describe is how an event is told to the AI: with the title of the line as
// the person saw it, and what it says now if that differs.
func describe(e Event, nodes []*node) string {
	now := byID(nodes, e.ID)
	seen := e.Title
	if seen == "" && now != nil {
		seen = now.title() // recorded before titles were kept
	}
	ctx := seen
	switch {
	case now == nil && seen == "":
		ctx = "gone"
	case now == nil:
		ctx += " → now gone"
	case now.title() != seen:
		ctx += " → now: " + now.title()
	}
	switch e.Op {
	case "react":
		answer := e.Option != "" || (now != nil && now.kind == kQuestion) ||
			(now == nil && e.Value != "y" && e.Value != "n")
		if !answer {
			return fmt.Sprintf("%s %s (%s)", e.Value, e.ID, ctx)
		}
		opt := e.Option
		if opt == "" && now != nil {
			if ch := now.choice(e.Value); ch != nil {
				opt = plainOf(ch.text)
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
		ctx := cutTitle(seen, thoughtTitleMax)
		switch {
		case now == nil && seen == "":
			ctx = "gone"
		case now == nil:
			ctx += " → now gone"
		case now.title() != seen:
			ctx += " → now: " + cutTitle(now.title(), thoughtTitleMax)
		}
		return fmt.Sprintf("thought %d on %s (%s): %s", e.N, e.ID, ctx, text)
	case "hide", "unhide":
		return fmt.Sprintf("%s: %s", e.Op, strings.ReplaceAll(e.Title, "\n", " "))
	case "edit":
		text := strings.ReplaceAll(strings.TrimSpace(e.Text), "\n", "\n  ")
		return fmt.Sprintf("edit thought %d: %s", e.Thought, text)
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
	return s.SettleAll([]settlement{{n, "done", text}})
}

// settlement is one thought's settle: its number, state (doing, hold or
// done) and text.
type settlement struct {
	n     int
	state string
	text  string
}

// SettleAll records each settlement: all of them, or (when a number is not
// a thought, or a done one has no text) none.
func (s *Store) SettleAll(ss []settlement) error {
	evs := s.Events()
	for i := range ss {
		x := &ss[i]
		x.text = strings.TrimSpace(x.text)
		if x.state == "" {
			x.state = "done"
		}
		if x.text == "" && x.state == "done" {
			return fmt.Errorf("settle %d needs a text", x.n)
		}
		if findThought(evs, x.n) == nil {
			return fmt.Errorf("no thought %d", x.n)
		}
	}
	for _, x := range ss {
		e := Event{Op: "settle", Thought: x.n, Text: x.text, By: "ai"}
		if x.state != "done" {
			e.State = x.state
		}
		if _, err := s.Append(e); err != nil {
			return err
		}
	}
	return nil
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

// ---- what the events add up to, for the viewer ----

type thought struct {
	Event
	settled   string // the AI's note, if any
	settledN  int    // the number of that settle event
	sstate    string // the AI's state for it: "" (none yet), doing, hold, done
	retracted bool   // the person took it back (not shown)
	lastN     int    // the number of the thought or its latest edit
}

// state is the events replayed: the latest reaction per id, thoughts, and
// the lines the person hid.
type state struct {
	react    map[string]Event
	thoughts []*thought      // oldest first
	seen     int             // what the AI has read ('changes')
	sent     int             // the latest send: what the person has sent
	hidden   map[string]bool // hideKey of each hidden line
}

func replayEvents(evs []Event, seen int) *state {
	st := &state{react: map[string]Event{}, seen: seen, sent: lastSend(evs), hidden: map[string]bool{}}
	byN := map[int]*thought{}
	for _, e := range evs {
		switch e.Op {
		case "react":
			st.react[e.ID] = e
		case "thought":
			t := &thought{Event: e, lastN: e.N}
			st.thoughts = append(st.thoughts, t)
			byN[e.N] = t
		case "settle":
			if t := byN[e.Thought]; t != nil {
				t.settled, t.settledN, t.sstate = e.Text, e.N, e.State
				if t.sstate == "" {
					t.sstate = "done"
				}
			}
		case "retract", "unretract":
			if t := byN[e.Thought]; t != nil {
				t.retracted = e.Op == "retract"
			}
		case "edit":
			// Rewritten: the AI's destination was for the old text.
			if t := byN[e.Thought]; t != nil {
				t.Text, t.lastN = e.Text, e.N
				t.settled, t.settledN, t.sstate = "", 0, ""
			}
		case "hide":
			st.hidden[e.Key] = true
		case "unhide":
			delete(st.hidden, e.Key)
		}
	}
	// A thought taken back is gone from the board.
	kept := st.thoughts[:0]
	for _, t := range st.thoughts {
		if !t.retracted {
			kept = append(kept, t)
		}
	}
	st.thoughts = kept
	return st
}

func (st *state) on(id string) []*thought {
	var out []*thought
	for _, t := range st.thoughts {
		if t.ID == id {
			out = append(out, t)
		}
	}
	return out
}

// finished says whether the AI is done with t.
func (t *thought) finished() bool { return t.sstate == "done" }

// statusOf is a comment's state in one word: not sent, sent (not read by
// the AI yet), read, then as the AI settled it: being handled, on hold,
// handled. Only what needs the person (not sent) is in color.
func (st *state) statusOf(t *thought) (word, color string) {
	switch {
	case t.sstate != "":
		return settleWord(t.sstate), dim // what the AI wrote is read in the list (tab, enter)
	case t.lastN > st.sent:
		return "未送信", yellow
	}
	return st.sentWord(t.lastN), dim
}

// sentWord is the state of something sent: 送信済 until the AI reads it,
// then 既読 (or 未送信 when it was not sent yet).
func (st *state) sentWord(n int) string {
	switch {
	case n > st.sent:
		return "未送信"
	case n > st.seen:
		return "送信済"
	}
	return "既読"
}

func settleWord(state string) string {
	switch state {
	case "doing":
		return "対応中"
	case "hold":
		return "保留"
	}
	return "対応済"
}
