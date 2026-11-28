package viewer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

func TestWakeMessageIsShort(t *testing.T) {
	base := t.TempDir()
	for _, s := range []string{"38d12f95-0b22-4d64-88fb-17ed69d428cb", "38d12f95-9999", "abc"} {
		os.MkdirAll(board.Open(base, s).Dir, 0o755)
	}
	st := board.Open(base, "38d12f95-0b22-4d64-88fb-17ed69d428cb")
	if got := page.WakeMessage(st, 4); got != "pitboard: 4 sent. Run: pitboard -s 38d12f95-0 changes" {
		t.Errorf("%q", got)
	}
}

// onHook puts the cursor on the Item "hook", pointed at by the person.
func onHook(t *testing.T, a *app) {
	t.Helper()
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	wait(t, settleDelay)
	a.render()
	if a.top().cur != "hook" {
		t.Fatalf("cursor on %q", a.top().cur)
	}
}

// ---- sending ----

// noSendTarget makes sure no test sends anywhere real.
func noSendTarget(t *testing.T) {
	t.Helper()
	t.Setenv("PITBOARD_SEND", "")
	t.Setenv("HERDR_ENV", "")
	t.Setenv("PATH", "/usr/bin:/bin") // no herdr (sh is still there)
}

// fakeHerdr puts a stand-in for herdr first on PATH: 'agent list' prints
// list, 'agent prompt' writes its arguments to the returned file.
func fakeHerdr(t *testing.T, list string) string {
	t.Helper()
	noSendTarget(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "list.json"), []byte(list), 0o644)
	prompted := filepath.Join(dir, "prompted")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1 $2" = "agent list" ]; then cat %q; exit 0; fi
