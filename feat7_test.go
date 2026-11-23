package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func shortPoll(t *testing.T) {
	prev := waitPoll
	waitPoll = 10 * time.Millisecond
	t.Cleanup(func() { waitPoll = prev })
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

func TestWaitReturnsAtOnceWhenSent(t *testing.T) {
	shortPoll(t)
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "one"})
	sendAll(st)
	var out bytes.Buffer
	if err := st.Wait(context.Background(), time.Second, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "thought 1: one\n" || st.Seen() != 2 {
		t.Errorf("out %q seen %d", out.String(), st.Seen())
	}
	if _, err := os.Stat(st.waitingPath()); !os.IsNotExist(err) {
		t.Error("waiting file left behind")
	}
}

func TestWaitWaitsForASend(t *testing.T) {
	shortPoll(t)
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "before"})
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
	st.Append(Event{Op: "thought", Text: "two"})
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
	if a.waiter || st.Waiter() != 0 {
		t.Error("still shown as waiting")
	}
}

func TestWaitTimeoutAndSignal(t *testing.T) {
	shortPoll(t)
	st := testStore(t)
	var out bytes.Buffer
	start := time.Now()
	if err := st.Wait(context.Background(), 50*time.Millisecond, &out); err != nil || out.Len() != 0 {
		t.Errorf("timeout: %v %q", err, out.String())
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Error("ended before the timeout")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	if err := st.Wait(ctx, 0, &out); err != nil || out.Len() != 0 {
		t.Errorf("signal: %v %q", err, out.String())
	}
	if st.Waiter() != 0 {
		t.Error("waiting file left behind")
	}
}

func TestSecondWaitAndStalePid(t *testing.T) {
	shortPoll(t)
	st := testStore(t)
	// A live process that is not us holds it.
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	os.WriteFile(st.waitingPath(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	var out bytes.Buffer
	if err := st.Wait(context.Background(), time.Second, &out); err != nil {
		t.Fatal(err)
	}
	if want := "already waiting (pid " + strconv.Itoa(cmd.Process.Pid) + ")\n"; out.String() != want {
		t.Errorf("out %q", out.String())
	}
	// Once it is gone, its pid is stale: the next wait takes over.
	cmd.Process.Kill()
	cmd.Wait()
	if st.Waiter() != 0 {
		t.Error("a dead pid counts as waiting")
	}
	out.Reset()
	if err := st.Wait(context.Background(), 30*time.Millisecond, &out); err != nil || out.Len() != 0 {
		t.Errorf("after the stale pid: %v %q", err, out.String())
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
