package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
)

// hook handles a Claude Code hook event: it delivers what the person did on
// the board that the AI has not been given yet, as `changes` does.
//
//	post-tool  while the AI is working, after its next tool call (stdout)
//	stop       after the AI stopped: it waits for the person's send, as
//	           `wait` does, and wakes the AI with it (see stopHook)
//
// The hook runs in every session of every project, so a session without a
// board (no .pitboard/ directory named by the session id or its head in the
// hook's cwd or above it), or input that is
// not what Claude Code sends, gets nothing: no output, no files, exit 0.
func hook(ctx context.Context, event string, stdin io.Reader, stdout, stderr io.Writer) error {
	var in struct {
		SessionID string `json:"session_id"`
		Cwd       string `json:"cwd"`
	}
	// A hook's input can be large (a tool's whole output); past the limit it
	// is not read at all.
	if err := json.NewDecoder(io.LimitReader(stdin, hookInputLimit)).Decode(&in); err != nil {
		return nil
	}
	if in.Cwd == "" || !board.ReSession.MatchString(in.SessionID) {
		return nil
	}
	// The AI may have cd'ed below where the board is: look upwards. The hook
	// is given the whole session id, while the board is named by its head
	// (the first 8 characters, as the help says) or by the id itself.
	base, name, ok := board.FindSessionByID(in.Cwd, in.SessionID)
	if !ok {
		return nil
	}
	st := board.Open(base, name)
	switch event {
	case "post-tool":
		// A board that cannot be read or written gets nothing either: a hook
		// failing on every tool call would make the session hard to use.
		lines, err := st.Changes()
		if err != nil || len(lines) == 0 {
			return nil
		}
		return json.NewEncoder(stdout).Encode(map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PostToolUse", "additionalContext": hookMessage(st, name, lines)}})
	case "stop":
		return stopHook(ctx, st, name, stderr)
	}
	return fmt.Errorf("hook: unknown event %q (post-tool or stop)", event)
}

// stopHook is the Stop hook, meant to run with "asyncRewake": true: Claude
// Code leaves it running after the AI stopped, and when it exits with code 2
// wakes the AI with what it wrote to stderr. It delivers at once what was
// sent and not delivered yet, or else waits for the next send as `wait`
// does (the same waiting file). Hidden lines need nothing from the AI: a
// send of only those does not wake it, and they go with the next delivery.
//
// It ends silently (exit 0) when another `wait` or hook waits on the session
// already, on a signal (Claude Code ends it at the hook's timeout), and when
// the board cannot be read or written.
//
// Each run leaves the marker that tells `wait` it is not needed here.
func stopHook(ctx context.Context, st *board.Store, name string, stderr io.Writer) error {
	st.MarkStopHook()
	if f, err := os.Open(st.EventsPath()); err == nil {
		f.Close()
	} else if !os.IsNotExist(err) {
		return nil
	}
	var msg string
	end, _, err := st.Await(ctx, 0, nil, func() (bool, error) {
		if !needsAI(st.Pending()) {
			return false, nil
		}
		lines, err := st.Changes()
		if err != nil {
			return false, err
		}
		// Delivered by someone else meanwhile (the post-tool hook, `changes`).
		if len(lines) == 0 {
			return false, nil
		}
		msg = hookMessage(st, name, lines)
		return true, nil
	})
	if err != nil || end != board.WaitDone {
		return nil
	}
	fmt.Fprintln(stderr, msg)
	return exitError(2)
}

// hookMessage is what a hook tells the AI: the lines of `changes`, how to
// settle, and the problem note if there is one.
func hookMessage(st *board.Store, name string, lines []string) string {
	if h := board.ChangesHelp(lines); h != "" {
		lines = append(lines, h)
	}
	msg := "pitboard (the person acted on the board):\n" + strings.Join(lines, "\n") + "\n" +
		fmt.Sprintf("Settle each thought with \"pitboard -s %s settle N [--doing|--hold|--done] TEXT\" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.", name)
	if n := st.ProblemNote(); n != "" {
		msg += "\n" + n
	}
	return msg
}

// hookInputLimit is the most of a hook's input that is read (1 MiB).
const hookInputLimit = 1 << 20

// needsAI says whether any of evs asks something of the AI (hide and unhide
// do not; settle is the AI's own).
func needsAI(evs []board.Event) bool {
	for _, e := range evs {
		switch e.Op {
		case "hide", "unhide", "settle":
		default:
			return true
		}
	}
	return false
}
