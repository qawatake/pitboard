package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// bg is a command running in the background.
type bg struct {
	cmd      *exec.Cmd
	mu       sync.Mutex
	out, err bytes.Buffer
	done     chan struct{}
	code     int
}

type lockedWriter struct {
	mu *sync.Mutex
	b  *bytes.Buffer
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (ss *session) start(args ...string) *bg {
	t := ss.s.t
	t.Helper()
	b := &bg{done: make(chan struct{})}
	c := exec.Command(bin, append([]string{"-s", ss.id}, args...)...)
	c.Dir = ss.s.dir
	c.Env = ss.s.environ()
	c.Stdout = lockedWriter{&b.mu, &b.out}
	c.Stderr = lockedWriter{&b.mu, &b.err}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	b.cmd = c
	go func() {
		err := c.Wait()
		if ee, ok := err.(*exec.ExitError); ok {
			b.code = ee.ExitCode()
		}
		close(b.done)
	}()
	t.Cleanup(func() {
		select {
		case <-b.done:
		default:
			_ = c.Process.Kill()
			<-b.done
		}
	})
	return b
}

func (b *bg) stdout() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.out.String()
}

func (b *bg) stderr() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err.String()
}

func (b *bg) exited() bool {
	select {
	case <-b.done:
		return true
	default:
		return false
	}
}

func (b *bg) waitExit(t *testing.T) int {
	t.Helper()
	select {
	case <-b.done:
		return b.code
	case <-time.After(waitTimeout):
		t.Fatalf("command did not end; stdout %q stderr %q", b.stdout(), b.stderr())
		return -1
	}
}

// waitingFile waits until the waiting file holds pid and returns its text.
func (ss *session) waitWaiting(t *testing.T, pid int) string {
	t.Helper()
	return pollFile(t, filepath.Join(ss.dir, "waiting"), "the waiting pid", func(s string) bool {
		return strings.HasPrefix(s, strconv.Itoa(pid))
	})
}

