package board

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

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

// 16: a waiting file whose pid now belongs to another process.
func TestWaiterStartTime(t *testing.T) {
	st := testStore(t)
	me := os.Getpid()
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d %s\n", me, startOf(me))), 0o644)
	if st.Waiter() != me {
		t.Error("own pid with its start not taken")
	}
	if startOf(me) == "" {
		t.Skip("ps gives no start time here")
	}
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d Mon Jan  1 00:00:00 2001\n", me)), 0o644)
	if st.Waiter() != 0 {
		t.Error("a reused pid counts as waiting")
	}
	os.WriteFile(st.waitingPath(), []byte(fmt.Sprintf("%d\n", me)), 0o644)
	if st.Waiter() != me {
		t.Error("an old file (pid only) is not read")
	}
}
