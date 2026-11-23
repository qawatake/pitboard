package board

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// errOut is where commands say things that are not their output (tests
// replace it).
var errOut io.Writer = os.Stderr

// ProblemNote is what every command the AI runs (other than 'check')
// ends with: the warnings about rows removed that the person still had
// business with (see Removed), each said once and then forgotten, and,
// while the report has problems, the line saying so, so that the AI learns
// of them without running 'check' itself. It is empty when there are none,
// or the report does not exist or cannot be read.
func (s *Store) ProblemNote() string { return s.problemNote(true) }

func (s *Store) problemNote(removed bool) (note string) {
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
	var lines []string
	if removed {
		lines = s.TakeRemoved()
	}
	if len(probs) > 0 {
		lines = append(lines, fmt.Sprintf("report.md has %d problem(s). Run: pitboard -s %s check", len(probs), ShortSession(s)))
	}
	return strings.Join(lines, "\n")
}

// NoteProblems prints ProblemNote, if there is one.
func (s *Store) NoteProblems(w io.Writer) {
	if n := s.ProblemNote(); n != "" {
		fmt.Fprintln(w, n)
	}
}

// NoteReportProblems prints only the problem line of ProblemNote: the
// removed rows are left to be said where the AI reads them.
func (s *Store) NoteReportProblems(w io.Writer) {
	if n := s.problemNote(false); n != "" {
		fmt.Fprintln(w, n)
	}
}
