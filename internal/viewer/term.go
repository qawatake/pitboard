package viewer

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

	"github.com/qawatake/pitboard/internal/page"
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
	at   time.Time // when it was read (zero in tests)
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
	// prev is the frame on screen, so the next one writes only the rows
	// that differ (nil: the screen is not known; write every row).
	prev []string
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
// bracketed paste, and keys told apart (keysOn). Reads time out after 0.1s
// (min 0 time 1) so the key reader can notice that it has been paused.
func (t *term) enter() error {
	if _, err := stty("raw", "-echo", "min", "0", "time", "1"); err != nil {
		return err
	}
	fmt.Print("\x1b[?1049h\x1b[?25l\x1b[?2004h" + keysOn)
	return nil
}

// keysOn asks the terminal to send keys that are otherwise the same bytes
// (shift+enter and enter) differently, and keysOff gives that back. Both
// ways are asked for; a terminal ignores what it does not know:
//
//	CSI > 1 u     kitty keyboard protocol, "disambiguate" (kitty, Ghostty,
//	              WezTerm with enable_kitty_keyboard, foot, iTerm2 3.5+).
//	              Popped with CSI < u.
//	CSI > 4 ; 1 m xterm modifyOtherKeys level 1 (xterm, and tmux with
//	              extended-keys on, which passes it on to the outer
//	              terminal). Undone with CSI > 4 ; 0 m.
//	CSI ? u       asks for the kitty flags: a terminal that speaks the
//	              protocol answers CSI ? flags u (read as the key "kbd").
//	              No answer: shift+enter may still be the same as enter, so
//	              the editor says ctrl+j for a new line.
//
// With them on, esc, ctrl+letter and alt+letter may arrive as CSI 27 u,
// CSI 106 ; 5 u, CSI 27 ; 5 ; 106 ~ and so on: parseKey reads them as the
// same keys as before.
const (
	keysOn  = "\x1b[>1u\x1b[>4;1m\x1b[?u"
	keysOff = "\x1b[<u\x1b[>4;0m"
)

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
	fmt.Print(keysOff + "\x1b[?2004l\x1b[?25h\x1b[?1049l")
	_, _ = stty(t.saved)
}

func (t *term) resize() {
	t.prev = nil
	t.w, t.h = 80, 24
	if out, err := stty("size"); err == nil {
		fmt.Sscanf(out, "%d %d", &t.h, &t.w)
	}
}

