package board

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// When the AI rewrites report.md it may drop a row the person still has
// business with: one that wants them (an Ask they have not answered, wait="you" they have not acted on, look="y" they have not
// read) or one with a sent comment the AI has not settled done. Such a
// removal is said once: 'check' lists it until then, and the next other
// command the AI runs ends with it; then it is forgotten. The AI learns it
// from the tool, not from the person's next comment arriving on a gone row.
//
// The rows are remembered in the session's rows.json: every row with an
// id, and every Fold without one (by report.Target: "Fold|title=<title>",
// which is what its reactions, comments and hiding go by), with what is
// needed to tell whether the person is done with it. Each command the AI
// runs adds the rows the report has now, when it parses without problems.
// A gone row is forgotten once it has been said, or at once when nothing
// about it is left open.

// rowsPath is the session's remembered rows.
func (s *Store) rowsPath() string { return filepath.Join(s.Dir, "rows.json") }

// seenRow is a row as it was remembered.
type seenRow struct {
	Title   string
	Ask     bool   `json:",omitempty"` // an Ask: answered by an option, a sent comment or hiding
	Wait    bool   `json:",omitempty"` // wait="you": a reaction, hiding or a sent comment
	Look    bool   `json:",omitempty"` // look="y": reading it is enough
	HideKey string `json:",omitempty"`
	// Sig is what the row said (report.Node.Sig): an answer given to what
	// it said before does not answer it (as its mark: page.Board.Waiting).
	Sig  string `json:",omitempty"`
	View string `json:",omitempty"` // the hash of its ViewKey (read = the person viewed it)
}

func (s *Store) readRows() map[string]seenRow {
	rows := map[string]seenRow{}
	if b, err := os.ReadFile(s.rowsPath()); err == nil {
		json.Unmarshal(b, &rows)
	}
	return rows
}

func (s *Store) writeRows(rows map[string]seenRow) {
	b, err := json.Marshal(rows)
	if err == nil {
		writeFile(s.rowsPath(), b)
	}
}

// reportRows is the rows of the report as it is now, by report.Target
// (rows with an id, and Folds without one); ok false when it cannot be
// read or does not parse; valid false when it has problems.
func (s *Store) reportRows() (now map[string]*report.Node, ok, valid bool) {
	b, err := os.ReadFile(s.ReportPath())
	if err != nil {
		return nil, false, false
	}
	nodes, probs, err := report.Parse(string(b))
	if err != nil {
		return nil, false, false
	}
	now = map[string]*report.Node{}
	report.WalkAll(nodes, func(n *report.Node) {
		if n.ID != "" || n.Kind == report.KindDetails {
			if k := report.Target(n); k != "" {
				if _, dup := now[k]; !dup {
					now[k] = n
				}
			}
		}
	})
	return now, true, len(probs) == 0
}

// addRows remembers the rows of now (as they are now) in rows.
func addRows(rows map[string]seenRow, now map[string]*report.Node) {
	for k, n := range now {
		rows[k] = seenRow{Title: n.Title(), HideKey: report.HideKey(n), View: viewHash(n), Sig: n.Sig(),
			Ask:  n.Kind == report.KindQuestion,
			Wait: n.Wait == "you",
			Look: n.Look == "y"}
	}
}

// RememberShown remembers the rows of the report as a screen (the
// viewer, a browser page) has just shown them to the person, so that one
// the AI removes before it runs a command is still warned of. Only a
// report without problems; rows.json is written only when it changes.
func (s *Store) RememberShown() {
	defer func() { recover() }()
	now, ok, valid := s.reportRows()
	if !ok || !valid || len(now) == 0 {
		return
	}
	rows := s.readRows()
	before, _ := json.Marshal(rows)
	addRows(rows, now)
	if after, _ := json.Marshal(rows); string(after) != string(before) {
		s.writeRows(rows)
	}
}

// Removed is the warnings about rows the report no longer has, one line
// each, in key order ('check' prints them; they stay until said once by
// TakeRemoved). Nothing when the report cannot be read (it is not the time
// to warn of rows).
func (s *Store) Removed() []string { return s.removed(false) }

// TakeRemoved is Removed, and forgets the rows it warns of: each removal
// is said once.
func (s *Store) TakeRemoved() []string { return s.removed(true) }

func (s *Store) removed(take bool) (warnings []string) {
	defer func() {
		if recover() != nil {
			warnings = nil
		}
	}()
	if _, err := os.Stat(s.Dir); err != nil {
		return nil
	}
	now, ok, valid := s.reportRows()
	if !ok {
		return nil
	}
	rows := s.readRows()
	before, _ := json.Marshal(rows)
	// A report with problems is not remembered: its titles (or its rows)
	// may be what a broken edit left, and the warnings keep the last good
	// ones.
	if valid {
		addRows(rows, now)
	}
	st := Replay(s.Events(), s.Seen())
	viewed := s.Viewed()
	var keys []string
	for k := range rows {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if _, there := now[k]; there {
			continue
		}
		w := removedWarnings(k, rows[k], st, viewed)
		warnings = append(warnings, w...)
		// Forgotten when nothing about it is left open, or once said.
		if len(w) == 0 || take {
			delete(rows, k)
		}
	}
	if after, _ := json.Marshal(rows); string(after) != string(before) {
		s.writeRows(rows)
	}
	return warnings
}

func viewHash(n *report.Node) string {
	if k := ViewKey(n); k != "" {
		return HashKey(k)
	}
	return ""
}

// removedWarnings is what to say about the row remembered by key (its id,
// or an id-less Fold's target), gone from the report.
func removedWarnings(key string, r seenRow, st *State, viewed map[string]bool) []string {
	var out []string
	head := fmt.Sprintf("warning: row %q (%s) was removed", key, cutTitle(r.Title))
	if title, fold := report.FoldTitle(key); fold {
		head = fmt.Sprintf("warning: Fold %q was removed", cutTitle(title))
	}
	// What is in effect now: Replay leaves out what was taken back.
	// An answer counts only on what the row said when it was answered.
	e, reacted := st.React[key]
	reacted = reacted && (r.Sig == "" || e.Line == "" || e.Line == r.Sig)
	hidden := r.HideKey != "" && st.Hidden[r.HideKey]
	var sent, open []string
	for _, t := range st.On(key) {
		// A comment never sent is not known to the AI: not its business.
		if t.N > st.Sent {
			continue
		}
		sent = append(sent, strconv.Itoa(t.N))
		if !t.Finished() {
			open = append(open, strconv.Itoa(t.N))
		}
	}
	var handled bool
	switch {
	case r.Ask, r.Wait:
		handled = reacted || hidden || len(sent) > 0
	case r.Look:
		handled = reacted || hidden || (r.View != "" && viewed[r.View])
	default:
		handled = true
	}
	if !handled {
		out = append(out, head+" but the person has not hidden or answered it")
	}
	if len(open) > 0 {
		out = append(out, head+" but it has an open comment ("+strings.Join(open, ", ")+")")
	}
	return out
}

func cutTitle(t string) string {
	r := []rune(t)
	if len(r) > 24 {
		return string(r[:23]) + "…"
	}
	return t
}
