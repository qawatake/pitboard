package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
)

// hook handles a Claude Code hook event: it delivers what the person did on
// the board that the AI has not been given yet, as `changes` does.
//
//	prompt-submit  with the user's prompt (plain stdout)
//	post-tool      while the AI is working, after its next tool call
//	stop           when the AI is about to stop: it is kept going to read them
//
// The hook runs in every session of every project, so a session without a
// board (no .pitboard/<session_id>/ in the hook's cwd or above it), or input that is
// not what Claude Code sends, gets nothing: no output, no files, exit 0.
func hook(event string, stdin io.Reader, stdout io.Writer) error {
	var in struct {
		SessionID      string `json:"session_id"`
		Cwd            string `json:"cwd"`
		StopHookActive bool   `json:"stop_hook_active"`
	}
	// A hook's input can be large (a tool's whole output); past the limit it
	// is not read at all.
	if err := json.NewDecoder(io.LimitReader(stdin, hookInputLimit)).Decode(&in); err != nil {
		return nil
	}
	if in.Cwd == "" || !board.ReSession.MatchString(in.SessionID) {
		return nil
	}
	// The AI may have cd'ed below where the board is: look upwards.
	base, _, ok, err := board.FindSession(in.Cwd, in.SessionID, false)
	if err != nil || !ok {
		return nil
	}
	st := board.Open(base, in.SessionID)
	switch event {
	case "prompt-submit", "post-tool", "stop":
	default:
		return fmt.Errorf("hook: unknown event %q (prompt-submit, post-tool or stop)", event)
	}
	// The AI was already kept from stopping once for this stop: let it stop,
	// and leave what is new for the next delivery.
	if event == "stop" && in.StopHookActive {
		return nil
	}
	// Hidden lines need nothing from the AI: they do not keep it from
	// stopping, and wait to go with the next delivery.
	if event == "stop" && !needsAI(st.Pending()) {
		return nil
	}
	// A board that cannot be read or written gets nothing either: a hook
	// failing on every tool call would make the session hard to use.
	lines, err := st.Changes()
	if err != nil || len(lines) == 0 {
		return nil
	}
	msg := "pitboard (the person acted on the board):\n" + strings.Join(lines, "\n") + "\n" +
		fmt.Sprintf("Settle each thought with \"pitboard -s %s settle N [--doing|--hold|--done] TEXT\" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.", in.SessionID)
	if n := st.ProblemNote(); n != "" {
		msg += "\n" + n
	}
	enc := json.NewEncoder(stdout)
	switch event {
	case "prompt-submit":
		_, err = fmt.Fprintln(stdout, msg)
	case "post-tool":
		err = enc.Encode(map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PostToolUse", "additionalContext": msg}})
	case "stop":
		err = enc.Encode(map[string]any{"decision": "block", "reason": msg})
	}
	return err
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