// draw paints a frame without clearing the screen, so there is no
// flicker: the first frame (and the first after a resize) writes every
// row, later ones only the rows that changed. With at (row, column;
// 1-based), the terminal's cursor is left there and shown, so an input
// method draws what is being composed where the person writes; without, it
// is hidden. The frame and the cursor go out in one write, so the cursor is
// never seen jumping.
func (t *term) draw(lines []string, at *[2]int) {
	if len(lines) > t.h {
		lines = lines[:t.h]
	}
	frame := make([]string, t.h)
	copy(frame, lines)
	// However a line was made, it never runs past the width: the terminal
	// would wrap it and shift everything below.
	for i, l := range frame {
		if t.w > 0 && visibleWidth(l) > t.w {
			frame[i] = cutVisible(l, t.w)
		}
	}
	// Each row is cleared before it is written, never after: once a row is
	// written up to the last column the cursor rests on that column, and
	// erasing from there would take the last character with it.
	var b strings.Builder
	if len(t.prev) != t.h {
		b.WriteString("\x1b[H")
		for i, l := range frame {
			if i > 0 {
				b.WriteString("\r\n")
			}
			b.WriteString("\x1b[2K" + l)
		}
	} else {
		for i, l := range frame {
			if l != t.prev[i] {
				fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K%s", i+1, l)
			}
		}
	}
	t.prev = frame
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
		at := time.Now()
		for len(buf) > 0 {
			k, size := parseKey(buf, &pasting)
			if size == 0 {
				break // incomplete sequence; wait for more bytes
			}
			buf = buf[size:]
			if k != (key{}) {
				k.at = at
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
		if len(b) > 1 && !*pasting {
			switch b[1] {
			case 'b':
				return key{name: "alt+b"}, 2
			case 'f':
				return key{name: "alt+f"}, 2
			case 127, 8:
				return key{name: "alt+backspace"}, 2
			}
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
			seq := string(b[2 : i+1])
			if k, ok := extendedKey(seq); ok {
				if *pasting && k.name != "" {
					k = key{} // a key inside a paste is not a key
				}
				return k, i + 1
			}
			switch seq {
			case "3~":
				name = "delete"
			case "1;3D", "1;9D", "1;5D":
				name = "alt+left" // option+← (and ctrl+←)
			case "1;3C", "1;9C", "1;5C":
				name = "alt+right"
			case "200~":
				*pasting = true
				name = "paste-start"
			case "201~":
				*pasting = false
				name = "paste-end"
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
			return key{name: "backspace"}, 1 // ctrl+h too
		case 3:
			return key{name: "ctrl+c"}, 1
		}
		// The other control keys, by their letter (ctrl+a ... ctrl+w).
		if name, ok := ctrlKeys[b[0]]; ok {
			return key{name: name}, 1
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

// extendedKey reads a key sent in the forms keysOn asks for: CSI code u,
// CSI code ; mods u (kitty) and CSI 27 ; mods ; code ~ (modifyOtherKeys),
// seq being what follows CSI. The answer to CSI ? u is the key "kbd". ok is
// false for anything else (arrows, pgup ... are read as before).
func extendedKey(seq string) (k key, ok bool) {
	var code, mods int
	switch {
	case strings.HasPrefix(seq, "?") && strings.HasSuffix(seq, "u"):
		return key{name: "kbd"}, true
	case strings.HasSuffix(seq, "u"):
		parts := strings.Split(strings.TrimSuffix(seq, "u"), ";")
		if len(parts) > 2 {
			return key{}, false
		}
		code, mods = atoiPart(parts[0]), 1
		if len(parts) == 2 {
			mods = atoiPart(parts[1])
		}
	case strings.HasPrefix(seq, "27;") && strings.HasSuffix(seq, "~"):
		parts := strings.Split(strings.TrimSuffix(seq, "~"), ";")
		if len(parts) != 3 {
			return key{}, false
		}
		mods, code = atoiPart(parts[1]), atoiPart(parts[2])
	default:
		return key{}, false
	}
	if code <= 0 || mods <= 0 {
		return key{}, true // not readable: dropped
	}
	m := mods - 1 // bits: 1 shift, 2 alt, 4 ctrl (8 and up: super ...)
	shift, alt, ctrl := m&1 != 0, m&2 != 0, m&4 != 0
	if m&^7 != 0 {
		return key{}, true
	}
	switch code {
	case 13:
		if shift || alt || ctrl {
			return key{name: "newline"}, true
		}
		return key{name: "enter"}, true
	case 27:
		return key{name: "esc"}, true
	case 9:
		if shift {
			return key{name: "shift+tab"}, true
		}
		return key{name: "tab"}, true
	case 127, 8:
		if alt || ctrl {
			return key{name: "alt+backspace"}, true
		}
		return key{name: "backspace"}, true
	}
	r := rune(code)
	switch {
	case ctrl && !alt && r >= 'a' && r <= 'z':
		switch r {
		case 'c':
			return key{name: "ctrl+c"}, true
		case 'j':
			return key{name: "newline"}, true
		case 'h':
			return key{name: "backspace"}, true
		case 'i':
			return key{name: "tab"}, true
		case 'm':
			return key{name: "enter"}, true
		}
		if name, ok := ctrlKeys[byte(r-'a'+1)]; ok {
			return key{name: name}, true
		}
		return key{}, true
	case alt && !ctrl:
		switch r {
		case 'b':
			return key{name: "alt+b"}, true
		case 'f':
			return key{name: "alt+f"}, true
		}
		return key{}, true
	case ctrl || alt:
		return key{}, true
	}
	if !utf8.ValidRune(r) || r < 0x20 || (r >= 0xe000 && r <= 0xf8ff) {
		return key{}, true // keypad, lone modifiers and other functional keys
	}
	if shift && r >= 'a' && r <= 'z' {
		r -= 'a' - 'A'
	}
	return key{r: r}, true
}

// atoiPart reads a number from a part of a sequence ("106", or "106:74"
// with alternate keys); -1 if it is not one.
func atoiPart(s string) int {
	s, _, _ = strings.Cut(s, ":")
	n := 0
	if s == "" {
		return -1
	}
	for _, c := range s {
		if c < '0' || c > '9' || n > 0x10ffff {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ctrlKeys are the control keys the viewer reads, by their byte.
var ctrlKeys = map[byte]string{
	1: "ctrl+a", 2: "ctrl+b", 4: "ctrl+d", 5: "ctrl+e", 6: "ctrl+f", 7: "ctrl+g",
	11: "ctrl+k", 14: "ctrl+n", 16: "ctrl+p", 19: "ctrl+s", 21: "ctrl+u", 22: "ctrl+v", 23: "ctrl+w",
}

// runeWidth is the number of terminal cells a rune takes: 2 for CJK, kana,
// full-width forms and emoji, 0 for combining marks, 1 otherwise.
func runeWidth(r rune) int { return page.RuneWidth(r) }

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

// wrapPlain breaks text into lines of at most w cells.
func wrapPlain(s string, w int) []string {
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

// truncPlain cuts a single line to w cells, ending in … when cut.
func truncPlain(s string, w int) string {
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
