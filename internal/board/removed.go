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
// business with: one that asks for them (look="y", ask="y", an Ask) that
// they have not reacted to, read or hidden, or one with a comment the AI
// has not settled done. 'check' warns of each (and the line every command
// the AI runs ends with counts them), so the AI learns it from the tool,
// not from the person's next comment arriving on a gone row.
//
// The rows are remembered in the session's rows.json: every row with an
// id the report has had since it was last acknowledged (each command the
// AI runs adds the rows the report has now). A gone row is warned of
// until 'ack-removed', or until it comes back, or until nothing about it
// is left open (its comments settled done); then it is forgotten.

// rowsPath is the session's remembered rows.
func (s *Store) rowsPath() string { return filepath.Join(s.Dir, "rows.json") }

// seenRow is a row as it was remembered.
type seenRow struct {
	Title   string
	Asks    bool   `json:",omitempty"` // look="y", ask="y" or an Ask: it wants the person
	HideKey string `json:",omitempty"`
	View    string `json:",omitempty"` // the hash of its ViewKey (read = the person viewed it)
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

// reportRows is the rows with an id of the report as it is now (ok false
// when it cannot be read or does not parse).
func (s *Store) reportRows() (map[string]*report.Node, bool) {
	b, err := os.ReadFile(s.ReportPath())
	if err != nil {
		return nil, false
	}
	nodes, _, err := report.Parse(string(b))
	if err != nil {
		return nil, false
	}
	now := map[string]*report.Node{}
	report.WalkAll(nodes, func(n *report.Node) {
		if n.ID != "" {
			now[n.ID] = n
		}
	})
	return now, true
}

// Removed is the warnings about rows the report no longer has, one line
// each, in id order; it remembers the rows the report has now. Nothing
// when the report cannot be read (it is not the time to warn of rows).
func (s *Store) Removed() (warnings []string) {
	defer func() {
		if recover() != nil {
			warnings = nil
		}
	}()
	if _, err := os.Stat(s.Dir); err != nil {
		return nil
	}
	now, ok := s.reportRows()
	if !ok {
		return nil
	}
	rows := s.readRows()
	before, _ := json.Marshal(rows)
	for id, n := range now {
		rows[id] = seenRow{Title: n.Title(), HideKey: report.HideKey(n), View: viewHash(n),
			Asks: n.Look == "y" || n.Ask == "y" || n.Kind == report.KindQuestion}
	}
	st := Replay(s.Events(), s.Seen())
	viewed := s.Viewed()
	var ids []string
	for id := range rows {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if _, there := now[id]; there {
			continue
		}
		r := rows[id]
		w := removedWarnings(id, r, st, viewed)
		if len(w) == 0 {
			delete(rows, id) // nothing about it is left open
			continue
		}
		warnings = append(warnings, w...)
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

// removedWarnings is what to say about row id, gone from the report.
func removedWarnings(id string, r seenRow, st *State, viewed map[string]bool) []string {
	var out []string
	head := fmt.Sprintf("warning: row %q (%s) was removed", id, cutTitle(r.Title))
	_, reacted := st.React[id]
	answered := reacted || (r.HideKey != "" && st.Hidden[r.HideKey]) || (r.View != "" && viewed[r.View])
	if r.Asks && !answered {
		out = append(out, head+" but the person has not hidden or answered it")
	}
	var open []string
	for _, t := range st.On(id) {
		if !t.Finished() {
			open = append(open, strconv.Itoa(t.N))
		}
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
	now, ok := s.reportRows()
	if !ok {
		return 0, fmt.Errorf("%s: cannot be read", s.ReportPath())
	}
	rows := s.readRows()
	n := 0
	for id := range rows {
		if _, there := now[id]; !there {
			delete(rows, id)
			n++
		}
	}
	if n > 0 {
		s.writeRows(rows)
	}
	return n, nil
}
