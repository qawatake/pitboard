package main

import (
	"strings"
	"testing"
)

// ed makes an edit from s, with the cursor at "|" (or at the end).
func ed(s string) *edit {
	i := strings.Index(s, "|")
	if i < 0 {
		e := newEdit(s)
		return &e
	}
	e := newEdit(s[:i] + s[i+1:])
	e.pos = len([]rune(s[:i]))
	return &e
}

// show is the text with "|" at the cursor.
func (e *edit) show() string {
	return string(e.text[:e.pos]) + "|" + string(e.text[e.pos:])
}

func TestEditKeys(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		do   func(e *edit)
		want string
	}{
		// moving by characters
		{"left", "ab|c", (*edit).Left, "a|bc"},
		{"left at the start", "|abc", (*edit).Left, "|abc"},
		{"right at the end", "abc|", (*edit).Right, "abc|"},
		{"left over wide", "日本|語", (*edit).Left, "日|本語"},
		{"left over emoji", "a😀|b", (*edit).Left, "a|😀b"},
		{"left over a combining mark", "aé|x", (*edit).Left, "a|éx"},
		{"right over a combining mark", "a|éx", (*edit).Right, "aé|x"},
		{"right over a joined emoji", "|👩‍💻x", (*edit).Right, "👩‍💻|x"},
		{"left across a newline", "a\n|b", (*edit).Left, "a|\nb"},
		{"left on empty", "|", (*edit).Left, "|"},
		// line ends
		{"home", "ab\ncd|e", (*edit).Home, "ab\n|cde"},
		{"home at line start", "ab\n|cd", (*edit).Home, "ab\n|cd"},
		{"end", "a|b\ncd", (*edit).End, "ab|\ncd"},
		{"end on the last line", "ab\nc|d", (*edit).End, "ab\ncd|"},
		{"home on empty", "|", (*edit).Home, "|"},
		// words
		{"word left", "hello wor|ld", (*edit).WordLeft, "hello |world"},
		{"word left over spaces", "hello   |world", (*edit).WordLeft, "|hello   world"},
		{"word left by kind", "日本語のテキスト|", (*edit).WordLeft, "日本語の|テキスト"},
		{"word left kana", "日本語の|テキスト", (*edit).WordLeft, "日本語|のテキスト"},
		{"word left at start", "|abc", (*edit).WordLeft, "|abc"},
		{"word right", "|hello world", (*edit).WordRight, "hello| world"},
		{"word right over spaces", "hello| world", (*edit).WordRight, "hello world|"},
		{"word right by kind", "|日本語をpitboardで", (*edit).WordRight, "日本語|をpitboardで"},
		{"word right ascii", "日本語を|pitboardで", (*edit).WordRight, "日本語をpitboard|で"},
		{"word right at end", "abc|", (*edit).WordRight, "abc|"},
		// deleting
		{"backspace", "ab|c", (*edit).Backspace, "a|c"},
		{"backspace at start", "|abc", (*edit).Backspace, "|abc"},
		{"backspace wide", "日本|", (*edit).Backspace, "日|"},
		{"backspace combining", "aé|", (*edit).Backspace, "a|"},
		{"backspace joins lines", "a\n|b", (*edit).Backspace, "a|b"},
		{"backspace on empty", "|", (*edit).Backspace, "|"},
		{"delete", "a|bc", (*edit).Delete, "a|c"},
		{"delete at end", "abc|", (*edit).Delete, "abc|"},
		{"delete emoji", "|😀x", (*edit).Delete, "|x"},
		{"delete on empty", "|", (*edit).Delete, "|"},
		{"kill word left", "hello wor|ld", (*edit).KillWordLeft, "hello |ld"},
		{"kill word left with spaces", "hello  |", (*edit).KillWordLeft, "|"},
		{"kill word left kanji", "思いつきを書く|", (*edit).KillWordLeft, "思いつきを書|"},
		{"kill word left at start", "|abc", (*edit).KillWordLeft, "|abc"},
		{"kill to end", "ab|cd\nef", (*edit).KillToEnd, "ab|\nef"},
		{"kill to end at line end joins", "ab|\nef", (*edit).KillToEnd, "ab|ef"},
		{"kill to end at text end", "ab|", (*edit).KillToEnd, "ab|"},
		{"kill to start", "ab\ncd|ef", (*edit).KillToStart, "ab\n|ef"},
		{"kill to start at line start", "ab\n|ef", (*edit).KillToStart, "ab\n|ef"},
		// typing
		{"insert in the middle", "a|c", func(e *edit) { e.Insert('b') }, "ab|c"},
		{"insert a paste", "a|d", func(e *edit) { e.Insert([]rune("b\nc")...) }, "ab\nc|d"},
		{"insert into empty", "|", func(e *edit) { e.Insert('日') }, "日|"},
	} {
		e := ed(c.in)
		c.do(e)
		if got := e.show(); got != c.want {
			t.Errorf("%s: %q -> %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestEditUpDown(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		w    int
		up   bool
		want string
	}{
		{"down to a shorter line", "abc|def\nxy", 20, false, "abcdef\nxy|"},
		{"up to the same column", "abcdef\nxy|", 20, true, "ab|cdef\nxy"},
		{"up on the first row", "ab|c", 20, true, "ab|c"},
		{"down on the last row", "ab|c", 20, false, "ab|c"},
		// wrapped at width 4: "abcd" "efgh" "ij"
		{"down across a wrap", "a|bcdefghij", 4, false, "abcde|fghij"},
		{"up across a wrap", "abcdefghi|j", 4, true, "abcde|fghij"},
		{"down to the last wrapped row end", "abcdefg|hij", 4, false, "abcdefghij|"},
		// wide characters count 2: "あいう" is 6 cells; column 3 is between
		// い and う, rounded to the nearer boundary (2 or 4, the left one)
		{"down onto wide", "abc|\nあいう", 20, false, "abc\nあ|いう"},
		{"up from wide", "abcdef\nあい|う", 20, true, "abcd|ef\nあいう"},
		{"empty text", "|", 20, false, "|"},
		{"down to an empty line", "ab|\n", 20, false, "ab\n|"},
	} {
		e := ed(c.in)
		if c.up {
			e.Up(c.w)
		} else {
			e.Down(c.w)
		}
		if got := e.show(); got != c.want {
			t.Errorf("%s: %q -> %q, want %q", c.name, c.in, got, c.want)
		}
	}
	// The goal column is kept through a short line.
	e := ed("abcde|\nx\nabcdefg")
	e.Down(20)
	e.Down(20)
	if got := e.show(); got != "abcde\nx\nabcde|fg" {
		t.Errorf("goal column: %q", got)
	}
}

func TestEditCursor(t *testing.T) {
	for _, c := range []struct {
		in       string
		w        int
		row, col int
	}{
		{"|", 10, 0, 0},
		{"abc|", 10, 0, 3},
		{"日本|語", 10, 0, 4},
		{"ab\n|", 10, 1, 0},
		{"abcd|ef", 4, 1, 0}, // the end of a wrapped row is the start of the next
		{"abcd|", 4, 0, 4},   // the end of the text stays on its row
		{"ab́|c", 10, 0, 2},
		{"あいう|", 5, 1, 2}, // あい on the first row (4 cells), う on the second
	} {
		e := ed(c.in)
		if r, col := e.Cursor(c.w); r != c.row || col != c.col {
			t.Errorf("%q at width %d: (%d, %d), want (%d, %d)", c.in, c.w, r, col, c.row, c.col)
		}
	}
}
