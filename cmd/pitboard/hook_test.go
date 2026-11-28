package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

func TestHookTellsProblems(t *testing.T) {
	st := badStore(t)
	for _, ev := range []string{"post-tool", "stop"} {
		st.Append(board.Event{Op: "thought", Text: ev})
		sendAll(st)
		got := runHook(t, ev, hookInput(st.Base, "s1", ""))
		if !strings.Contains(got, "report.md has 2 problem(s). Run: pitboard -s s1 check") {
			t.Errorf("%s: %q", ev, got)
		}
	}
	// Nothing new: still nothing at all.
	if got := runHook(t, "post-tool", hookInput(st.Base, "s1", "")); got != "" {
		t.Errorf("nothing new: %q", got)
	}
}

func TestStopHookIgnoresTakenBack(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(board.Event{Op: "thought", Text: "x"})
	st.Append(board.Event{Op: "retract", Thought: th.N})
	if got := runHook(t, "stop", hookInput(st.Base, "s1", "")); got != "" {
		t.Errorf("stop woke the AI for nothing: %q", got)
	}
}

// A retract the hooks deliver comes with the pointer to the topic that
// explains it; an ordinary thought does not.
func TestHookPointsAtHelpChanges(t *testing.T) {
	const pointer = "(what these lines mean: pitboard help changes)"
	for _, ev := range []string{"post-tool", "stop"} {
		t.Run(ev, func(t *testing.T) {
			st := testStore(t)
			in := hookInput(st.Base, "s1", "")
			th, _ := st.Append(board.Event{Op: "thought", Text: "x"})
			sendAll(st)
			if got := runHook(t, ev, in); !strings.Contains(got, "thought 1: x") || strings.Contains(got, pointer) {
				t.Errorf("thought: %q", got)
			}
			st.Append(board.Event{Op: "retract", Thought: th.N})
			sendAll(st)
			if got := runHook(t, ev, in); !strings.Contains(got, "retract thought 1: x") || !strings.Contains(got, pointer) {
				t.Errorf("retract: %q", got)
			}
		})
	}
}

// ---- hook ----

func hookInput(cwd, session string, extra string) string {
	b, _ := json.Marshal(map[string]any{"session_id": session, "cwd": cwd, "hook_event_name": "x"})
	s := string(b)
	if extra != "" {
		s = strings.TrimSuffix(s, "}") + "," + extra + "}"
	}
	return s
}

// runHook runs a hook that ends by itself and returns what it gives the AI:
// post-tool's stdout, or what stop wrote to stderr when it woke the AI (exit
// 2). stop gets a context that is already done: it delivers what is there
// now, or ends quietly instead of waiting.
func runHook(t *testing.T, event, input string) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := hookOut(ctx, event, input)
	if err != nil {
		t.Fatalf("hook %s: %v", event, err)
	}
	return out
}

// hookOut runs a hook. What stop writes to stderr comes with exit code 2
// and nothing on stdout, and nothing is written without it: anything else
// is an error.
func hookOut(ctx context.Context, event, input string) (string, error) {
	var out, errOut strings.Builder
	err := hook(ctx, event, strings.NewReader(input), &out, &errOut)
	var ex exitError
	switch {
	case errors.As(err, &ex):
		if event != "stop" || int(ex) != 2 || out.Len() != 0 || errOut.Len() == 0 {
			return "", errors.New("exit " + strconv.Itoa(int(ex)) + ", stdout " + strconv.Quote(out.String()) + ", stderr " + strconv.Quote(errOut.String()))
		}
		return errOut.String(), nil
	case err != nil:
		return "", err
	case errOut.Len() != 0:
		return "", errors.New("exit 0 with stderr " + strconv.Quote(errOut.String()))
	}
	return out.String(), nil
}

// waitingPath is the board's waiting file.
func waitingPath(st *board.Store) string { return filepath.Join(st.Dir, "waiting") }

