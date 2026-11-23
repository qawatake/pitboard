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

// Waiting is sending that does not depend on the terminal: the AI runs
// 'pitboard wait' in the background before it stops, and its agent wakes it
// when the command ends. The command ends when the person sends from the
// board, printing what 'changes' prints.
//
// While it waits, .pitboard/<session>/waiting holds its pid: the viewer
// sees from it that sending will reach the AI (and wakes no terminal), and a
// second 'wait' leaves the waiting to the first.

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
	// Waiting goes on, but the AI hears now that the report needs fixing.
	s.NoteProblems(errOut)
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
			if s.sentPending() {
				lines, err := s.Changes()
				if err != nil {
					return err
				}
				for _, l := range lines {
					fmt.Fprintln(out, l)
				}
				s.NoteOpen(out)
				s.NoteProblems(out)
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-deadline:
			s.NoteProblems(out)
			return nil
		case <-tick.C:
		}
	}
}
