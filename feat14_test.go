package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const badReport = "<Item id=\"a\" wait=\"me\">x</Item>\n<Item id=\"b\" wait=\"me\">y</Item>\n"

func badStore(t *testing.T) *Store {
	t.Helper()
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(badReport), 0o644)
	if _, probs, _ := parseReport(badReport); len(probs) != 2 {
		t.Fatalf("the bad report has %d problems", len(probs))
	}
	return st
}

const wantNote = "report.md has 2 problem(s). Run: pitboard -s s1 check\n"

func inDir(t *testing.T, dir string) {
	t.Helper()
	prev, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(prev) })
}

func TestCommandsTellProblems(t *testing.T) {
	st := badStore(t)
	inDir(t, st.base)
	st.Append(Event{Op: "thought", Text: "one"})
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
	os.WriteFile(st.reportPath(), []byte(`<Item id="a">x</Item>`), 0o644)
	out.Reset()
	run([]string{"-s", "s1", "changes"}, &out)
	run([]string{"-s", "s1", "settle", "1", "done"}, &out)
	if out.Len() != 0 {
		t.Errorf("printed without problems: %q", out.String())
	}
	// No report, or one that cannot be read: nothing, no crash.
	os.Remove(st.reportPath())
	if n := st.problemNote(); n != "" {
		t.Errorf("no report: %q", n)
	}
	os.Mkdir(st.reportPath(), 0o755)
	if n := st.problemNote(); n != "" {
		t.Errorf("unreadable report: %q", n)
	}
}

func TestWaitTellsProblems(t *testing.T) {
	shortPoll(t)
	st := badStore(t)
	var errBuf bytes.Buffer
	errOut = &errBuf
	t.Cleanup(func() { errOut = os.Stderr })

	// Timeout.
	var out bytes.Buffer
	if err := st.Wait(context.Background(), 50*time.Millisecond, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != wantNote || errBuf.String() != wantNote {
		t.Errorf("timeout: out %q err %q", out.String(), errBuf.String())
	}
	// Delivered.
	out.Reset()
	errBuf.Reset()
	st.Append(Event{Op: "thought", Text: "one"})
	sendAll(st)
	if err := st.Wait(context.Background(), time.Second, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "thought 1: one\n"+wantNote || errBuf.String() != wantNote {
		t.Errorf("delivered: out %q err %q", out.String(), errBuf.String())
	}
	// Ended by a signal: nothing added.
	out.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st.Wait(ctx, 0, &out)
	if out.Len() != 0 {
		t.Errorf("cancelled: %q", out.String())
	}
}

func TestHookTellsProblems(t *testing.T) {
	st := badStore(t)
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
		st.Append(Event{Op: "thought", Text: ev})
		sendAll(st)
		got := runHook(t, ev, hookInput(st.base, "s1", ""))
		if !strings.Contains(got, "report.md has 2 problem(s). Run: pitboard -s s1 check") {
			t.Errorf("%s: %q", ev, got)
		}
	}
	// Nothing new: still nothing at all.
	if got := runHook(t, "post-tool", hookInput(st.base, "s1", "")); got != "" {
		t.Errorf("nothing new: %q", got)
	}
}
