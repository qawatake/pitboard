package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

func TestSettleMany(t *testing.T) {
	st := testStore(t)
	for range 9 {
		st.Append(board.Event{Op: "thought", Text: "t"})
	}
	t.Chdir(st.Base)
	var out bytes.Buffer
	if err := run([]string{"-s", "s1", "settle", "--done", "1", "todo にした", "--hold", "3", "答えた", "あとで"}, &out); err != nil {
		t.Fatal(err)
	}
	// Without a state first: one thought, and every word is its text.
	if err := run([]string{"-s", "s1", "settle", "9", "fixed", "5", "bugs"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-s", "s1", "settle", "2", "--doing", "made", "it", "a", "todo"}, &out); err != nil {
		t.Fatal(err)
	}
	s := board.Replay(st.Events(), 0)
	got := []string{}
	for _, n := range []int{1, 2, 3, 5, 9} {
		for _, th := range s.Thoughts {
			if th.N == n {
				got = append(got, th.SettledState+":"+th.Settled)
			}
		}
	}
	if strings.Join(got, "|") != "done:todo にした|doing:made it a todo|hold:答えた あとで|:|done:fixed 5 bugs" {
		t.Errorf("settled %q", got)
	}
	// Errors, and nothing recorded.
	n := len(st.Events())
	for _, args := range [][]string{
		{"--done", "1", "x", "--done", "99", "y"}, // no thought 99
		{"5", "--doing", "--hold", "x"},           // two states
		{"--doing", "5", "--hold", "x"},           // two states
		{"1", "x", "--done", "2", "y"},            // several without a state first
		{"x"},
	} {
		err := run(append([]string{"-s", "s1", "settle"}, args...), &out)
		if err == nil || len(st.Events()) != n {
			t.Errorf("%q: %v, %d events", args, err, len(st.Events())-n)
		}
	}
	err := run([]string{"-s", "s1", "settle", "5", "--doing", "--hold", "x"}, &out)
	if err == nil || !strings.Contains(err.Error(), "two states") {
		t.Errorf("two states: %v", err)
	}
}

func TestSettleStates(t *testing.T) {
	st := testStore(t)
	for range 3 {
		st.Append(board.Event{Op: "thought", Text: "t"})
	}
	t.Chdir(st.Base)
	var out bytes.Buffer
	for _, args := range [][]string{
		{"settle", "1", "--doing", "調べています"},
		{"settle", "--hold", "2"},
		{"settle", "--doing", "3", "a", "--done", "1", "済んだ"},
	} {
		if err := run(append([]string{"-s", "s1"}, args...), &out); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}
	s := board.Replay(st.Events(), 0)
	var got []string
	words := map[board.Status]string{board.StatusDone: "対応済", board.StatusHold: "保留", board.StatusDoing: "対応中"}
	for _, th := range s.Thoughts {
		w := words[s.StatusOf(th)]
		got = append(got, w+":"+th.Settled)
	}
	if strings.Join(got, "|") != "対応済:済んだ|保留:|対応中:a" {
		t.Errorf("states %q", got)
	}
	if err := run([]string{"-s", "s1", "settle", "2", "--done"}, &out); err == nil {
		t.Error("done without a text was accepted")
	}
}

func inDir(t *testing.T, dir string) {
	t.Helper()
	prev, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(prev) })
}

