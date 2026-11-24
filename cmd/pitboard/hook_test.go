package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

func TestHookTellsProblems(t *testing.T) {
	st := badStore(t)
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
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
		t.Errorf("stop blocked for nothing: %q", got)
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

func runHook(t *testing.T, event, input string) string {
	t.Helper()
	var out strings.Builder
	if err := hook(event, strings.NewReader(input), &out); err != nil {
		t.Fatalf("hook %s: %v", event, err)
	}
	return out.String()
}

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
		for _, ev := range []string{"prompt-submit", "post-tool", "stop", "bogus"} {
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

func TestHookOutputs(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.Base, "s1", "")
	const settleLine = `Settle each thought with "pitboard -s s1 settle N [--doing|--hold|--done] TEXT" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.`

	// Nothing to deliver: nothing printed.
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
		if got := runHook(t, ev, in); got != "" {
			t.Errorf("%s with nothing new: %q", ev, got)
		}
	}

	st.Append(board.Event{Op: "react", ID: "hook", Value: "a", Title: "hook を入れるか決める", Option: "入れる"})
	// Not sent yet: nothing is delivered.
	if got := runHook(t, "prompt-submit", in); got != "" {
		t.Errorf("delivered before send: %q", got)
	}
	sendAll(st)
	got := runHook(t, "prompt-submit", in)
	want := "pitboard (the person acted on the board):\nanswer hook = a (入れる)\n" + settleLine + "\n"
	if got != want {
		t.Errorf("prompt-submit:\n%q\nwant\n%q", got, want)
	}
	if again := runHook(t, "prompt-submit", in); again != "" {
		t.Errorf("delivered twice: %q", again)
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

	st.Append(board.Event{Op: "thought", Text: "止まる前に"})
	sendAll(st)
	// Already kept from stopping once: no block, and nothing is used up.
	if got := runHook(t, "stop", hookInput(st.Base, "s1", `"stop_hook_active":true`)); got != "" {
		t.Errorf("stop with stop_hook_active: %q", got)
	}
	var stop struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(runHook(t, "stop", in)), &stop); err != nil {
		t.Fatal(err)
	}
	if stop.Decision != "block" || !strings.Contains(stop.Reason, "thought 5: 止まる前に") {
		t.Errorf("stop: %+v", stop)
	}
	if got := runHook(t, "stop", in); got != "" {
		t.Errorf("stop delivered twice: %q", got)
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
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
		if got := runHook(t, ev, hookInput(st.Base, "s1", "")); got != "" {
			t.Errorf("%s: %q", ev, got)
		}
	}
	if _, err := os.Stat(st.EventsPath()); err == nil {
		t.Error("events.jsonl was created")
	}
	// Written but unreadable: still nothing, and no error.
	os.Chmod(st.Dir, 0o755)
	st.Append(board.Event{Op: "thought", Text: "x"})
	os.Chmod(st.EventsPath(), 0o000)
	defer os.Chmod(st.EventsPath(), 0o644)
	if got := runHook(t, "post-tool", hookInput(st.Base, "s1", "")); got != "" {
		t.Errorf("unreadable events: %q", got)
	}
}

func TestStopDoesNotBlockForHides(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.Base, "s1", "")
	st.Append(board.Event{Op: "hide", Key: "-|x", Title: "x"})
	sendAll(st)
	if got := runHook(t, "stop", in); got != "" {
		t.Errorf("stop blocked for a hide: %q", got)
	}
	if got := runHook(t, "prompt-submit", in); !strings.Contains(got, "hide (1): x") {
		t.Errorf("the hide must wait for the next delivery: %q", got)
	}
	st.Append(board.Event{Op: "hide", Key: "-|y", Title: "y"})
	st.Append(board.Event{Op: "react", ID: "hook", Value: "a", Option: "入れる"})
	sendAll(st)
	if got := runHook(t, "stop", in); !strings.Contains(got, "block") || !strings.Contains(got, "hide (1): y") {
		t.Errorf("stop with an answer: %q", got)
	}
}