if [ "$1 $2" = "agent prompt" ]; then printf '%%s\n%%s\n' "$3" "$4" > %q; exit 0; fi
echo "unexpected: $*" >&2; exit 2
`, filepath.Join(dir, "list.json"), prompted)
	os.WriteFile(filepath.Join(dir, "herdr"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// HERDR_ENV is left unset: herdr on PATH is enough (the viewer may run
	// outside herdr).
	return prompted
}

const herdrList = `{"id":"cli:agent:list","result":{"agents":[
 {"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"other"},"pane_id":"w1:p1"},
 {"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"s1"},"pane_id":"w1:p2"}
],"type":"agent_list"}}`

func TestSendThroughHerdr(t *testing.T) {
	prompted := fakeHerdr(t, herdrList)
	st := testStore(t)
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if a.msg != "未送信のものはありません" {
		t.Errorf("nothing to send: %q", a.msg)
	}
	if _, err := os.Stat(prompted); err == nil {
		t.Fatal("sent with nothing to send")
	}
	st.Append(board.Event{Op: "thought", Text: "a"})
	st.Append(board.Event{Op: "hide", Key: "-|x", Title: "x"})
	a.handle(key{r: 's'})
	if a.msg != "送りました" {
		t.Fatalf("msg %q", a.msg)
	}
	b, _ := os.ReadFile(prompted)
	want := "w1:p2\n" + page.WakeMessage(st, 2) + "\n"
	if string(b) != want {
		t.Errorf("prompted %q, want %q", b, want)
	}
	if !strings.Contains(want, `pitboard: 2 sent. Run: pitboard -s s1 changes`) {
		t.Errorf("message: %q", want)
	}
	if st.Seen() != 0 {
		t.Error("sending must not mark anything as delivered")
	}
	if evs := st.Events(); evs[len(evs)-1].Op != "send" {
		t.Errorf("s must record a send: %+v", evs[len(evs)-1])
	}
}

func TestSendHerdrWithoutTheSession(t *testing.T) {
	prompted := fakeHerdr(t, `{"result":{"agents":[{"agent_session":{"value":"other"},"pane_id":"w1:p1"}]}}`)
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if !strings.Contains(a.msg, "送りました。ただ") || !strings.Contains(a.msg, "herdr にこのセッション (s1) の AI がいません。AI を herdr の中で動かすか、PITBOARD_SEND を設定してください") {
		t.Errorf("msg %q", a.msg)
	}
	if _, err := os.Stat(prompted); err == nil {
		t.Error("prompted another agent")
	}
}

func TestSendNowhere(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if a.msg != "送りました。ただ AI が待っていないので、チャットで一言伝えてください。\n"+`AI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard help send)` {
		t.Errorf("msg %q", a.msg)
	}
	// The mark stays: the AI gets the thought from changes.
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "thought 1: a" {
		t.Errorf("changes after a failed wake: %q", lines)
	}
	// Wrapped, never cut: how to fix it can be read at any width.
	a.t.w = 50
	if b := plainLines(a.bottom()); strings.Contains(b, "…") || !strings.HasSuffix(b, "help send)") {
		t.Errorf("failure message lines: %q", b)
	}
}

func TestSendCommand(t *testing.T) {
	noSendTarget(t)
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("PITBOARD_SEND", `printf '%s|%s' "$PITBOARD_SESSION" "$PITBOARD_MESSAGE" > `+out)
	t.Setenv("HERDR_ENV", "1") // PITBOARD_SEND comes first
	st := testStore(t)
	a := testApp(t, st)
	// ctrl+s in the editor does nothing; s sends once written.
	a.handle(key{r: 'm'})
	for _, r := range "思いつき" {
		a.handle(key{r: r})
	}
	a.handle(key{name: "ctrl+s"})
	if a.in == nil || len(st.Events()) != 0 {
		t.Fatalf("ctrl+s in the editor must do nothing: %+v", st.Events())
	}
	a.handle(key{name: "enter"})
	a.handle(key{r: 's'})
	if a.in != nil || len(st.Events()) != 2 || st.Events()[0].Text != "思いつき" || st.Events()[1].Op != "send" {
		t.Fatalf("enter then s must write and send: %+v", st.Events())
	}
	b, _ := os.ReadFile(out)
	if string(b) != "s1|"+page.WakeMessage(st, 1) {
		t.Errorf("PITBOARD_SEND got %q", b)
	}
	if a.msg != "送りました" {
		t.Errorf("msg %q", a.msg)
	}
	// enter only writes.
	os.Remove(out)
	a.handle(key{r: 'm'})
	a.handle(key{r: 'x'})
	a.handle(key{name: "enter"})
	if _, err := os.Stat(out); err == nil || len(st.Events()) != 3 {
		t.Error("enter must write without sending")
	}
}

func TestSendFailsAndTimesOut(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	t.Setenv("PITBOARD_SEND", "echo first line >&2; echo second >&2; exit 3")
	a.handle(key{r: 's'})
	if !strings.Contains(a.msg, "送れませんでした") || !strings.Contains(a.msg, "first line") || strings.Contains(a.msg, "second") {
		t.Errorf("failure: %q", a.msg)
	}
	prev := page.SendTimeout
	page.SendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { page.SendTimeout = prev })
	t.Setenv("PITBOARD_SEND", "sleep 5")
	st.Append(board.Event{Op: "thought", Text: "b"})
	start := time.Now()
	a.handle(key{r: 's'})
	if d := time.Since(start); d > 2*time.Second || !strings.Contains(a.msg, "打ち切りました") {
		t.Errorf("timeout after %v: %q", d, a.msg)
	}
}

func TestSendRecordsOnlyWhenThereIsSomething(t *testing.T) {
	noSendTarget(t)
	t.Setenv("PITBOARD_SEND", "true")
	st := testStore(t)
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if len(st.Events()) != 0 || a.msg != "未送信のものはありません" {
		t.Errorf("s with nothing: %d events, %q", len(st.Events()), a.msg)
	}
	st.Append(board.Event{Op: "thought", Text: "a"})
	a.handle(key{r: 's'})
	a.handle(key{r: 's'})
	n := 0
	for _, e := range st.Events() {
		if e.Op == "send" {
			n++
		}
	}
	if n != 1 || a.msg != "未送信のものはありません" {
		t.Errorf("sends %d, msg %q", n, a.msg)
	}
	// A thought taken back before it was sent: nothing to send.
	th, _ := st.Append(board.Event{Op: "thought", Text: "b"})
	st.Append(board.Event{Op: "retract", Thought: th.N})
	a.handle(key{r: 's'})
	if a.msg != "未送信のものはありません" {
		t.Errorf("only a taken-back thought: %q", a.msg)
	}
}

// safeBuf is a buffer read while another goroutine writes it.
type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// The wait polls at its usual pace here (board's tests shorten it).
func TestWaitWaitsForASend(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", Text: "before"})
	var out safeBuf
	done := make(chan error)
	go func() { done <- st.Wait(context.Background(), 0, &out) }()
	deadline := time.Now().Add(2 * time.Second)
	for st.Waiter() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st.Waiter() != os.Getpid() {
		t.Fatalf("waiter %d", st.Waiter())
	}
	// Not sent yet: it keeps waiting.
	st.Append(board.Event{Op: "thought", Text: "two"})
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("wait ended before a send")
	default:
	}
	// The viewer sees it and sends without typing into a terminal.
	noSendTarget(t)
	a := testApp(t, st)
	a.render()
	if !strings.Contains(reEsc.ReplaceAllString(a.overlayRight("", a.sendMark()), ""), "AI 待機中") {
		t.Errorf("top right: %q", reEsc.ReplaceAllString(a.overlayRight("", a.sendMark()), ""))
	}
	a.handle(key{r: 's'})
	if a.msg != "送りました" {
		t.Errorf("msg %q", a.msg)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not end after the send")
	}
	if out.String() != "thought 1: before\nthought 2: two\n" {
		t.Errorf("out %q", out.String())
	}
	a.loadEvents()
	if a.b.Waiter || st.Waiter() != 0 {
		t.Error("still shown as waiting")
	}
}
