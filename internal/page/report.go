package page

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/report"
)

// Report is the report as last read.
type Report struct {
	Path    string // "" when there is no session
	Mtime   time.Time
	Err     string // it could not be read (not a format problem)
	Missing bool   // it does not exist yet
	Nodes   []*report.Node
	Probs   []report.Problem // what is wrong with how it is written
	// ReadErr is set when the last read failed (a bug): Nodes and Probs
	// are still what was read before.
	ReadErr string
	// Fixing is set when the file has problems (what 'pitboard check'
	// fails on) and a version without any was read before: Nodes are still
	// that version, Probs are empty, and screens say MsgFixing. A report
	// that never had a good version shows its problems.
	Fixing bool
	good   bool // Nodes are from a version without problems (an empty one is not kept)
}

// NewReport reads the report at path ("" when there is no session).
func NewReport(path string) *Report {
	r := &Report{Path: path}
	r.Load()
	return r
}

// Load reads the report again. Whatever goes wrong while reading, what was
// read before is kept; so is the last version without problems, while the
// file has problems (Fixing).
func (r *Report) Load() {
	if r.Path == "" {
		r.Missing = true
		return
	}
	r.Mtime = ModTime(r.Path)
	b, err := os.ReadFile(r.Path)
	if err != nil {
		if os.IsNotExist(err) {
			r.Missing, r.Err, r.Nodes, r.Probs = true, "", nil, nil
			r.Fixing, r.good = false, false
			return
		}
		r.Err = err.Error()
		return
	}
	r.Err, r.Missing = "", false
	src := strings.ReplaceAll(string(b), "\r\n", "\n")
	err = Safely(func() error {
		nodes, probs, err := report.Parse(src)
		if err != nil {
			return err
		}
		if len(probs) > 0 && r.good {
			// The person keeps seeing the last good version.
			r.Probs, r.Fixing = nil, true
			return nil
		}
		r.Nodes, r.Probs = nodes, probs
		r.good, r.Fixing = len(probs) == 0 && len(nodes) > 0, false
		return nil
	})
	r.ReadErr = ""
	if err != nil {
		r.ReadErr = "報告を読めませんでした (前の画面を出しています): " + err.Error()
	}
}

// Stale says whether the file changed since it was read.
func (r *Report) Stale() bool { return !ModTime(r.Path).Equal(r.Mtime) }

// fresh reads the report from the file as it is now, for checking that a
// line is still what the person saw before recording on it.
func (r *Report) fresh() ([]*report.Node, error) {
	b, err := os.ReadFile(r.Path)
	if err != nil {
		return nil, &readError{err}
	}
	nodes, _, err := report.Parse(string(b))
	return nodes, err
}

// Safely runs f and turns a panic into an error.
func Safely(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return f()
}

// ModTime is when the file at p was changed (zero when it is not there).
func ModTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// readError is the report file that could not be read (not a report that
// could not be parsed).
type readError struct{ err error }

func (e *readError) Error() string { return e.err.Error() }
