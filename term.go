package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
)

// A minimal terminal layer on the standard library only. Raw mode and the
// window size go through stty(1) so there is no ioctl/unsafe code and no
// dependency; everything else is plain ANSI escape sequences.

const (
	rst     = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	rev     = "\x1b[7m"
	ul      = "\x1b[4m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	cyan    = "\x1b[36m"
	magenta = "\x1b[35m"
	blue    = "\x1b[94m"
)

func sty(code, s string) string {
	if code == "" {
		return s
	}
	return code + s + rst
}

// key is a named key ("enter", "up", ...) or, when name is empty, a typed rune.
type key struct {
	name string
	r    rune
}

type term struct {
	saved string
	w, h  int
	keys  chan key
	winch chan os.Signal
	// stop receives SIGTERM, SIGINT and SIGHUP: the viewer ends through its
	// normal path, so the terminal is given back.
	stop chan os.Signal
	// paused stops the key reader while another program (the user's editor)
	// owns the terminal.
	paused atomic.Bool
	out    io.Writer // where frames go; nil is stdout
}

func stty(args ...string) (string, error) {
	c := exec.Command("stty", args...)
	c.Stdin = os.Stdin
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

func openTerm() (*term, error) {
	saved, err := stty("-g")
	if err != nil {
		return nil, fmt.Errorf("not a terminal: %w", err)
	}
	t := &term{saved: saved, keys: make(chan key, 64), winch: make(chan os.Signal, 1), stop: make(chan os.Signal, 1)}
	// Catch the stopping signals before taking the terminal, so there is no
	// moment when one would leave it in raw mode.
	signal.Notify(t.stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	if err := t.enter(); err != nil {
		signal.Stop(t.stop)
		return nil, err
	}
	signal.Notify(t.winch, syscall.SIGWINCH)
	t.resize()
	go t.readKeys()
	return t, nil
}

// enter takes the terminal: raw mode, alternate screen, hidden cursor,
// bracketed paste. Reads time out after 0.1s (min 0 time 1) so the key reader
// can notice that it has been paused.
func (t *term) enter() error {
	if _, err := stty("raw", "-echo", "min", "0", "time", "1"); err != nil {
		return err
	}
	fmt.Print("\x1b[?1049h\x1b[?25l\x1b[?2004h")
	return nil
}

// run hands the terminal to another program and takes it back afterwards.
func (t *term) run(c *exec.Cmd) error {
	t.paused.Store(true)
	time.Sleep(150 * time.Millisecond) // let a read in flight time out
	t.close()
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	// While the editor runs, a ctrl+c in it may reach the viewer too (an
	// editor that leaves ISIG on): the viewer lets it pass. SIGINT goes to a
	// channel nobody reads, then stop stops taking it; it is caught all the
	// while, so it never falls back to ending the program. (Caught signals
	// are reset in the editor, so the editor still gets its ctrl+c.)
	ign := make(chan os.Signal, 1)
	signal.Notify(ign, syscall.SIGINT)
	signal.Stop(t.stop)
	signal.Notify(t.stop, syscall.SIGTERM, syscall.SIGHUP)
	err := c.Run()
	signal.Notify(t.stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	signal.Stop(ign)
	if e := t.enter(); e != nil {
		return e
	}
	t.resize()
	t.paused.Store(false)
	return err
}

func (t *term) close() {
	fmt.Print("\x1b[?2004l\x1b[?25h\x1b[?1049l")
	_, _ = stty(t.saved)
}

func (t *term) resize() {
	t.w, t.h = 80, 24
	if out, err := stty("size"); err == nil {
		fmt.Sscanf(out, "%d %d", &t.h, &t.w)
	}
}

// draw paints a full frame without clearing first, so there is no flicker.
// With at (row, column; 1-based), the terminal's cursor is left there and
// shown, so an input method draws what is being composed where the person
// writes; without, it is hidden. The frame and the cursor go out in one
// write, so the cursor is never seen jumping.
func (t *term) draw(lines []string, at *[2]int) {
	if len(lines) > t.h {
		lines = lines[:t.h]
	}
	// Each row is cleared before it is written, never after: once a row is
	// written up to the last column the cursor rests on that column, and
	// erasing from there would take the last character with it.
	var b strings.Builder
	b.WriteString("\x1b[H")
	for i := 0; i < t.h; i++ {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString("\x1b[2K")
		if i < len(lines) {
			b.WriteString(lines[i])
		}
	}
	if at != nil {
		fmt.Fprintf(&b, "\x1b[%d;%dH\x1b[?25h", at[0], at[1])
	} else {
		b.WriteString("\x1b[?25l")
	}
	out := t.out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprint(out, b.String())
}

func (t *term) readKeys() {
	var buf []byte
	tmp := make([]byte, 512)
	pasting := false
	for {
		if t.paused.Load() {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		n, err := os.Stdin.Read(tmp)
		if n == 0 {
			if err != nil && err != io.EOF { // EOF is only the read timing out
				close(t.keys)
				return
			}
			continue
		}
		buf = append(buf, tmp[:n]...)
		for len(buf) > 0 {
			k, size := parseKey(buf, &pasting)
			if size == 0 {
				break // incomplete sequence; wait for more bytes
			}
			buf = buf[size:]
			if k != (key{}) {
				t.keys <- k
			}
		}
	}
}

// parseKey decodes one key from the front of b and returns how many bytes it
// used (0: need more bytes). Inside a bracketed paste everything is text, so a
// pasted newline does not submit the input.
func parseKey(b []byte, pasting *bool) (key, int) {
	if b[0] == 0x1b {
		if len(b) > 1 && b[1] == '\r' {
			return key{name: "newline"}, 2 // alt+enter, and shift+enter in some terminals
		}
		if len(b) == 1 || (b[1] != '[' && b[1] != 'O') {
			return key{name: "esc"}, 1
		}
		for i := 2; i < len(b); i++ {
			if b[i] < 0x40 || b[i] > 0x7e {
				continue
			}
			name := map[string]string{
				"A": "up", "B": "down", "C": "right", "D": "left", "Z": "shift+tab",
				"5~": "pgup", "6~": "pgdn", "H": "home", "F": "end", "1~": "home", "4~": "end",
			}[string(b[2:i+1])]
			switch string(b[2 : i+1]) {
			case "27;2;13~", "13;2u":
				name = "newline" // shift+enter
			case "200~":
				*pasting = true
			case "201~":
				*pasting = false
			}
			return key{name: name}, i + 1
		}
		return key{}, 0
	}
	if !*pasting {
		switch b[0] {
		case '\r':
			return key{name: "enter"}, 1
		case '\n':
			return key{name: "newline"}, 1 // ctrl+j
		case '\t':
			return key{name: "tab"}, 1
		case 127, 8:
			return key{name: "backspace"}, 1
		case 3:
			return key{name: "ctrl+c"}, 1
		case 21:
			return key{name: "ctrl+u"}, 1
		case 5:
			return key{name: "ctrl+e"}, 1
		case 19:
			return key{name: "ctrl+s"}, 1
		}
	}
	// Inside a paste, a line break is one newline whether it came as CR,
	// LF or CRLF.
	if b[0] == '\r' {
		if len(b) > 1 && b[1] == '\n' {
			return key{r: '\n'}, 2
		}
		if len(b) == 1 {
			return key{}, 0 // an LF may follow in the next read
		}
		return key{r: '\n'}, 1
	}
	if b[0] < 0x20 && b[0] != '\n' {
		return key{}, 1
	}
	if !utf8.FullRune(b) {
		return key{}, 0
	}
	r, size := utf8.DecodeRune(b)
	return key{r: r}, size
}

// runeWidth is the number of terminal cells a rune takes: 2 for CJK, kana,
// full-width forms and emoji, 0 for combining marks, 1 otherwise.
func runeWidth(r rune) int {
	switch {
	case r >= 0x300 && r <= 0x36f, r >= 0x200b && r <= 0x200f, r >= 0xfe00 && r <= 0xfe0f:
		return 0
	case r >= 0x1100 && r <= 0x115f, r >= 0x2e80 && r <= 0xa4cf, r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff, r >= 0xfe30 && r <= 0xfe4f, r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6, r >= 0x1f300 && r <= 0x1faff, r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}

func width(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// clean makes text from the AI or from files safe to print: control
// characters (including escape sequences) are dropped, tabs become spaces.
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\n' || (r >= 0x20 && r != 0x7f && !(r >= 0x80 && r < 0xa0)):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrap breaks text into lines of at most w cells.
func wrap(s string, w int) []string {
	w = max(w, 4)
	var out []string
	for _, ln := range strings.Split(clean(s), "\n") {
		var cur []rune
		cw := 0
		for _, r := range ln {
			rw := runeWidth(r)
			if cw+rw > w {
				// A line must not start with closing punctuation: take the
				// previous character down with it.
				var carry []rune
				if strings.ContainsRune("、。，．）」』】！？!?,.)", r) && len(cur) > 1 {
					carry = []rune{cur[len(cur)-1]}
					cur = cur[:len(cur)-1]
				}
				out = append(out, string(cur))
				cur, cw = carry, width(string(carry))
			}
			cur = append(cur, r)
			cw += rw
		}
		out = append(out, string(cur))
	}
	return out
}

// trunc cuts a single line to w cells, ending in … when cut.
func trunc(s string, w int) string {
	s = strings.ReplaceAll(clean(s), "\n", " ")
	if width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(w-width(s), 0))
}

// tail keeps the end of a single line that fits in w cells.
func tail(s string, w int) string {
	r := []rune(s)
	for len(r) > 0 && width(string(r)) > w {
		r = r[1:]
	}
	return string(r)
}
