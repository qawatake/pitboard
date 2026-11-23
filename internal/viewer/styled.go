package viewer

import (
	"strings"

	"github.com/qawatake/pitboard/internal/page"
)

// Styled text: one style per rune, so that wrapping and truncating can work
// on cells without cutting escape sequences.

type cell struct {
	r    rune
	st   string
	link string // a URL: the cell is part of an OSC 8 hyperlink
}

type text []cell

func plainText(s, st string) text {
	var t text
	for _, r := range clean(s) {
		t = append(t, cell{r: r, st: st})
	}
	return t
}

// inline renders inline Markdown (page.Spans): code spans colored (no
// backticks), links (title only) and **bold** (no asterisks).
func inline(s, base string) text {
	var t text
	for _, sp := range page.Spans(s) {
		st := base
		switch sp.Style {
		case page.StyleCode:
			st += cyan
		case page.StyleLink:
			st += blue
		case page.StyleBold:
			st += bold
		}
		t = append(t, plainText(sp.Text, st)...)
	}
	return t
}

func (t text) width() int {
	w := 0
	for _, c := range t {
		w += runeWidth(c.r)
	}
	return w
}

// hyperlink marks t as a link to url (OSC 8: terminals that support it
// make it clickable; others show the text as it is).
func (t text) hyperlink(url string) text {
	out := make(text, len(t))
	for i, c := range t {
		c.link = url
		out[i] = c
	}
	return out
}

func (t text) String() string {
	var b strings.Builder
	cur, link := "", ""
	for _, c := range t {
		if c.link != link {
			if link != "" {
				b.WriteString("\x1b]8;;\x1b\\")
			}
			if c.link != "" {
				b.WriteString("\x1b]8;;" + safeURL(c.link) + "\x1b\\")
			}
			link = c.link
		}
		if c.st != cur {
			if cur != "" {
				b.WriteString(rst)
			}
			b.WriteString(c.st)
			cur = c.st
		}
		b.WriteRune(printable(c.r))
	}
	if link != "" {
		b.WriteString("\x1b]8;;\x1b\\")
	}
	if cur != "" {
		b.WriteString(rst)
	}
	return b.String()
}

// safeURL keeps only printable ASCII in a URL put into an escape sequence,
// so a URL cannot end the sequence early or inject another.
func safeURL(u string) string {
	var b strings.Builder
	for _, r := range u {
		if r > 0x20 && r < 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// trunc cuts t to w cells, ending in … when cut. It looks at each cell
// once, so a very long line costs no more than its length.
func (t text) trunc(w int) text {
	keep, acc := 0, 0 // keep: how many cells fit in w-1 (room for the …)
	for i, c := range t {
		rw := runeWidth(c.r)
		if acc+rw > w {
			st := ""
			if keep > 0 {
				st = t[keep-1].st
			}
			return append(append(text{}, t[:keep]...), cell{r: '…', st: st})
		}
		acc += rw
		if acc <= w-1 {
			keep = i + 1
		}
	}
	return t
}

// capped keeps at most n cells of t, ending in … when cut. Titles go
// through it before they are drawn, so no line costs more than a few
// screens' worth of work however long the AI made it.
func (t text) capped(n int) text {
	if len(t) <= n {
		return t
	}
	return append(append(text{}, t[:n-1]...), cell{r: '…', st: t[n-2].st})
}

// wrap breaks t into lines of at most w cells. ASCII words are kept whole
// when possible; CJK breaks anywhere except before closing punctuation.
func (t text) wrap(w int) []text {
	w = max(w, 4)
	var out []text
	var cur text
	cw := 0
	for _, c := range t {
		if c.r == '\n' { // a line break the AI asked for
			out = append(out, cur)
			cur, cw = nil, 0
			continue
		}
		rw := runeWidth(c.r)
		if cw+rw > w {
			var carry text
			if c.r != ' ' {
				// back up to the start of an ASCII word
				i := len(cur)
				for i > 0 && cur[i-1].r < 0x80 && cur[i-1].r != ' ' {
					i--
				}
				if i > 0 && i < len(cur) && c.r < 0x80 {
					carry = append(carry, cur[i:]...)
					cur = cur[:i]
				} else if strings.ContainsRune("、。，．）」』】！？!?,.)", c.r) && len(cur) > 1 {
					carry = text{cur[len(cur)-1]}
					cur = cur[:len(cur)-1]
				}
			}
			for len(cur) > 0 && cur[len(cur)-1].r == ' ' {
				cur = cur[:len(cur)-1]
			}
			out = append(out, cur)
			cur, cw = carry, carry.width()
			if c.r == ' ' && len(cur) == 0 {
				continue
			}
		}
		cur = append(cur, c)
		cw += rw
	}
	return append(out, cur)
}

// printable keeps control characters (newlines, tabs, escapes, C1 codes)
// from reaching the terminal: every text drawn goes through String, which
// shows them as a space.
func printable(r rune) rune {
	if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
		return ' '
	}
	return r
}

// plain is the text without its styles.
func (t text) plain() string {
	r := make([]rune, len(t))
	for i, c := range t {
		r[i] = c.r
	}
	return string(r)
}
