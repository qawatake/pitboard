package board

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Waiting is sending that does not depend on the terminal: a process waits
// for the person's send and its ending wakes the AI. It is the Stop hook
// ('pitboard hook stop', started by Claude Code each time the AI stops), or
// 'pitboard wait', which the AI runs in the background before it stops when
// the hook is not installed. Either ends when the person sends from the
// board, with what 'changes' prints.
//
// While it waits, .pitboard/<session>/waiting holds its pid: the viewer
// sees from it that sending will reach the AI (and wakes no terminal), and a
// second waiter leaves the waiting to the first.

func (s *Store) waitingPath() string { return filepath.Join(s.Dir, "waiting") }

// waitPoll is how often 'wait' looks at events.jsonl (tests shorten it).
var waitPoll = 300 * time.Millisecond

// Waiter is the pid of the 'wait' waiting on this board, or 0 if none (a
// pid whose process is gone counts as none). The file holds the pid and,
// when it could be found, when that process started: a pid reused by
// another process (the 'wait' was killed with -9) does not count.
func (s *Store) Waiter() int {
	b, err := os.ReadFile(s.waitingPath())
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	pid, err := strconv.Atoi(f[0])
	if err != nil || pid <= 0 || !alive(pid) {
		return 0
	}
	if len(f) > 1 {
		if now := startOf(pid); now != "" && now != strings.Join(f[1:], " ") {
			return 0
		}
	}
	return pid
}

// startOf is when process pid started, as ps(1) says it, or "" when that
// cannot be found. Answers are kept for a few seconds (the viewer asks on
// every tick).
func startOf(pid int) string {
	startMu.Lock()
	defer startMu.Unlock()
	if c, ok := startCache[pid]; ok && time.Since(c.at) < 5*time.Second {
		return c.start
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	start := ""
	if err == nil {
		start = strings.Join(strings.Fields(string(out)), " ")
	}
	startCache[pid] = startEntry{start, time.Now()}
	return start
}

type startEntry struct {
	start string
	at    time.Time
}

var (
	startMu    sync.Mutex
	startCache = map[int]startEntry{}
)

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
			line := strconv.Itoa(me)
			if st := startOf(me); st != "" {
				line += " " + st
			}
			_, err = fmt.Fprintln(f, line)
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
	if f := strings.Fields(string(b)); err == nil && len(f) > 0 && f[0] == strconv.Itoa(os.Getpid()) {
		os.Remove(s.waitingPath())
	}
}

// sentPending says whether a send has not been delivered yet.
func (s *Store) sentPending() bool {
	return lastSend(s.Events()) > s.Seen()
}

// How an Await ended.
type WaitEnd int

const (
	WaitDone      WaitEnd = iota // try said it was done
	WaitOther                    // another process is waiting already
	WaitTimeout                  // the timeout passed
	WaitCancelled                // ctx was done (a signal)
)

// Await holds the waiting file and calls try at once and then every time
// events.jsonl has changed, until try says it is done. It ends without it
// when another process waits on this board already (other is its pid), at
// timeout (0: never) or when ctx is done. started, if given, is called once
// the waiting file is ours.
func (s *Store) Await(ctx context.Context, timeout time.Duration, started func(), try func() (bool, error)) (end WaitEnd, other int, err error) {
	other, err = s.claimWaiting()
	if err != nil {
		return 0, 0, err
	}
	if other != 0 {
		return WaitOther, other, nil
	}
	defer s.releaseWaiting()
	if started != nil {
		started()
	}
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
		if fi, err := os.Stat(s.EventsPath()); err == nil && (fi.Size() != size || !fi.ModTime().Equal(mod)) {
			size, mod = fi.Size(), fi.ModTime()
			if done, err := try(); err != nil || done {
				return WaitDone, 0, err
			}
		}
		select {
		case <-ctx.Done():
			return WaitCancelled, 0, nil
		case <-deadline:
			return WaitTimeout, 0, nil
		case <-tick.C:
		}
	}
}

// Wait waits until the person sends, then prints what 'changes' prints. It
// ends quietly at timeout (0: never) or when ctx is done (a signal).
//
// Once the Stop hook has run for this session (see MarkStopHook) the hook
// does the waiting: Wait then never waits, and whatever it prints ends with
// StopHookNote.
func (s *Store) Wait(ctx context.Context, timeout time.Duration, out io.Writer) error {
	deliver := func() error {
		lines, err := s.Changes()
		if err != nil {
			return err
		}
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		s.NoteOpen(out)
		return nil
	}
	if s.StopHook() {
		if s.sentPending() {
			if err := deliver(); err != nil {
				return err
			}
		}
		s.NoteProblems(out)
		fmt.Fprintln(out, StopHookNote)
		return nil
	}
	end, other, err := s.Await(ctx, timeout, func() {
		// Waiting goes on, but the AI hears now that the report needs fixing.
		s.NoteReportProblems(errOut)
	}, func() (bool, error) {
		if !s.sentPending() {
			return false, nil
		}
		return true, deliver()
	})
	if err != nil {
		return err
	}
	switch end {
	case WaitOther:
		fmt.Fprintf(out, "already waiting (pid %d)\n", other)
	case WaitDone, WaitTimeout:
		s.NoteProblems(out)
	case WaitCancelled:
		return nil
	}
	// The hook started while this was waiting (it left the waiting to us).
	if s.StopHook() {
		fmt.Fprintln(out, StopHookNote)
	}
	return nil
}

// StopHookNote is the line 'wait' ends its output with once the Stop hook
// has run for the session.
const StopHookNote = "The Stop hook now waits for the person's sends: they arrive by themselves. Do not run 'wait' again in this session."

// StopHookPath is the marker 'pitboard hook stop' leaves: the Stop hook is
// installed for this session, so 'wait' is not needed.
func (s *Store) StopHookPath() string { return filepath.Join(s.Dir, "stophook") }

// MarkStopHook records that the Stop hook ran for this session.
func (s *Store) MarkStopHook() error { return os.WriteFile(s.StopHookPath(), nil, 0o644) }

// StopHook says whether the Stop hook has run for this session.
func (s *Store) StopHook() bool {
	_, err := os.Stat(s.StopHookPath())
	return err == nil
}

// ProcessStart is when process pid started, as ps(1) says it ("" when it
// cannot be found): kept with a pid, it tells a reused pid apart.
func ProcessStart(pid int) string { return startOf(pid) }

// ProcessLive says whether process pid runs and, when start is known, is
// the process that started then (the same check as the waiting file).
func ProcessLive(pid int, start string) bool {
	if pid <= 0 || !alive(pid) {
		return false
	}
	if start != "" {
		if now := startOf(pid); now != "" && now != start {
			return false
		}
	}
	return true
}
