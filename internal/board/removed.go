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
// business with: one that wants them (ask="y" or an Ask they have not
// answered, wait="you" they have not acted on, look="y" they have not
// read) or one with a sent comment the AI has not settled done. 'check'
// warns of each every time; the line the other commands the AI runs end
// with counts each removal once, so the AI learns it from the tool, not
// from the person's next comment arriving on a gone row.
//
// The rows are remembered in the session's rows.json: every row with an
// id, and every Fold without one (by report.Target: "Fold|title=<title>",
// which is what its reactions, comments and hiding go by), the report has
// had since it was last acknowledged (each command the AI runs adds the
// rows the report has now, when it parses without problems). A gone row
// is warned of until 'ack-removed', or until it comes back, or until
// nothing about it is left open; then it is forgotten.

// rowsPath is the session's remembered rows.
func (s *Store) rowsPath() string { return filepath.Join(s.Dir, "rows.json") }

// seenRow is a row as it was remembered.
type seenRow struct {
	Title   string
	Ask     bool   `json:",omitempty"` // ask="y" or an Ask: answered by a reaction or hiding
	Wait    bool   `json:",omitempty"` // wait="you": a reaction, hiding or a sent comment
	Look    bool   `json:",omitempty"` // look="y": reading it is enough
	HideKey string `json:",omitempty"`
	View    string `json:",omitempty"` // the hash of its ViewKey (read = the person viewed it)
	Noted   bool   `json:",omitempty"` // counted once on a command's last line since it went
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

// Removed is the warnings about rows the report no longer has, one line
// each, in key order ('check' prints them all every time). Nothing when
// the report cannot be read (it is not the time to warn of rows).
func (s *Store) Removed() []string {
	w, _ := s.removed(false)
	return w
}

// removed is Removed; with note, it also counts the warnings of rows not
// counted before (fresh) and remembers that they were.
func (s *Store) removed(note bool) (warnings []string, fresh int) {
	defer func() {
		if recover() != nil {
			warnings, fresh = nil, 0
		}
	}()
	if _, err := os.Stat(s.Dir); err != nil {
		return nil, 0
	}
	now, ok, valid := s.reportRows()
	if !ok {
		return nil, 0
	}
	rows := s.readRows()
	before, _ := json.Marshal(rows)
	// A report with problems is not remembered: its titles (or its rows)
	// may be what a broken edit left, and the warnings keep the last good
	// ones.
	if valid {
		for k, n := range now {
			rows[k] = seenRow{Title: n.Title(), HideKey: report.HideKey(n), View: viewHash(n),
				Ask:  n.Ask == "y" || n.Kind == report.KindQuestion,
				Wait: n.Wait == "you",
				Look: n.Look == "y"}
		}
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
		r := rows[k]
		w := removedWarnings(k, r, st, viewed)
		if len(w) == 0 {
			delete(rows, k) // nothing about it is left open
			continue
		}
		warnings = append(warnings, w...)
		if note && !r.Noted {
			fresh += len(w)
			r.Noted = true
			rows[k] = r
		}
	}
	if after, _ := json.Marshal(rows); string(after) != string(before) {
		s.writeRows(rows)
	}
	return warnings, fresh
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
	_, reacted := st.React[key]
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
	case r.Ask:
		handled = reacted || hidden
	case r.Wait:
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

// AckRemoved forgets the rows the report no longer has: their warnings
// stop. It says how many.
func (s *Store) AckRemoved() (int, error) {
	now, ok, _ := s.reportRows()
	if !ok {
		return 0, fmt.Errorf("%s: cannot be read", s.ReportPath())
	}
	rows := s.readRows()
	n := 0
	for k := range rows {
		if _, there := now[k]; !there {
			delete(rows, k)
			n++
		}
	}
	if n > 0 {
		s.writeRows(rows)
	}
	return n, nil
}