func TestHookWithoutBoardDoesNothing(t *testing.T) {
	dir := t.TempDir()
	for _, in := range []string{
		hookInput(dir, "nosuch", ""),
		"not json",
		"",
		`{"cwd":"` + dir + `"}`,
		hookInput(dir, "../x", ""),
		hookInput("", "s1", ""),
	} {
		for _, ev := range []string{"post-tool", "stop", "prompt-submit", "bogus"} {
			if got := runHook(t, ev, in); got != "" {
				t.Errorf("%s with %q printed %q", ev, in, got)
			}
		}
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("hook created files: %v", ents)
	}
	// The whole command, as Claude Code runs it, exits 0 too.
	hookStdin = strings.NewReader(hookInput(dir, "nosuch", ""))
	defer func() { hookStdin = os.Stdin }()
	var out strings.Builder
	if err := run([]string{"hook", "stop"}, &out); err != nil || out.Len() != 0 {
		t.Errorf("run hook: %v %q", err, out.String())
	}
}

// The whole command, as Claude Code runs it: the stop hook wakes the AI by
// exit code 2 with the message on stderr, and prints no error line of its
// own (main exits with an exitError's code without a word).
func TestStopHookExitsTwo(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "x"})
	sendAll(st)
	var errBuf strings.Builder
	hookStdin, hookStderr = strings.NewReader(hookInput(st.Base, "s1", "")), &errBuf
	defer func() { hookStdin, hookStderr = os.Stdin, os.Stderr }()
	var out strings.Builder
	err := run([]string{"hook", "stop"}, &out)
	var ex exitError
	if !errors.As(err, &ex) || int(ex) != 2 || out.Len() != 0 {
		t.Fatalf("run hook stop: %v, stdout %q", err, out.String())
	}
	if !strings.HasPrefix(errBuf.String(), "pitboard (the person acted on the board):\nthought 1: x\n") {
		t.Errorf("stderr %q", errBuf.String())
	}
}

// prompt-submit is gone: nothing is delivered through it.
func TestHookPromptSubmitIsGone(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "x"})
	sendAll(st)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := hookOut(ctx, "prompt-submit", hookInput(st.Base, "s1", "")); err == nil || !strings.Contains(err.Error(), "unknown event") || out != "" {
		t.Errorf("prompt-submit: %v %q", err, out)
	}
	if got := runHook(t, "post-tool", hookInput(st.Base, "s1", "")); !strings.Contains(got, "thought 1: x") {
		t.Errorf("the thought must still be there: %q", got)
	}
	if err := run([]string{"hook"}, new(strings.Builder)); err == nil || strings.Contains(err.Error(), "prompt-submit") {
		t.Errorf("hook usage: %v", err)
	}
	if strings.Contains(usage, "prompt-submit") || strings.Contains(usage, "UserPromptSubmit") {
		t.Error("-h still names prompt-submit")
	}
}

