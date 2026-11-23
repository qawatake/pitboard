package viewer

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

// wake types a line into the AI's terminal telling it that the board has
// something it has not read. What was sent is not marked as delivered: that
// happens when the AI runs 'changes'.
func (a *app) wake() {
	if !a.canWrite() {
		return
	}
	a.loadEvents()
	n := a.unsent
	if n == 0 {
		a.msg = "未送信のものはありません"
		return
	}
	// The mark comes first: what is above it is what the AI gets from
	// 'changes', whether or not it can be woken now.
	if _, err := a.st.Append(board.Event{Op: "send"}); err != nil {
		a.msg = "送れませんでした: " + err.Error()
		return
	}
	a.loadEvents()
	// A 'pitboard wait' waiting for this send wakes the AI by ending: no
	// terminal is typed into.
	if a.st.Waiter() != 0 {
		a.msg = "送りました"
		return
	}
	// Waking may take seconds: say so, and drop the keys pressed meanwhile
	// (they were pressed at a screen that said nothing yet, and would wipe
	// out the result).
	a.msg = "送っています…"
	a.render()
	err := deliver(a.st.Session, wakeMessage(a.st, n))
	a.dropBefore = time.Now()
	if errors.Is(err, errNoTarget) {
		a.msg, a.msgLines = "送りました。ただ AI が待っていないので、チャットで一言伝えてください。\n"+err.Error(), 2
		return
	}
	if err != nil {
		a.msg, a.msgLines = "送りました。ただ AI を起こせなかったので、チャットで一言伝えてください。\n"+err.Error(), 2
		return
	}
	a.msg = "送りました"
}

// Sending wakes the AI from the board. A hook can only deliver while the AI
// is working; when it has stopped, what the person wrote waits until they
// type something in the chat. Sending types one line into the AI's terminal
// saying there is something to read; the AI reads it with 'changes', so what
// has been delivered is still kept in one place (seen).

// sendTimeout is how long sending may take before it is cut off (tests
// shorten it).
var sendTimeout = 5 * time.Second

// errNoTarget means there is no way to wake the AI: no $PITBOARD_SEND and
// no herdr. Its text says how to fix it.
var errNoTarget = errors.New(`AI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")`)

// wakeMessage is the line typed into the AI's terminal. It stays in the
// AI's conversation, so it is short; -h says what it means.
func wakeMessage(st *board.Store, n int) string {
	return fmt.Sprintf("pitboard: %d sent. Run: pitboard -s %s changes", n, board.ShortSession(st))
}

// deliver types msg into the terminal of the AI of session: through
// $PITBOARD_SEND if set, or else through herdr when it is on PATH (the
// viewer itself need not run inside herdr; the AI's agent is looked up by
// its session). The error's text says why it failed and how to fix it.
func deliver(session, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
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
		return fmt.Errorf("%s: %v で打ち切りました", name, sendTimeout)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
	if first = clean(strings.TrimSpace(first)); first != "" {
		return fmt.Errorf("%s: %v: %s", name, err, first)
	}
	return fmt.Errorf("%s: %v", name, err)
}
