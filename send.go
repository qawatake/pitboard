package main

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
)

// Sending wakes the AI from the board. A hook can only deliver while the AI
// is working; when it has stopped, what the person wrote waits until they
// type something in the chat. Sending types one line into the AI's terminal
// saying there is something to read; the AI reads it with 'changes', so what
// has been delivered is still kept in one place (seen).

// sendTimeout is how long sending may take before it is cut off (tests
// shorten it).
var sendTimeout = 5 * time.Second

// errNoTarget means there is nowhere to send to.
var errNoTarget = errors.New("送り先がありません (herdr の中で開くか、PITBOARD_SEND を設定)")

// wakeMessage is the line typed into the AI's terminal.
func wakeMessage(session string, n int) string {
	return fmt.Sprintf("pitboard: the person sent %d item(s) from the board. Run \"pitboard -s %s changes\" and act on them.", n, session)
}

// unsent counts what the AI has not been given yet: reactions, thoughts and
// hides (settle is the AI's own).
func unsent(evs []Event, seen int) int {
	c := 0
	for _, e := range evs {
		if e.N > seen && e.Op != "settle" {
			c++
		}
	}
	return c
}

// deliver types msg into the terminal of the AI of session: through
// $PITBOARD_SEND if set, or through herdr when running inside it.
func deliver(session, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	if sh := os.Getenv("PITBOARD_SEND"); sh != "" {
		c := exec.CommandContext(ctx, "sh", "-c", sh)
		c.Env = append(os.Environ(), "PITBOARD_SESSION="+session, "PITBOARD_MESSAGE="+msg)
		return runQuiet(ctx, c, "PITBOARD_SEND")
	}
	if os.Getenv("HERDR_ENV") != "1" {
		return errNoTarget
	}
	pane, err := herdrPane(ctx, session)
	if err != nil {
		return err
	}
	return runQuiet(ctx, exec.CommandContext(ctx, "herdr", "agent", "prompt", pane, msg), "herdr agent prompt")
}

// herdrPane finds the pane of the agent whose session is session.
func herdrPane(ctx context.Context, session string) (string, error) {
	c := exec.CommandContext(ctx, "herdr", "agent", "list")
	var out bytes.Buffer
	c.Stdout = &out
	if err := runQuiet(ctx, c, "herdr agent list"); err != nil {
		return "", err
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
		return "", fmt.Errorf("herdr agent list: 読めませんでした: %v", err)
	}
	for _, a := range list.Result.Agents {
		if a.AgentSession.Value == session && a.PaneID != "" {
			return a.PaneID, nil
		}
	}
	return "", fmt.Errorf("送り先がありません (herdr にセッション %s の AI が見つかりません)", session)
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
