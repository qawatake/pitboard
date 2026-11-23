package e2e_test

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// screen is a small terminal model: enough of VT100/xterm to replay what
// the viewer writes (cursor moves, erase line/screen, SGR colors, wide
// characters, OSC hyperlinks) and assert on the result.
type screen struct {
	w, h     int
	cells    [][]cell
	row, col int // 0-based cursor
	cur      attr
	cursorOn bool
	pending  []byte // an incomplete sequence or rune from the last feed
}

type attr struct {
	fg        int // 0 default, 30-37, 90-97
	bold, dim bool
	ul, rev   bool
}

type cell struct {
	r    rune // 0: empty; -1: the right half of a wide character
	attr attr
}

func newScreen(w, h int) *screen {
	s := &screen{w: w, h: h, cursorOn: true}
	s.resize(w, h)
	return s
}

func (s *screen) resize(w, h int) {
	cells := make([][]cell, h)
	for i := range cells {
		cells[i] = make([]cell, w)
		if i < len(s.cells) {
			copy(cells[i], s.cells[i])
		}
	}
	s.w, s.h, s.cells = w, h, cells
	s.row, s.col = min(s.row, h-1), min(s.col, w-1)
}

func (s *screen) feed(b []byte) {
	data := append(s.pending, b...)
	s.pending = nil
	i := 0
	for i < len(data) {
		c := data[i]
		switch {
		case c == 0x1b:
			n := s.escape(data[i:])
			if n == 0 { // incomplete
				s.pending = append([]byte(nil), data[i:]...)
				return
			}
			i += n
		case c == '\r':
			s.col = 0
			i++
		case c == '\n':
			s.lineFeed()
			i++
		case c == '\b':
			if s.col > 0 {
				s.col--
			}
			i++
		case c == '\a', c < 0x20:
			i++
		default:
			if !utf8.FullRune(data[i:]) {
				s.pending = append([]byte(nil), data[i:]...)
				return
			}
			r, n := utf8.DecodeRune(data[i:])
			s.put(r)
			i += n
		}
	}
}

func (s *screen) lineFeed() {
	if s.row < s.h-1 {
		s.row++
		return
	}
	copy(s.cells, s.cells[1:])
	s.cells[s.h-1] = make([]cell, s.w)
}

func (s *screen) put(r rune) {
	w := termWidth(r)
	if w == 0 {
		return
	}
	if s.col+w > s.w {
		s.col = 0
		s.lineFeed()
	}
	// Writing over half of a wide character blanks the other half.
	if s.cells[s.row][s.col].r == -1 && s.col > 0 {
		s.cells[s.row][s.col-1] = cell{}
	}
	s.cells[s.row][s.col] = cell{r, s.cur}
	if w == 2 && s.col+1 < s.w {
		s.cells[s.row][s.col+1] = cell{-1, s.cur}
	}
	s.col += w
	if s.col >= s.w {
		s.col = s.w - 1 // simplified: no deferred wrap beyond the last column
	}
}

// escape handles one escape sequence at the start of b and returns its
// length, or 0 when it is incomplete.
func (s *screen) escape(b []byte) int {
	if len(b) < 2 {
		return 0
	}
	switch b[1] {
	case '[':
		j := 2
		for j < len(b) && (b[j] < 0x40 || b[j] > 0x7e) {
			j++
		}
		if j >= len(b) {
			return 0
		}
		s.csi(string(b[2:j]), b[j])
		return j + 1
	case ']': // OSC, up to BEL or ST
		for j := 2; j < len(b); j++ {
			if b[j] == '\a' {
				return j + 1
			}
			if b[j] == 0x1b {
				if j+1 >= len(b) {
					return 0
				}
				return j + 2
			}
		}
		return 0
	case '(', ')':
		if len(b) < 3 {
			return 0
		}
		return 3
	default:
		return 2
	}
}