// With nothing sent, the stop hook waits as `wait` does (the waiting file is
// its own) and wakes the AI with the next send that needs it.
func TestStopHookWaitsForSend(t *testing.T) {
	st := testStore(t)
	type res struct {
		out string
		err error
	}
	done := make(chan res, 1)
	go func() {
		out, err := hookOut(context.Background(), "stop", hookInput(st.Base, "s1", ""))
		done <- res{out, err}
	}()
	waitFor(t, "the hook to wait", func() bool { return st.Waiter() == os.Getpid() })
	// What is not sent yet does not wake the AI.
	st.Append(board.Event{Op: "thought", Text: "later"})
	st.Append(board.Event{Op: "hide", Key: "-|x", Title: "x"})
	select {
	case r := <-done:
		t.Fatalf("ended before the send: %q %v", r.out, r.err)
	default:
	}
	sendAll(st)
	select {
	case r := <-done:
		if r.err != nil || !strings.Contains(r.out, "thought 1: later") || !strings.Contains(r.out, "hide (1): x") {
			t.Errorf("delivered %q %v", r.out, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the send did not end the hook")
	}
	if _, err := os.Stat(waitingPath(st)); !os.IsNotExist(err) {
		t.Error("waiting file left behind")
	}
}

// A signal (Claude Code ends the hook at its timeout) ends it quietly, with
// no waiting file left.
func TestStopHookEndsQuietlyOnSignal(t *testing.T) {
	st := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var out string
	go func() {
		var err error
		out, err = hookOut(ctx, "stop", hookInput(st.Base, "s1", ""))
		done <- err
	}()
	waitFor(t, "the hook to wait", func() bool { return st.Waiter() == os.Getpid() })
	cancel()
	select {
	case err := <-done:
		if err != nil || out != "" {
			t.Errorf("ended with %q %v", out, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the hook did not end")
	}
	if _, err := os.Stat(waitingPath(st)); !os.IsNotExist(err) {
		t.Error("waiting file left behind")
	}
}

// Another `wait` or hook waits on the session already: the stop hook ends
// silently and leaves what was sent to it, but still leaves its marker.
func TestStopHookLeavesItToAnotherWaiter(t *testing.T) {
	st := testStore(t)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	os.WriteFile(waitingPath(st), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	st.Append(board.Event{Op: "thought", Text: "x"})
	sendAll(st)
	// Not cancelled: it must end by itself.
	out, err := hookOut(context.Background(), "stop", hookInput(st.Base, "s1", ""))
	if err != nil || out != "" {
		t.Errorf("stop beside another waiter: %q %v", out, err)
	}
	if !st.StopHook() {
		t.Error("no marker")
	}
	if b, _ := os.ReadFile(waitingPath(st)); strings.TrimSpace(string(b)) != strconv.Itoa(cmd.Process.Pid) {
		t.Errorf("the waiting file was touched: %q", b)
	}
	if got := runHook(t, "post-tool", hookInput(st.Base, "s1", "")); !strings.Contains(got, "thought 1: x") {
		t.Errorf("the thought must still be there: %q", got)
	}
}

// Each run of the stop hook on a session with a board leaves the marker
// that tells `wait` it is not needed; post-tool does not.
func TestStopHookLeavesMarker(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.Base, "s1", "")
	runHook(t, "post-tool", in)
	if st.StopHook() {
		t.Error("post-tool left the marker")
	}
	runHook(t, "stop", in)
	if !st.StopHook() {
		t.Error("stop left no marker")
	}
	// The marker is nothing to the board: no problem, nothing to deliver.
	t.Chdir(st.Base)
	for _, cmd := range []string{"check", "changes"} {
		var out strings.Builder
		if err := run([]string{"-s", "s1", cmd}, &out); err != nil || out.Len() != 0 {
			t.Errorf("%s with the marker: %v %q", cmd, err, out.String())
		}
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); !ok(); {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHookOutputs(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.Base, "s1", "")
	const settleLine = `Settle each thought with "pitboard -s s1 settle N [--doing|--hold|--done] TEXT" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.`

	// Nothing to deliver: nothing printed.
	for _, ev := range []string{"post-tool", "stop"} {
		if got := runHook(t, ev, in); got != "" {
			t.Errorf("%s with nothing new: %q", ev, got)
		}
	}

	st.Append(board.Event{Op: "react", ID: "hook", Value: "a", Title: "hook を入れるか決める", Option: "入れる"})
	// Not sent yet: nothing is delivered.
	for _, ev := range []string{"post-tool", "stop"} {
		if got := runHook(t, ev, in); got != "" {
			t.Errorf("%s delivered before send: %q", ev, got)
		}
	}
	sendAll(st)
	// Sent and not delivered: stop wakes the AI at once, with the message
	// as it is (stderr, exit 2: see hookOut).
	got := runHook(t, "stop", in)
	want := "pitboard (the person acted on the board):\nanswer hook = a (入れる)\n" + settleLine + "\n"
	if got != want {
		t.Errorf("stop:\n%q\nwant\n%q", got, want)
	}
	if again := runHook(t, "stop", in); again != "" {
		t.Errorf("delivered twice: %q", again)
	}
	if _, err := os.Stat(waitingPath(st)); !os.IsNotExist(err) {
		t.Error("waiting file left behind")
	}

	st.Append(board.Event{Op: "thought", Text: "経過は要らないかも"})
	sendAll(st)
	var post struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(runHook(t, "post-tool", in)), &post); err != nil {
		t.Fatal(err)
	}
	if post.HookSpecificOutput.HookEventName != "PostToolUse" ||
		!strings.Contains(post.HookSpecificOutput.AdditionalContext, "thought 3: 経過は要らないかも\n"+settleLine) {
		t.Errorf("post-tool: %+v", post)
	}

	// stop_hook_active means nothing to the stop hook: it wakes an AI that
	// has stopped, and keeps none from stopping.
	st.Append(board.Event{Op: "thought", Text: "止まったあとに"})
	sendAll(st)
	got = runHook(t, "stop", hookInput(st.Base, "s1", `"stop_hook_active":true`))
	if !strings.Contains(got, "thought 5: 止まったあとに") || strings.Contains(got, "decision") {
		t.Errorf("stop with stop_hook_active: %q", got)
	}
}

// A board made at the top of a repository is found from below it, by the
// hook and by the commands; a new board goes where the boards are.
func TestBoardFoundFromBelow(t *testing.T) {
	st := testStore(t)
	sub := filepath.Join(st.Base, "a", "b")
	os.MkdirAll(sub, 0o755)
	st.Append(board.Event{Op: "thought", Text: "下から"})
	sendAll(st)

	if got := runHook(t, "post-tool", hookInput(sub, "s1", "")); !strings.Contains(got, "thought 1: 下から") {
		t.Errorf("hook from a subdirectory: %q", got)
	}
	st.Append(board.Event{Op: "thought", Text: "もう一つ"})
	sendAll(st)
	t.Chdir(sub)
	var out strings.Builder
	if err := run([]string{"-s", "s1", "changes"}, &out); err != nil || !strings.Contains(out.String(), "thought 3: もう一つ") {
		t.Errorf("changes from a subdirectory: %v %q", err, out.String())
	}
	out.Reset()
	if err := run([]string{"-s", "s", "settle", "3", "見送った"}, &out); err != nil {
		t.Errorf("settle by prefix from a subdirectory: %v", err)
	}
	out.Reset()
	if err := run([]string{"-s", "new1", "path"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(st.Base, ".pitboard", "new1", "report.md"); strings.TrimSpace(out.String()) != want {
		t.Errorf("new board at %q, want %q", out.String(), want)
	}
	if _, err := os.Stat(filepath.Join(sub, ".pitboard")); err == nil {
		t.Error("path made a second .pitboard/ in the subdirectory")
	}
}

func TestHookInputLimit(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "x"})
	sendAll(st)
	in := hookInput(st.Base, "s1", `"tool_response":"`+strings.Repeat("a", hookInputLimit)+`"`)
	if got := runHook(t, "post-tool", in); got != "" {
		t.Errorf("input over the limit was read: %q", got[:min(len(got), 80)])
	}
	if got := runHook(t, "post-tool", hookInput(st.Base, "s1", "")); got == "" {
		t.Error("the thought must still be there")
	}
}

func TestHookOnUnwritableBoard(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	st := testStore(t)
	os.Chmod(st.Dir, 0o555)
	defer os.Chmod(st.Dir, 0o755)
	// Not cancelled: an unwritable board ends the stop hook by itself.
	for _, ev := range []string{"post-tool", "stop"} {
		if got, err := hookOut(context.Background(), ev, hookInput(st.Base, "s1", "")); got != "" || err != nil {
			t.Errorf("%s: %q %v", ev, got, err)
		}
	}
	for _, p := range []string{st.EventsPath(), st.StopHookPath(), waitingPath(st)} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s was created", filepath.Base(p))
		}
	}
	// Written but unreadable: still nothing, and no error; the stop hook
	// does not wait on it.
	os.Chmod(st.Dir, 0o755)
	st.Append(board.Event{Op: "thought", Text: "x"})
	sendAll(st)
	os.Chmod(st.EventsPath(), 0o000)
	defer os.Chmod(st.EventsPath(), 0o644)
	for _, ev := range []string{"post-tool", "stop"} {
		if got, err := hookOut(context.Background(), ev, hookInput(st.Base, "s1", "")); got != "" || err != nil {
			t.Errorf("%s with unreadable events: %q %v", ev, got, err)
		}
	}
	if _, err := os.Stat(waitingPath(st)); !os.IsNotExist(err) {
		t.Error("waiting file left behind")
	}
}

func TestStopDoesNotWakeForHides(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.Base, "s1", "")
	st.Append(board.Event{Op: "hide", Key: "-|x", Title: "x"})
	sendAll(st)
	if got := runHook(t, "stop", in); got != "" {
		t.Errorf("stop woke the AI for a hide: %q", got)
	}
	// The hide waits for the next delivery.
	st.Append(board.Event{Op: "hide", Key: "-|y", Title: "y"})
	st.Append(board.Event{Op: "react", ID: "hook", Value: "a", Option: "入れる"})
	sendAll(st)
	if got := runHook(t, "stop", in); !strings.Contains(got, "answer hook = a") || !strings.Contains(got, "hide (2): x / y") {
		t.Errorf("stop with an answer: %q", got)
	}
}

// Claude Code hands a hook the whole session id, while the help tells the AI
// to name its board by the first 8 characters: the hook finds that board.
func TestHookFindsBoardNamedByIDHead(t *testing.T) {
	const id = "0a1b2c3d-4e5f-6789-abcd-ef0123456789"
	mk := func(base, name string) *board.Store {
		st := board.Open(base, name)
		if err := os.MkdirAll(st.Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(st.ReportPath(), []byte("<Title>t</Title>\n"), 0o644)
		return st
	}
	for _, ev := range []string{"post-tool", "stop"} {
		base := t.TempDir()
		st := mk(base, "0a1b2c3d")
		st.Append(board.Event{Op: "thought", Text: "by head"})
		sendAll(st)
		// From a directory below the board, as after a cd.
		sub := filepath.Join(base, "a", "b")
		os.MkdirAll(sub, 0o755)
		got := runHook(t, ev, hookInput(sub, id, ""))
		if !strings.Contains(got, "by head") {
			t.Errorf("%s: not delivered: %q", ev, got)
		}
		// The command it gives names the board as it is named.
		if !strings.Contains(got, "pitboard -s 0a1b2c3d settle") {
			t.Errorf("%s: settle line: %q", ev, got)
		}
		if again := runHook(t, ev, hookInput(sub, id, "")); again != "" {
			t.Errorf("%s: delivered twice: %q", ev, again)
		}
	}

	// A head shorter than 8 characters, or another session's board, is not this session's.
	base := t.TempDir()
	for _, name := range []string{"0a1b", "0a1b2c3e", "0a1b2c3d-4e5f-0000"} {
		st := mk(base, name)
		st.Append(board.Event{Op: "thought", Text: "other"})
		sendAll(st)
	}
	if got := runHook(t, "post-tool", hookInput(base, id, "")); got != "" {
		t.Errorf("delivered another board's: %q", got)
	}

	// The id itself wins over a head; among heads, the longest.
	short := mk(base, "0a1b2c3d")
	short.Append(board.Event{Op: "thought", Text: "short head"})
	sendAll(short)
	long := mk(base, "0a1b2c3d-4e5f")
	long.Append(board.Event{Op: "thought", Text: "long head"})
	sendAll(long)
	if got := runHook(t, "post-tool", hookInput(base, id, "")); !strings.Contains(got, "long head") || strings.Contains(got, "short head") {
		t.Errorf("longest head: %q", got)
	}
	whole := mk(base, id)
	whole.Append(board.Event{Op: "thought", Text: "whole id"})
	sendAll(whole)
	if got := runHook(t, "post-tool", hookInput(base, id, "")); !strings.Contains(got, "whole id") {
		t.Errorf("whole id: %q", got)
	}
}