// SPEC.md "wait".
func TestWait(t *testing.T) {
	setup := func(t *testing.T) *session {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write("<Title>t</Title>\n")
		return ss
	}

	t.Run("delivers when the person sends", func(t *testing.T) {
		ss := setup(t)
		w := ss.start("wait")
		ss.waitWaiting(t, w.cmd.Process.Pid)
		// An event without a send does not end it.
		ss.appendEvents(ev(1, "thought", `"text":"hi"`))
		// The send ends it with what changes prints.
		ss.appendEvents(ev(2, "send", ""))
		if code := w.waitExit(t); code != 0 {
			t.Errorf("exit %d", code)
		}
		wantLines(t, w.stdout(), "thought 1: hi")
		if _, err := os.Stat(filepath.Join(ss.dir, "waiting")); !os.IsNotExist(err) {
			t.Errorf("the waiting file is left behind: %v", err)
		}
		if out := ss.mustRun("changes"); out != "" {
			t.Errorf("wait did not move seen: changes printed %q", out)
		}
	})

	t.Run("an undelivered send ends it at once", func(t *testing.T) {
		ss := setup(t)
		ss.appendEvents(ev(1, "thought", `"text":"hi"`), ev(2, "send", ""))
		r := ss.run("wait")
		if r.code != 0 {
			t.Fatalf("exit %d", r.code)
		}
		wantLines(t, r.stdout, "thought 1: hi")
	})

	t.Run("a send delivered by someone else: waits again", func(t *testing.T) {
		ss := setup(t)
		w := ss.start("wait", "--timeout", "1500ms")
		ss.waitWaiting(t, w.cmd.Process.Pid)
		ss.appendEvents(ev(1, "thought", `"text":"hi"`), ev(2, "send", ""))
		// Delivered by changes (as a hook would) before wait reads it, or
		// by wait itself: either way exactly one of them prints it.
		c := ss.mustRun("changes")
		w.waitExit(t)
		if got := c + w.stdout(); strings.TrimSpace(got) != "thought 1: hi" {
			t.Errorf("delivered %q (changes) + %q (wait)", c, w.stdout())
		}
	})

	t.Run("timeout ends quietly", func(t *testing.T) {
		ss := setup(t)
		for _, args := range [][]string{{"wait", "--timeout", "200ms"}, {"wait", "--timeout=200ms"}} {
			r := ss.run(args...)
			if r.code != 0 || r.stdout != "" {
				t.Errorf("%v: exit %d, out %q", args, r.code, r.stdout)
			}
		}
		if r := ss.run("wait", "--timeout", "bogus"); r.code == 0 {
			t.Errorf("a bad duration is an error")
		}
	})

	t.Run("SIGTERM and SIGINT end quietly", func(t *testing.T) {
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
			ss := setup(t)
			w := ss.start("wait")
			ss.waitWaiting(t, w.cmd.Process.Pid)
			if err := w.cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if code := w.waitExit(t); code != 0 || w.stdout() != "" {
				t.Errorf("%v: exit %d, out %q", sig, code, w.stdout())
			}
			if _, err := os.Stat(filepath.Join(ss.dir, "waiting")); !os.IsNotExist(err) {
				t.Errorf("%v: the waiting file is left behind", sig)
			}
		}
	})

	t.Run("a second waiter leaves it to the first", func(t *testing.T) {
		ss := setup(t)
		w := ss.start("wait")
		pid := w.cmd.Process.Pid
		ss.waitWaiting(t, pid)
		r := ss.run("wait")
		if r.code != 0 || !strings.Contains(r.stdout+r.stderr, fmt.Sprintf("already waiting (pid %d)", pid)) {
			t.Errorf("second wait: exit %d, out %q %q", r.code, r.stdout, r.stderr)
		}
		if w.exited() {
			t.Errorf("the first waiter ended")
		}
	})

	t.Run("a stale waiting file is taken over", func(t *testing.T) {
		ss := setup(t)
		// A pid that is not running (beyond pid_max on both systems is not
		// guaranteed, so take a finished process's pid).
		dead := exec.Command("sh", "-c", "exit 0")
		dead.Env = ss.s.environ()
		dead.Path = filepath.Join(toolDir, "sh")
		if err := dead.Run(); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(ss.dir, "waiting"), fmt.Sprintf("%d Mon Jan  1 00:00:00 2001\n", dead.Process.Pid))
		w := ss.start("wait")
		ss.waitWaiting(t, w.cmd.Process.Pid)
		// A live pid with another start time is stale too.
		_ = w.cmd.Process.Kill()
		w.waitExit(t)
		writeFile(t, filepath.Join(ss.dir, "waiting"), fmt.Sprintf("%d Mon Jan  1 00:00:00 2001\n", os.Getpid()))
		w2 := ss.start("wait")
		ss.waitWaiting(t, w2.cmd.Process.Pid)
	})

	t.Run("a report problem goes to stderr at the start", func(t *testing.T) {
		ss := setup(t)
		ss.write("<Item id=\"z\">\n")
		r := ss.run("wait", "--timeout", "100ms")
		if !strings.Contains(r.stderr, "problem(s). Run: pitboard -s abcdef12 check") {
			t.Errorf("stderr %q", r.stderr)
		}
	})
}

