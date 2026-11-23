package page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// Send sends what the person has not sent: marks it all as sent (the AI
// gets it from 'changes'), then wakes the AI. progress is told what to
// show while waking takes time (it may take seconds). The message says how
// it went; when it is in two parts (what happened, then why and how to fix
// it) they are split by "\n".
func (b *Board) Send(progress func(msg string)) (string, error) {
	if !b.CanWrite() {
		return "", errCannotWrite
	}
	b.Load()
	n := b.Unsent
	if n == 0 {
		return "未送信のものはありません", nil
	}
	// The mark comes first: what is above it is what the AI gets from
	// 'changes', whether or not it can be woken now.
	if _, err := b.Store.Append(board.Event{Op: "send"}); err != nil {
		return "", fmt.Errorf("送れませんでした: %w", err)
	}
	b.Load()
	// A 'pitboard wait' waiting for this send wakes the AI by ending: no
	// terminal is typed into.
	if b.Store.Waiter() != 0 {
		return "送りました", nil
	}
	if progress != nil {
		progress("送っています…")
	}
	err := deliver(b.Store.Session, WakeMessage(b.Store, n))
	if errors.Is(err, errNoTarget) {
		return "送りました。ただ AI が待っていないので、チャットで一言伝えてください。\n" + err.Error(), nil
	}
	if err != nil {
		return "送りました。ただ AI を起こせなかったので、チャットで一言伝えてください。\n" + err.Error(), nil
	}
	return "送りました", nil
}

// Sending wakes the AI from the board. A hook can only deliver while the AI
// is working; when it has stopped, what the person wrote waits until they
// type something in the chat. Sending types one line into the AI's terminal
// saying there is something to read; the AI reads it with 'changes', so what
// has been delivered is still kept in one place (seen).

// SendTimeout is how long sending may take before it is cut off (tests
// shorten it).
var SendTimeout = 5 * time.Second

// errNoTarget means there is no way to wake the AI: no $PITBOARD_SEND and
// no herdr. Its text says how to fix it.
var errNoTarget = errors.New(`AI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")`)

// wakeMessage is the line typed into the AI's terminal. It stays in the
// AI's conversation, so it is short; -h says what it means.
func WakeMessage(st *board.Store, n int) string {
	return fmt.Sprintf("pitboard: %d sent. Run: pitboard -s %s changes", n, board.ShortSession(st))
}

// deliver types msg into the terminal of the AI of session: through
// $PITBOARD_SEND if set, or else through herdr when it is on PATH (the
// viewer itself need not run inside herdr; the AI's agent is looked up by
// its session). The error's text says why it failed and how to fix it.
func deliver(session, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), SendTimeout)
	defer cancel()
	if sh := os.Getenv("PITBOARD_SEND"); sh != "" {
		c := exec.CommandContext(ctx, "sh", "-c", sh)
		c.Env = append(os.Environ(), "PITBOARD_SESSION="+session, "PITBOARD_MESSAGE="+msg)
		if err := runQuiet(ctx, c, "PITBOARD_SEND"); err != nil {
			return fmt.Errorf("送れませんでした: %w", err)
		}
		return nil
	}
	if _, err := exec.LookPath("herdr"); err != nil {
		return errNoTarget
	}
	pane, err := herdrPane(ctx, session)
	if err != nil {
		return err
	}
	if err := runQuiet(ctx, exec.CommandContext(ctx, "herdr", "agent", "prompt", pane, msg), "herdr agent prompt"); err != nil {
		return fmt.Errorf("送れませんでした: %w", err)
	}
	return nil
}

// shortID is the head of a session id, as shown to the person.
func shortID(s string) string {
	if r := []rune(s); len(r) > 8 {
		return string(r[:8])
	}
	return s
}

// herdrPane finds the pane of the agent whose session is session.
func herdrPane(ctx context.Context, session string) (string, error) {
	c := exec.CommandContext(ctx, "herdr", "agent", "list")
	var out bytes.Buffer
	c.Stdout = &out
	if err := runQuiet(ctx, c, "herdr agent list"); err != nil {
		return "", fmt.Errorf("送れませんでした: %w", err)
	}
	var list struct {
		Result struct {
			Agents []struct {
				PaneID       string `json:"pane_id"`
				AgentSession struct {
					Value string `json:"value"`
				} `json:"agent_session"`
			} `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		return "", fmt.Errorf("送れませんでした: herdr agent list: 読めませんでした: %v", err)
	}
	for _, a := range list.Result.Agents {
		if a.AgentSession.Value == session && a.PaneID != "" {
			return a.PaneID, nil
		}
	}
	return "", fmt.Errorf("herdr にこのセッション (%s) の AI がいません。AI を herdr の中で動かすか、PITBOARD_SEND を設定してください", shortID(session))
}

// runQuiet runs c without the terminal (no stdin; output is not shown) and
// turns a failure into an error carrying the first line of its stderr.
func runQuiet(ctx context.Context, c *exec.Cmd, name string) error {
	var stderr bytes.Buffer
	c.Stdin = nil
	if c.Stdout == nil {
		c.Stdout = &bytes.Buffer{}
	}
	c.Stderr = &stderr
	c.WaitDelay = 500 * time.Millisecond // a child left holding the pipes does not keep us waiting
	err := c.Run()
	if err == nil {
		return nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s: %v で打ち切りました", name, SendTimeout)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
	if first = clean(strings.TrimSpace(first)); first != "" {
		return fmt.Errorf("%s: %v: %s", name, err, first)
	}
	return fmt.Errorf("%s: %v", name, err)
}

// clean drops control characters but newlines (a command's output may
// hold escapes), and widens tabs.
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\n' || (r >= 0x20 && r != 0x7f && !(r >= 0x80 && r < 0xa0)):
			b.WriteRune(r)
		}
	}
	return b.String()
}