func (s *screen) csi(params string, final byte) {
	private := ""
	if params != "" && strings.ContainsRune("?<>=", rune(params[0])) {
		private, params = params[:1], params[1:]
	}
	var ps []int
	for _, p := range strings.Split(params, ";") {
		n, _ := strconv.Atoi(p)
		ps = append(ps, n)
	}
	arg := func(i, def int) int {
		if i < len(ps) && ps[i] != 0 {
			return ps[i]
		}
		return def
	}
	if private != "" {
		if private == "?" && params == "25" {
			s.cursorOn = final == 'h'
		}
		return
	}
	switch final {
	case 'H', 'f':
		s.row = clamp(arg(0, 1)-1, 0, s.h-1)
		s.col = clamp(arg(1, 1)-1, 0, s.w-1)
	case 'A':
		s.row = clamp(s.row-arg(0, 1), 0, s.h-1)
	case 'B':
		s.row = clamp(s.row+arg(0, 1), 0, s.h-1)
	case 'C':
		s.col = clamp(s.col+arg(0, 1), 0, s.w-1)
	case 'D':
		s.col = clamp(s.col-arg(0, 1), 0, s.w-1)
	case 'G':
		s.col = clamp(arg(0, 1)-1, 0, s.w-1)
	case 'd':
		s.row = clamp(arg(0, 1)-1, 0, s.h-1)
	case 'K':
		from, to := s.col, s.w
		switch arg(0, 0) {
		case 1:
			from, to = 0, s.col+1
		case 2:
			from, to = 0, s.w
		}
		for c := from; c < to && c < s.w; c++ {
			s.cells[s.row][c] = cell{}
		}
	case 'J':
		switch arg(0, 0) {
		case 2, 3:
			for r := range s.cells {
				s.cells[r] = make([]cell, s.w)
			}
		case 0:
			for c := s.col; c < s.w; c++ {
				s.cells[s.row][c] = cell{}
			}
			for r := s.row + 1; r < s.h; r++ {
				s.cells[r] = make([]cell, s.w)
			}
		}
	case 'm':
		if len(ps) == 0 {
			ps = []int{0}
		}
		for i := 0; i < len(ps); i++ {
			switch p := ps[i]; {
			case p == 0:
				s.cur = attr{}
			case p == 1:
				s.cur.bold = true
			case p == 2:
				s.cur.dim = true
			case p == 4:
				s.cur.ul = true
			case p == 7:
				s.cur.rev = true
			case p == 22:
				s.cur.bold, s.cur.dim = false, false
			case p == 24:
				s.cur.ul = false
			case p == 27:
				s.cur.rev = false
			case p == 39:
				s.cur.fg = 0
			case p >= 30 && p <= 37, p >= 90 && p <= 97:
				s.cur.fg = p
			case p == 38: // 38;5;n or 38;2;r;g;b: skip
				if i+1 < len(ps) && ps[i+1] == 5 {
					i += 2
				} else if i+1 < len(ps) && ps[i+1] == 2 {
					i += 4
				}
			}
		}
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// line returns row n (0-based) as text, without trailing spaces. A wide
// character takes one rune in the string and two columns on screen.
func (s *screen) line(n int) string {
	if n < 0 || n >= s.h {
		return ""
	}
	var b strings.Builder
	for _, c := range s.cells[n] {
		switch c.r {
		case -1:
		case 0:
			b.WriteByte(' ')
		default:
			b.WriteRune(c.r)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func (s *screen) text() string {
	lines := make([]string, s.h)
	for i := range lines {
		lines[i] = s.line(i)
	}
	return strings.Join(lines, "\n")
}

// find returns the first (row, col) where sub starts on a line, col in
// screen columns, or (-1, -1).
func (s *screen) find(sub string) (int, int) {
	for r := 0; r < s.h; r++ {
		l := s.line(r)
		if i := strings.Index(l, sub); i >= 0 {
			return r, colOf(l[:i])
		}
	}
	return -1, -1
}

// colOf is the screen width of a line prefix.
func colOf(prefix string) int {
	w := 0
	for _, r := range prefix {
		w += termWidth(r)
	}
	return w
}

// termWidth is a terminal's idea of a rune's width: East Asian wide and
// fullwidth characters and emoji take two columns.
func termWidth(r rune) int {
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

// color names the look of the cell at (row, col): "yellow", "green",
// "blue", "red", "cyan", "magenta", "dim", "bold" or "" (default). A color
// wins over dim and bold.
func (s *screen) color(row, col int) string {
	if row < 0 || row >= s.h || col < 0 || col >= s.w {
		return ""
	}
	a := s.cells[row][col].attr
	switch a.fg {
	case 33, 93:
		return "yellow"
	case 32, 92:
		return "green"
	case 34, 94:
		return "blue"
	case 31, 91:
		return "red"
	case 36, 96:
		return "cyan"
	case 35, 95:
		return "magenta"
	}
	if a.dim {
		return "dim"
	}
	if a.bold {
		return "bold"
	}
	return ""
}

func (s *screen) attrAt(row, col int) attr {
	if row < 0 || row >= s.h || col < 0 || col >= s.w {
		return attr{}
	}
	return s.cells[row][col].attr
}