func TestCommandsTellProblems(t *testing.T) {
	st := badStore(t)
	inDir(t, st.Base)
	st.Append(board.Event{Op: "thought", Text: "one"})
	sendAll(st)
	var out bytes.Buffer
	if err := run([]string{"-s", "s1", "changes"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "thought 1: one\n"+wantNote {
		t.Errorf("changes: %q", out.String())
	}
	out.Reset()
	if err := run([]string{"-s", "s1", "changes"}, &out); err != nil || out.String() != wantNote {
		t.Errorf("changes with nothing new: %v %q", err, out.String())
	}
	out.Reset()
	if err := run([]string{"-s", "s1", "settle", "1", "done"}, &out); err != nil || out.String() != wantNote {
		t.Errorf("settle: %v %q", err, out.String())
	}
	// The exit code stays: a failed settle still fails.
	out.Reset()
	if err := run([]string{"-s", "s1", "settle", "99", "x"}, &out); err == nil || out.String() != wantNote {
		t.Errorf("failed settle: %v %q", err, out.String())
	}

	// Fixed: nothing is added.
	os.WriteFile(st.ReportPath(), []byte(`<Item id="a">x</Item>`), 0o644)
	out.Reset()
	run([]string{"-s", "s1", "changes"}, &out)
	run([]string{"-s", "s1", "settle", "1", "done"}, &out)
	if out.Len() != 0 {
		t.Errorf("printed without problems: %q", out.String())
	}
	// No report, or one that cannot be read: nothing, no crash.
	os.Remove(st.ReportPath())
	if n := st.ProblemNote(); n != "" {
		t.Errorf("no report: %q", n)
	}
	os.Mkdir(st.ReportPath(), 0o755)
	if n := st.ProblemNote(); n != "" {
		t.Errorf("unreadable report: %q", n)
	}
}

func TestWaitArgs(t *testing.T) {
	st := testStore(t)
	for _, args := range [][]string{{"--timeout"}, {"--timeout", "x"}, {"--bogus"}} {
		if err := runWait(st, args, &bytes.Buffer{}); err == nil {
			t.Errorf("%q: no error", args)
		}
	}
	if err := runWait(st, []string{"--timeout=1ms"}, &bytes.Buffer{}); err != nil {
		t.Error(err)
	}
}

func TestSessionFlagOnlyNextToTheCommand(t *testing.T) {
	st := testStore(t)
	t.Chdir(st.Base)
	th, _ := st.Append(board.Event{Op: "thought", Text: "x"})
	var out strings.Builder
	if err := run([]string{"settle", "-s", "s1", "1", "see", "-s", "option"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-s", "s1", "settle", "1", "-s", "x"}, &out); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, e := range st.Events() {
		if e.Op == "settle" && e.Thought == th.N {
			texts = append(texts, e.Text)
		}
	}
	if strings.Join(texts, "|") != "see -s option|-s x" {
		t.Errorf("settle texts = %q", texts)
	}
}

func TestNewBoardWithoutAnyHere(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out strings.Builder
	if err := run([]string{"-s", "x1", "path"}, &out); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(filepath.Dir(strings.TrimSpace(out.String())))
	if got != filepath.Join(real, ".pitboard", "x1") {
		t.Errorf("path = %q", out.String())
	}
}

func TestCheckCommand(t *testing.T) {
	st := testStore(t)
	t.Chdir(st.Base)
	var out strings.Builder
	if err := run([]string{"-s", "s1", "check"}, &out); err != nil {
		t.Fatalf("good report: %v %s", err, out.String())
	}
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"a\">x</Item>\n<Item id=\"a\">y</Item>\n"), 0o644)
	out.Reset()
	err := run([]string{"-s", "s1", "check"}, &out)
	if err != exitError(1) || !strings.Contains(out.String(), ":2: duplicate id") {
		t.Errorf("bad report: %v %q", err, out.String())
	}
}

func TestSessionFlagAfterCommand(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	var out strings.Builder
	if err := run([]string{"path", "-s", "abc"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.String()), filepath.Join(".pitboard", "abc", "report.md")) {
		t.Errorf("path = %q", out.String())
	}
	// Writing to a session that does not exist creates nothing.
	if _, err := board.Open(base, "nope").Append(board.Event{Op: "thought", Text: "x"}); err == nil {
		t.Error("appending to a missing session must fail")
	}
	if _, err := os.Stat(filepath.Join(base, ".pitboard", "nope")); err == nil {
		t.Error("a missing session was created")
	}
}

// The numbers of what is still doing or hold end what changes prints, in one
// line before the note on the report's problems; settle does not print it;
// open lists them.
func TestOpenComments(t *testing.T) {
	st := badStore(t)
	t.Chdir(st.Base)
	st.Append(board.Event{Op: "thought", Text: "一つ目"})
	st.Append(board.Event{Op: "thought", Text: "二つ目"})
	sendAll(st)
	cmd := func(args ...string) string {
		var out strings.Builder
		if err := run(append([]string{"-s", "s1"}, args...), &out); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		return out.String()
	}
	if got := cmd("open"); got != "" {
		t.Errorf("open with nothing open: %q", got)
	}
	cmd("changes")
	if got := cmd("settle", "--doing", "1", "見ています", "--hold", "2", "あとで"); strings.Contains(got, "open:") {
		t.Errorf("settle: %q", got)
	}
	// (oldest 0s or 1s: the settle was just now.)
	if got := cmd("changes"); !strings.HasPrefix(got, "open: 1 2 (oldest ") || !strings.HasSuffix(got, "s). Details: pitboard -s s1 open\n"+wantNote) || strings.Count(got, "\n") != 2 {
		t.Errorf("changes:\n%s", got)
	}
	got := cmd("open")
	if !strings.HasPrefix(got, "doing 1 (") || !strings.Contains(got, "\nhold 2 (") || strings.Contains(got, "problem") {
		t.Errorf("open:\n%s", got)
	}
	if got := cmd("settle", "1", "--done", "済んだ"); strings.Contains(got, "open:") {
		t.Errorf("settle done:\n%s", got)
	}
	if got := cmd("changes"); !strings.HasPrefix(got, "open: 2 (") {
		t.Errorf("changes after done:\n%s", got)
	}
	cmd("settle", "2", "--done", "済んだ")
	if got := cmd("open"); got != "" {
		t.Errorf("all done: %q", got)
	}
}