// SPEC.md "hook".
func TestHook(t *testing.T) {
	const settleLine = `Settle each thought with "pitboard -s abcdef12345 settle N [--doing|--hold|--done] TEXT" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.`
	body := "pitboard (the person acted on the board):\nthought 1: hi\n" + settleLine
	setup := func(t *testing.T, events ...string) *session {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write("<Title>t</Title>\n")
		if len(events) == 0 {
			events = []string{ev(1, "thought", `"text":"hi"`), ev(2, "send", "")}
		}
		ss.appendEvents(events...)
		return ss
	}
	hookIn := func(ss *session, dir string) string {
		return fmt.Sprintf(`{"session_id":%q,"cwd":%q}`, ss.id, dir)
	}

	tests := []struct {
		event string
		want  string
	}{
		{"prompt-submit", body + "\n"},
		{"post-tool", `{"hookSpecificOutput":{"additionalContext":` + strconv.Quote(body) + `,"hookEventName":"PostToolUse"}}` + "\n"},
		{"stop", `{"decision":"block","reason":` + strconv.Quote(body) + `}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			ss := setup(t)
			sub := filepath.Join(ss.s.dir, "sub")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			// Run from elsewhere: only cwd in the JSON counts.
			r := ss.s.runIn(ss.s.home, hookIn(ss, sub), "hook", tt.event)
			if r.code != 0 {
				t.Fatalf("exit %d", r.code)
			}
			// post-tool is JSON; compare decoded keys loosely by content.
			if tt.event == "post-tool" {
				if !strings.Contains(r.stdout, `"hookEventName":"PostToolUse"`) || !strings.Contains(r.stdout, strconv.Quote(body)) {
					t.Errorf("got %s", r.stdout)
				}
			} else if r.stdout != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", r.stdout, tt.want)
			}
			// Delivered once.
			if r := ss.s.runIn(ss.s.home, hookIn(ss, sub), "hook", tt.event); r.stdout != "" {
				t.Errorf("second run printed %q", r.stdout)
			}
		})
	}

	t.Run("stop_hook_active: no block, kept for later", func(t *testing.T) {
		ss := setup(t)
		in := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"stop_hook_active":true}`, ss.id, ss.s.dir)
		r := ss.s.runIn(ss.s.dir, in, "hook", "stop")
		if r.code != 0 || r.stdout != "" {
			t.Errorf("exit %d, out %q", r.code, r.stdout)
		}
		wantLines(t, ss.mustRun("changes"), "thought 1: hi")
	})

	t.Run("stop with only hide: no block, kept for later", func(t *testing.T) {
		ss := setup(t, ev(1, "hide", `"key":"k","title":"T"`), ev(2, "send", ""))
		r := ss.s.runIn(ss.s.dir, hookIn(ss, ss.s.dir), "hook", "stop")
		if r.code != 0 || r.stdout != "" {
			t.Errorf("exit %d, out %q", r.code, r.stdout)
		}
		wantLines(t, ss.mustRun("changes"), "hide (1): T")
	})

	t.Run("sessions without a board are untouched", func(t *testing.T) {
		ss := setup(t)
		ins := []string{
			`not json`,
			`{"cwd":"` + ss.s.dir + `"}`,
			`{"session_id":"abcdef","cwd":"` + ss.s.dir + `"}`, // a prefix is not enough
			`{"session_id":"../x","cwd":"` + ss.s.dir + `"}`,
			`{"session_id":"other","cwd":"` + ss.s.dir + `"}`,
			`{"session_id":"abcdef12345","cwd":"` + ss.s.home + `"}`, // no board above home/
		}
		for _, in := range ins {
			for _, e := range []string{"prompt-submit", "post-tool", "stop"} {
				r := ss.s.runIn(ss.s.dir, in, "hook", e)
				if r.code != 0 || r.stdout != "" || r.stderr != "" {
					t.Errorf("%s %s: exit %d, out %q %q", in, e, r.code, r.stdout, r.stderr)
				}
			}
		}
		entries, _ := os.ReadDir(filepath.Join(ss.s.dir, ".pitboard"))
		if len(entries) != 1 {
			t.Errorf("hooks made something: %v", entries)
		}
		if _, err := os.Stat(filepath.Join(ss.s.home, ".pitboard")); !os.IsNotExist(err) {
			t.Errorf("hooks made a board in home")
		}
		wantLines(t, ss.mustRun("changes"), "thought 1: hi")
	})

	t.Run("no events.jsonl: nothing, and it is not made", func(t *testing.T) {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		r := s.runIn(s.dir, hookIn(ss, s.dir), "hook", "stop")
		if r.code != 0 || r.stdout != "" {
			t.Errorf("exit %d, out %q", r.code, r.stdout)
		}
		if _, err := os.Stat(filepath.Join(ss.dir, "events.jsonl")); !os.IsNotExist(err) {
			t.Errorf("events.jsonl was made")
		}
	})

	t.Run("stdin over 1 MiB: nothing", func(t *testing.T) {
		ss := setup(t)
		in := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"pad":%q}`, ss.id, ss.s.dir, strings.Repeat("x", 1<<20))
		r := ss.s.runIn(ss.s.dir, in, "hook", "prompt-submit")
		if r.code != 0 || r.stdout != "" {
			t.Errorf("exit %d, out %q", r.code, r.stdout[:min(len(r.stdout), 100)])
		}
	})
}
