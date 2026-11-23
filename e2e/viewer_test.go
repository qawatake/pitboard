package e2e_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// waitTimeout bounds every poll. The viewer looks at files every 0.5 s, so a
// few seconds is plenty; the suite never sleeps to synchronize.
const waitTimeout = 6 * time.Second

// viewer is a running `pitboard` viewer on a pty, with a screen model fed
// by everything it writes.
type viewer struct {
	t      *testing.T
	cmd    *exec.Cmd
	master *os.File
	mu     sync.Mutex
	scr    *screen
	raw    strings.Builder // everything written, for debugging
	done   chan struct{}
	err    error // the process's exit, after done
}

// view starts the viewer for this session in a w x h terminal.
func (ss *session) view(w, h int, args ...string) *viewer {
	ss.s.t.Helper()
	return ss.s.view(w, h, append([]string{"-s", ss.id}, args...)...)
}

// view starts `pitboard args...` (the viewer) in the project directory.
func (s *sandbox) view(w, h int, args ...string) *viewer {
	return s.viewIn(s.dir, w, h, args...)
}

func (s *sandbox) viewIn(dir string, w, h int, args ...string) *viewer {
	t := s.t
	t.Helper()
	if !ptySupported {
		t.Skip("viewer tests need a pty; supported on darwin and linux only")
	}
	m, sl, err := openPty()
	if err != nil {
		t.Fatalf("opening a pty: %v", err)
	}
	if err := setSize(m.Fd(), w, h); err != nil {
		t.Fatalf("setting the pty size: %v", err)
	}
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Env = s.environ()
	c.Stdin, c.Stdout, c.Stderr = sl, sl, sl
	c.SysProcAttr = sysProcAttr()
	if err := c.Start(); err != nil {
		t.Fatalf("starting the viewer: %v", err)
	}
	sl.Close()
	v := &viewer{t: t, cmd: c, master: m, scr: newScreen(w, h), done: make(chan struct{})}
	go v.read()
	go func() {
		v.err = c.Wait()
		close(v.done)
	}()
	t.Cleanup(func() {
		select {
		case <-v.done:
		default:
			_ = c.Process.Kill()
			<-v.done
		}
		m.Close()
	})
	return v
}

func (v *viewer) read() {
	buf := make([]byte, 4096)
	for {
		n, err := v.master.Read(buf)
		if n > 0 {
			v.mu.Lock()
			v.scr.feed(buf[:n])
			v.raw.Write(buf[:n])
			v.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// keyBytes maps key names to what a terminal sends. Anything not named is
// sent as typed.
var keyBytes = map[string]string{
	"enter": "\r", "esc": "\x1b[27u", "tab": "\t", "space": " ",
	"backspace": "\x7f", "delete": "\x1b[3~",
	"up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D",
	"pgup": "\x1b[5~", "pgdn": "\x1b[6~", "home": "\x1b[H", "end": "\x1b[F",
	"shift+enter": "\x1b[13;2u", "alt+enter": "\x1b\r", "ctrl+j": "\n",
	"alt+b": "\x1bb", "alt+f": "\x1bf", "alt+backspace": "\x1b\x7f",
	"ctrl+c": "\x03",
}

func keyToBytes(k string) string {
	if b, ok := keyBytes[k]; ok {
		return b
	}
	if strings.HasPrefix(k, "ctrl+") && len(k) == 6 {
		return string(rune(k[5] - 'a' + 1))
	}
	return k
}

// Keys presses keys one write at a time.
func (v *viewer) Keys(keys ...string) {
	v.t.Helper()
	for _, k := range keys {
		if _, err := io.WriteString(v.master, keyToBytes(k)); err != nil {
			v.t.Fatalf("pressing %q: %v", k, err)
		}
	}
}

// Type types text (each rune a key).
func (v *viewer) Type(s string) {
	v.t.Helper()
	if _, err := io.WriteString(v.master, s); err != nil {
		v.t.Fatalf("typing %q: %v", s, err)
	}
}

// Paste sends s as a bracketed paste.
func (v *viewer) Paste(s string) {
	v.t.Helper()
	v.Type("\x1b[200~" + s + "\x1b[201~")
}

// Screen is the current screen as text.
func (v *viewer) Screen() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.scr.text()
}

// Line is row n (0-based) as text.
func (v *viewer) Line(n int) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.scr.line(n)
}

// Lines is every row.
func (v *viewer) Lines() []string {
	return strings.Split(v.Screen(), "\n")
}

// Find returns where sub is on screen, or (-1, -1).
func (v *viewer) Find(sub string) (row, col int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.scr.find(sub)
}

// ColorAt names the look of a cell (see screen.color).
func (v *viewer) ColorAt(row, col int) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.scr.color(row, col)
}

func (v *viewer) AttrAt(row, col int) attr {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.scr.attrAt(row, col)
}

// ColorOf names the look of the first cell of sub on screen; it fails when
// sub is not on screen.
func (v *viewer) ColorOf(sub string) string {
	v.t.Helper()
	r, c := v.Find(sub)
	if r < 0 {
		v.t.Fatalf("%q not on screen:\n%s", sub, v.dump())
	}
	return v.ColorAt(r, c)
}

// Raw is everything the viewer has written so far.
func (v *viewer) Raw() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.raw.String()
}

func (v *viewer) dump() string {
	var b strings.Builder
	b.WriteString("+" + strings.Repeat("-", v.scr.w) + "+\n")
	for _, l := range v.Lines() {
		b.WriteString("|" + l + strings.Repeat(" ", max(0, v.scr.w-colOf(l))) + "|\n")
	}
	b.WriteString("+" + strings.Repeat("-", v.scr.w) + "+")
	return b.String()
}

