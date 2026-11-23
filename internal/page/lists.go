package page

import (
	"regexp"
	"strings"
)

// How a list and a run-in head word read, on every screen.
//
//   - A list (the lines of one level under one parent) is loose when any
//     of its lines takes more than one drawn line at the width it is drawn
//     at: then a blank line (the terminal) or a gap (the browser) goes
//     between all of its lines. A list whose lines all fit on one line
//     stays tight. A nested list decides for itself.
//   - A line written "**head**: body" (a list line or a paragraph) is
//     drawn run-in, as written, when it fits on one line; otherwise as a
//     definition: the head alone on its line (no colon), the body on the
//     lines under it, one step in. The head stands out by where it is, not
//     by its weight (a font may have no bold for CJK).
//   - The text of a list or a paragraph is at most TextMeasure columns wide
//     (the terminal; the browser has its own measure).

// TextMeasure is the widest a list's or a paragraph's text is drawn, in
// columns, however wide the screen.
const TextMeasure = 72

var reHeadWord = regexp.MustCompile(`^\*\*([^*\n]+)\*\*\s*[:：]\s*`)

// HeadWord splits a line written "**head**: body" (or with a full-width
// colon) into its head word and its body; ok is false for any other line.
func HeadWord(s string) (head, body string, ok bool) {
	m := reHeadWord.FindStringSubmatchIndex(s)
	if m == nil {
		return "", s, false
	}
	head = strings.TrimSpace(s[m[2]:m[3]])
	body = s[m[1]:]
	if head == "" || strings.TrimSpace(body) == "" {
		return "", s, false
	}
	return head, body, true
}

// Loose says whether a list whose lines take these many drawn lines each
// is drawn loose (a gap between all of its lines).
func Loose(lines []int) bool {
	for _, n := range lines {
		if n > 1 {
			return true
		}
	}
	return false
}
