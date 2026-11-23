package main

import "unicode"

// edit is the text being written and the cursor in it, apart from the
// screen: each editing key is a method. pos is a rune index (0..len). goal
// is the column up and down aim for, kept while they are pressed in a row
// (-1: none).
type edit struct {
	text []rune
	pos  int
	goal int
}

func newEdit(s string) edit {
	r := []rune(s)
	return edit{text: r, pos: len(r), goal: -1}
}

func (e *edit) String() string { return string(e.text) }

// zwj joins the characters on its two sides into one.
const zwj = 0x200d

// boundary says whether i (0..len) is between two characters as the person
// sees them: never before a zero-width rune (a combining mark, a variation
// selector, a joiner) or right after a joiner.
func (e *edit) boundary(i int) bool {
	if i <= 0 || i >= len(e.text) {
		return true
	}
	return runeWidth(e.text[i]) != 0 && e.text[i-1] != zwj
}

func (e *edit) prev(i int) int {
	for i--; i > 0 && !e.boundary(i); i-- {
	}
	return max(i, 0)
}

func (e *edit) next(i int) int {
	for i++; i < len(e.text) && !e.boundary(i); i++ {
	}
	return min(i, len(e.text))
}

// lineStart and lineEnd are the ends of the line (between newlines) holding i.
func (e *edit) lineStart(i int) int {
	for i > 0 && e.text[i-1] != '\n' {
		i--
	}
	return i
}

func (e *edit) lineEnd(i int) int {
	for i < len(e.text) && e.text[i] != '\n' {
		i++
	}
	return i
}

// cut removes text[from:to] and leaves the cursor at from.
func (e *edit) cut(from, to int) {
	if from >= to {
		return
	}
	e.text = append(e.text[:from:from], e.text[to:]...)
	e.pos = from
}

// Insert puts rs at the cursor (typed or pasted text).
func (e *edit) Insert(rs ...rune) {
	e.goal = -1
	e.text = append(e.text[:e.pos:e.pos], append(append([]rune{}, rs...), e.text[e.pos:]...)...)
	e.pos += len(rs)
}

func (e *edit) Left()  { e.goal, e.pos = -1, e.prev(e.pos) }
func (e *edit) Right() { e.goal, e.pos = -1, e.next(e.pos) }
func (e *edit) Home()  { e.goal, e.pos = -1, e.lineStart(e.pos) }
func (e *edit) End()   { e.goal, e.pos = -1, e.lineEnd(e.pos) }

// Backspace removes the character left of the cursor; Delete the one right
// of it.
func (e *edit) Backspace() { e.goal = -1; e.cut(e.prev(e.pos), e.pos) }
func (e *edit) Delete()    { e.goal = -1; e.cut(e.pos, e.next(e.pos)) }

// KillToEnd removes from the cursor to the end of the line; at the end of a
// line, the newline (the lines are joined).
func (e *edit) KillToEnd() {
	e.goal = -1
	end := e.lineEnd(e.pos)
	if end == e.pos && end < len(e.text) {
		end++
	}
	e.cut(e.pos, end)
}

// KillToStart removes from the start of the line to the cursor.
func (e *edit) KillToStart() { e.goal = -1; e.cut(e.lineStart(e.pos), e.pos) }

// class is what kind of character r is, for moving by words: spaces, kana
// and kanji, letters and digits, and each other symbol on its own.
func class(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.Is(unicode.Hiragana, r):
		return 1
	case unicode.Is(unicode.Katakana, r) || r == 'ー' || r == 'ｰ':
		return 2
	case unicode.Is(unicode.Han, r) || r == '々':
		return 3
	case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || unicode.Is(unicode.Mn, r):
		return 4
	}
	return 5 + int(r) // every other symbol is a word of its own kind
}

// wordLeft is where the word left of i begins: spaces are skipped, then
// characters of one kind.
func (e *edit) wordLeft(i int) int {
	for i > 0 && class(e.text[i-1]) == 0 {
		i--
	}
	if i == 0 {
		return 0
	}
	c := class(e.text[e.prev(i)])
	for i > 0 && class(e.text[e.prev(i)]) == c {
		i = e.prev(i)
	}
	return i
}

// wordRight is where the word right of i ends.
func (e *edit) wordRight(i int) int {
	for i < len(e.text) && class(e.text[i]) == 0 {
		i++
	}
	if i == len(e.text) {
		return i
	}
	c := class(e.text[i])
	for i < len(e.text) && class(e.text[i]) == c {
		i = e.next(i)
	}
	return i
}

func (e *edit) WordLeft()     { e.goal, e.pos = -1, e.wordLeft(e.pos) }
func (e *edit) WordRight()    { e.goal, e.pos = -1, e.wordRight(e.pos) }
func (e *edit) KillWordLeft() { e.goal = -1; e.cut(e.wordLeft(e.pos), e.pos) }

// vrow is one row of the text as drawn at a width: runes [start, end), and
// whether a newline (or the end of the text) ends it.
type vrow struct {
	start, end int
	last       bool
}

// rows lays the text out in rows of at most w cells, breaking anywhere
// between characters.
func (e *edit) rows(w int) []vrow {
	w = max(w, 2)
	var out []vrow
	start, cw := 0, 0
	for i := 0; i <= len(e.text); i++ {
		if i == len(e.text) || e.text[i] == '\n' {
			out = append(out, vrow{start, i, true})
			start, cw = i+1, 0
			continue
		}
		rw := runeWidth(e.text[i])
		if cw+rw > w && i > start && e.boundary(i) {
			out = append(out, vrow{start, i, false})
			start, cw = i, 0
		}
		cw += rw
	}
	return out
}

// colOf is the column of rune index i in row r.
func (e *edit) colOf(r vrow, i int) int {
	c := 0
	for _, x := range e.text[r.start:i] {
		c += runeWidth(x)
	}
	return c
}

// Cursor is the row and column of the cursor in the layout at width w. At
// the end of a wrapped row, the cursor is at the start of the next.
func (e *edit) Cursor(w int) (row, col int) {
	rs := e.rows(w)
	for i, r := range rs {
		if e.pos >= r.start && (e.pos < r.end || (e.pos == r.end && r.last)) {
			return i, e.colOf(r, e.pos)
		}
	}
	return len(rs) - 1, e.colOf(rs[len(rs)-1], rs[len(rs)-1].end)
}

// vertical moves the cursor d rows at width w, to the character boundary
// nearest the goal column (wide characters count 2).
func (e *edit) vertical(d, w int) {
	rs := e.rows(w)
	row, col := e.Cursor(w)
	if e.goal < 0 {
		e.goal = col
	}
	t := row + d
	if t < 0 || t >= len(rs) {
		return
	}
	r := rs[t]
	best, bd := r.start, -1
	for i := r.start; i <= r.end; i++ {
		if i == r.end && !r.last {
			break // the end of a wrapped row is the start of the next
		}
		if i > r.start && i < r.end && !e.boundary(i) {
			continue
		}
		if dist := abs(e.colOf(r, i) - e.goal); bd < 0 || dist < bd {
			best, bd = i, dist
		}
	}
	e.pos = best
}

func (e *edit) Up(w int)   { e.vertical(-1, w) }
func (e *edit) Down(w int) { e.vertical(1, w) }
