package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Waiting is sending that does not depend on the terminal: the AI runs
// 'pitboard wait' in the background before it stops, and its agent wakes it
// when the command ends. The command ends when the person sends from the
// board, printing what 'changes' prints.
//
// While it waits, .pitboard/<session>/waiting holds its pid: the viewer
// sees from it that sending will reach the AI (and wakes no terminal), and a
// second 'wait' leaves the waiting to the first.

func (s *Store) waitingPath() string { return filepath.Join(s.dir, "waiting") }

// waitPoll is how often 'wait' looks at events.jsonl (tests shorten it).
var waitPoll = 300 * time.Millisecond

// Waiter is the pid of the 'wait' waiting on this board, or 0 if none (a
// pid whose process is gone counts as none).
func (s *Store) Waiter() int {
	b, err := os.ReadFile(s.waitingPath())
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 || !alive(pid) {
		return 0
	}
	return pid
}

// alive says whether a process with pid exists.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// claimWaiting writes our pid to the waiting file, unless a live process
// holds it already (its pid is returned).
func (s *Store) claimWaiting() (other int, err error) {
	me := os.Getpid()
	for range 3 {
		f, err := os.OpenFile(s.waitingPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, err = fmt.Fprintf(f, "%d\n", me)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			return 0, err
		}
		if !os.IsExist(err) {
			return 0, err
		}
		if pid := s.Waiter(); pid != 0 && pid != me {
			return pid, nil
		}
		// A stale pid (or an empty file being written right now): take it
		// over, unless it is filled in meanwhile.
		if b, _ := os.ReadFile(s.waitingPath()); strings.TrimSpace(string(b)) == "" {
			time.Sleep(50 * time.Millisecond)
			if pid := s.Waiter(); pid != 0 && pid != me {
				return pid, nil
			}
		}
		os.Remove(s.waitingPath())
	}
	return 0, errors.New("could not write " + s.waitingPath())
}

// releaseWaiting removes the waiting file if it is still ours.
func (s *Store) releaseWaiting() {
	b, err := os.ReadFile(s.waitingPath())
	if err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(os.Getpid()) {
		os.Remove(s.waitingPath())
	}
}

// sentPending says whether a send has not been delivered yet.
func (s *Store) sentPending() bool {
	return lastSend(s.Events()) > s.Seen()
}

// Wait waits until the person sends, then prints what 'changes' prints. It
// ends quietly at timeout (0: never) or when ctx is done (a signal).
func (s *Store) Wait(ctx context.Context, timeout time.Duration, out io.Writer) error {
	other, err := s.claimWaiting()
	if err != nil {
		return err
	}
	if other != 0 {
		fmt.Fprintf(out, "already waiting (pid %d)\n", other)
		return nil
	}
	defer s.releaseWaiting()
	var deadline <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		deadline = t.C
	}
	tick := time.NewTicker(waitPoll)
	defer tick.Stop()
	var size int64 = -1
	var mod time.Time
	for {
		// Read only when events.jsonl has changed (or the first time).
		if fi, err := os.Stat(s.eventsPath()); err == nil && (fi.Size() != size || !fi.ModTime().Equal(mod)) {
			size, mod = fi.Size(), fi.ModTime()
			if s.sentPending() {
				lines, err := s.Changes()
				if err != nil {
					return err
				}
				for _, l := range lines {
					fmt.Fprintln(out, l)
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-deadline:
			return nil
		case <-tick.C:
		}
	}
}

// runWait is 'pitboard wait [--timeout DURATION]'.
func runWait(st *Store, args []string, out io.Writer) error {
	var timeout time.Duration
	for len(args) > 0 {
		v, ok := "", false
		switch a := args[0]; {
		case a == "--timeout" || a == "-timeout":
			if len(args) < 2 {
				return errors.New("--timeout needs a duration (e.g. 30m)")
			}
			v, ok, args = args[1], true, args[2:]
		case strings.HasPrefix(a, "--timeout="), strings.HasPrefix(a, "-timeout="):
			_, v, _ = strings.Cut(a, "=")
			ok, args = true, args[1:]
		default:
			return fmt.Errorf("wait: unknown argument %q (usage: pitboard wait [--timeout DURATION])", a)
		}
		if ok {
			d, err := time.ParseDuration(v)
			if err != nil || d < 0 {
				return fmt.Errorf("--timeout %q: use a duration like 30m or 1h", v)
			}
			timeout = d
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return st.Wait(ctx, timeout, out)
}