// Until polls cond until it holds; on timeout it fails with what and the
// screen.
func (v *viewer) Until(what string, cond func() bool) {
	v.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			v.t.Fatalf("timed out waiting for %s; screen:\n%s", what, v.dump())
		}
		select {
		case <-v.done:
			if cond() {
				return
			}
			v.t.Fatalf("viewer exited (%v) while waiting for %s; screen:\n%s", v.err, what, v.dump())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// WaitFor waits until every text is on screen.
func (v *viewer) WaitFor(texts ...string) {
	v.t.Helper()
	v.Until(fmt.Sprintf("%q on screen", texts), func() bool {
		s := v.Screen()
		for _, t := range texts {
			if !strings.Contains(s, t) {
				return false
			}
		}
		return true
	})
}

// WaitForJoined waits until text is on screen, allowing it to wrap
// (lines are trimmed and joined).
func (v *viewer) WaitForJoined(text string) {
	v.t.Helper()
	v.Until(fmt.Sprintf("%q on screen (wrapped)", text), func() bool {
		var b strings.Builder
		for _, l := range v.Lines() {
			b.WriteString(strings.TrimSpace(l))
		}
		return strings.Contains(b.String(), text)
	})
}

// WaitGone waits until no text is on screen.
func (v *viewer) WaitGone(texts ...string) {
	v.t.Helper()
	v.Until(fmt.Sprintf("%q gone from screen", texts), func() bool {
		s := v.Screen()
		for _, t := range texts {
			if strings.Contains(s, t) {
				return false
			}
		}
		return true
	})
}

// WaitLine waits until row n contains sub.
func (v *viewer) WaitLine(n int, sub string) {
	v.t.Helper()
	v.Until(fmt.Sprintf("row %d to contain %q", n, sub), func() bool {
		return strings.Contains(v.Line(n), sub)
	})
}

// Bottom is the last row (the guide line).
func (v *viewer) Bottom() string {
	return v.Line(v.scr.h - 1)
}

// WaitBottom waits until the last row contains sub.
func (v *viewer) WaitBottom(sub string) {
	v.t.Helper()
	v.WaitLine(v.scr.h-1, sub)
}

// CursorRow is the row with the cursor mark ">" in column 0, or -1.
func (v *viewer) CursorRow() int {
	for i, l := range v.Lines() {
		if strings.HasPrefix(l, ">") {
			return i
		}
	}
	return -1
}

// WaitCursorOn waits until the row with the cursor contains sub.
func (v *viewer) WaitCursorOn(sub string) {
	v.t.Helper()
	v.Until(fmt.Sprintf("cursor on %q", sub), func() bool {
		r := v.CursorRow()
		return r >= 0 && strings.Contains(v.Line(r), sub)
	})
}

// helpMarker is on screen only while the key list (?) is open (when it is
// wide and tall enough to show the first description).
const helpMarker = "上下に動く"

// helpOpen says whether the key list is open: its title "キー" stands
// alone on a row (the guide's "? キー" never does).
func (v *viewer) helpOpen() bool {
	for _, l := range v.Lines() {
		if strings.TrimSpace(l) == "キー" {
			return true
		}
	}
	return false
}

// Sync makes sure every key sent so far has been handled: it opens the key
// list with ? and closes it again (the only keys that close it are q, esc
// and ?), waiting for each to show. Not for use inside the editor, where ?
// is text.
func (v *viewer) Sync() {
	v.t.Helper()
	v.Keys("?")
	v.Until("the key list to open", v.helpOpen)
	v.Keys("?")
	v.Until("the key list to close", func() bool { return !v.helpOpen() })
}

// In rebinds the viewer's failures to t (a subtest sharing the viewer).
func (v *viewer) In(t *testing.T) *viewer {
	v.t = t
	return v
}

// MoveTo presses j or k until the cursor's row contains sub (searching
// down first when sub is not on screen).
func (v *viewer) MoveTo(sub string) {
	v.t.Helper()
	for i := 0; i < 80; i++ {
		cur := v.CursorRow()
		if cur >= 0 && strings.Contains(v.Line(cur), sub) {
			return
		}
		r, _ := v.Find(sub)
		k := "j"
		if r >= 0 && cur >= 0 && r < cur {
			k = "k"
		}
		before := v.Screen()
		v.Keys(k)
		// Wait for the key to take effect (or a sync when nothing changes).
		deadline := time.Now().Add(time.Second)
		for v.Screen() == before && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}
	v.t.Fatalf("could not move the cursor to %q:\n%s", sub, v.dump())
}

// Resize changes the terminal size.
func (v *viewer) Resize(w, h int) {
	v.t.Helper()
	v.mu.Lock()
	v.scr.resize(w, h)
	v.mu.Unlock()
	if err := setSize(v.master.Fd(), w, h); err != nil {
		v.t.Fatal(err)
	}
}

// Quit presses q and waits for the process to end.
func (v *viewer) Quit() {
	v.t.Helper()
	v.Keys("q")
	v.WaitExit()
}

// WaitExit waits for the process to end and returns its exit code.
func (v *viewer) WaitExit() int {
	v.t.Helper()
	select {
	case <-v.done:
	case <-time.After(waitTimeout):
		v.t.Fatalf("viewer did not exit; screen:\n%s", v.dump())
	}
	var ee *exec.ExitError
	if errors.As(v.err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 0
}

// Signal sends sig to the viewer.
func (v *viewer) Signal(sig os.Signal) {
	v.t.Helper()
	if err := v.cmd.Process.Signal(sig); err != nil {
		v.t.Fatal(err)
	}
}

// pollFile waits until cond holds for the file's content.
func pollFile(t *testing.T, path string, what string, cond func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		b, _ := os.ReadFile(path)
		if cond(string(b)) {
			return string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s in %s; content:\n%s", what, path, b)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
