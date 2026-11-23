package main

import (
	"fmt"
	"io"
	"os"
)

// errOut is where commands say things that are not their output (tests
// replace it).
var errOut io.Writer = os.Stderr

// problemNote is the line every command the AI runs ends with while the
// report has problems, so that the AI learns of them without running
// 'check' itself. It is empty when the report has none, does not exist or
// cannot be read.
func (s *Store) problemNote() (note string) {
	defer func() {
		if recover() != nil {
			note = ""
		}
	}()
	b, err := os.ReadFile(s.reportPath())
	if err != nil {
		return ""
	}
	_, probs, err := parseReport(string(b))
	if err != nil || len(probs) == 0 {
		return ""
	}
	return fmt.Sprintf("report.md has %d problem(s). Run: pitboard -s %s check", len(probs), shortSession(s))
}

// noteProblems prints problemNote on its own line, if there is one.
func (s *Store) noteProblems(w io.Writer) {
	if n := s.problemNote(); n != "" {
		fmt.Fprintln(w, n)
	}
}
