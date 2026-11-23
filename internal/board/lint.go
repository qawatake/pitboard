package board

import (
	"fmt"
	"io"
	"os"

	"github.com/qawatake/pitboard/internal/report"
)

// errOut is where commands say things that are not their output (tests
// replace it).
var errOut io.Writer = os.Stderr

// ProblemNote is the line every command the AI runs ends with while the
// report has problems or warnings (rows removed that the person still has
// business with, see Removed), so that the AI learns of them without
// running 'check' itself. A removed row is counted on it once (the next
// command after it went); 'check' lists every warning each time. It is empty when there are none, or the report
// does not exist or cannot be read.
func (s *Store) ProblemNote() (note string) {
	defer func() {
		if recover() != nil {
			note = ""
		}
	}()
	b, err := os.ReadFile(s.ReportPath())
	if err != nil {
		return ""
	}
	_, probs, err := report.Parse(string(b))
	if err != nil {
		return ""
	}
	_, warns := s.removed(true)
	if len(probs) == 0 && warns == 0 {
		return ""
	}
	return fmt.Sprintf("report.md has %d problem(s), %d warning(s). Run: pitboard -s %s check", len(probs), warns, ShortSession(s))
}

// NoteProblems prints ProblemNote on its own line, if there is one.
func (s *Store) NoteProblems(w io.Writer) {
	if n := s.ProblemNote(); n != "" {
		fmt.Fprintln(w, n)
	}
}
