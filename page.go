package main

import (
	"fmt"
	"regexp"
	"strings"
)

// The report (named parts, see report.go) is read into a tree of nodes.

type kind int

const (
	kTitle    kind = iota // Title / # 題
	kSection              // Section / ## 見出し
	kTodo                 // Item state=todo
	kDone                 // Item state=done
	kDoing                // Item state=doing
	kQuestion             // Ask
	kChoice               // Option
	kPlain                // "- " list line (text)
	kLink                 // Link
	kDetails              // Fold / <details>
	kPara                 // paragraph
	kCode                 // Code / ```
)

type node struct {
	kind   kind
	text   string // inline Markdown (code spans, links, bold)
	id     string
	label  string // Option key: "a"
	status string // short word on the right end
	reason string // note: why, shown when opened
	ask    string // Item / Link: "y" when the AI asks for y / n on it
	wait   string // Item: "you" when it cannot move on without the person
	look   string // Item / Link / Fold (/ Ask): "y" when the AI wants it read
	path   string // Link: a file path, as written
	url    string // Link: an http(s) URL
	lang   string // Code language ("diff" is colored)
	code   []string
	body   []*node // shown when an item is opened: paragraphs, list lines, code
	kids   []*node // Section / Fold contents, Ask options
	line   int     // 1-based line in the file where the part starts
	depth  int     // indentation level: Fold contents and Ask options are nested
	key    string  // cursor identity: the id, or "fold:<n>" for a Fold
	// A list line (kPlain): num is its number as written ("1.") when it is
	// numbered, level its nesting (0-2), subs the lines nested under it.
	// Nested lines are read only: the cursor never stops at them.
	num   string
	level int
	subs  []*node
	head  bool // kPara: a line of only **…**, a small heading
	gap   bool // a blank line came before it in the text
}

func (n *node) item() bool {
	return n.kind == kTodo || n.kind == kDone || n.kind == kDoing
}

// part is the kind of part, with an Item's states counted as one.
func (n *node) part() string {
	switch {
	case n.item():
		return "Item"
	case n.kind == kQuestion:
		return "Ask"
	case n.kind == kLink:
		return "Link"
	case n.kind == kDetails:
		return "Fold"
	}
	return fmt.Sprint(n.kind)
}

// title is the line's text as the person reads it.
func (n *node) title() string { return plainOf(n.text) }

// sig is the line as the person saw it when reacting. If the AI rewrites the
// line (its text, state, status or note, a Link's target, or the options of
// an Ask), an
// earlier reaction no longer applies and is not shown.
func (n *node) sig() string {
	s := fmt.Sprintf("%d|%s|%s|%s|%s%s", n.kind, n.text, n.status, n.reason, n.path, n.url)
	if n.kind == kQuestion {
		for _, c := range n.kids {
			s += "|" + c.label + "=" + c.text
		}
	}
	return s
}

// choice returns the option of an Ask with the given key.
func (n *node) choice(label string) *node {
	for _, c := range n.kids {
		if c.kind == kChoice && c.label == label {
			return c
		}
	}
	return nil
}

// assignKeys sets the cursor key: the id for parts that have one, and for a
// Fold without an id, its order among such folds. A Fold's title is not in
// its key, so retitling it ("Done (2)" -> "Done (3)") keeps it open.
// Every node gets a unique key, so tracking never mixes two lines up.
//
// A plain "- " line is keyed by its text (and which one it is among lines
// with the same text), so the cursor stays on it when lines are added above
// and leaves it when its text changes.
func assignKeys(nodes []*node) {
	nf := 0
	plain := map[string]int{}
	walkAll(nodes, func(n *node) {
		switch {
		case n.id != "":
			n.key = n.id
		case n.kind == kDetails:
			nf++
			n.key = fmt.Sprintf("fold:%d", nf)
		case n.kind == kPlain:
			plain[n.text]++
			n.key = fmt.Sprintf("-%d:%s", plain[n.text], n.text)
		}
	})
}

// hideKey is what a line says, for hiding it: the person hides the line as
// it is now, and when the AI changes it, it is a new line and shows again.
// "" for lines that cannot be hidden.
//
//	plain line      its text (two lines with the same text hide together)
//	Item Ask Link   kind, id, title, state, status
//	Fold            kind, and its id or title
func hideKey(n *node) string {
	switch {
	case n.kind == kPlain:
		return "-|" + n.text + subsText(n.subs)
	case n.item() || n.kind == kQuestion || n.kind == kLink:
		if n.id == "" {
			return ""
		}
		return strings.Join([]string{n.part(), n.id, n.text, n.state(), n.status}, "|")
	case n.kind == kDetails:
		if n.id != "" {
			return "Fold|id=" + n.id
		}
		return "Fold|title=" + n.text
	}
	return ""
}

// subsText is the nested lines under a list line, for its keys ("" when
// there are none, so keys of lines without nesting stay as they were).
func subsText(ns []*node) string {
	s := ""
	for _, n := range ns {
		s += "\n" + strings.Repeat("  ", n.level) + n.num + "-" + n.text + subsText(n.subs)
	}
	return s
}

// state is an Item's state attribute, "" for other parts.
func (n *node) state() string {
	switch n.kind {
	case kTodo:
		return "todo"
	case kDoing:
		return "doing"
	case kDone:
		return "done"
	}
	return ""
}

// walkAll visits every node, opened or not.
func walkAll(ns []*node, f func(*node)) {
	for _, n := range ns {
		f(n)
		walkAll(n.kids, f)
	}
}

// byID finds the node with the given id.
func byID(ns []*node, id string) *node {
	var found *node
	walkAll(ns, func(n *node) {
		if found == nil && id != "" && n.id == id {
			found = n
		}
	})
	return found
}

// joinLines joins two source lines of a paragraph: with a space between
// words, with nothing between Japanese (or other wide) characters.
func joinLines(a, b string) string {
	if a == "" {
		return b
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) > 0 && len(rb) > 0 && (runeWidth(ra[len(ra)-1]) == 2 || runeWidth(rb[0]) == 2) {
		return a + b
	}
	return a + " " + b
}

var reMarkup = regexp.MustCompile("`([^`]+)`|\\[([^\\]]+)\\]\\([^)]*\\)|\\*\\*([^*]+)\\*\\*")

// plainOf drops inline markup, for text that goes to the AI or a title.
func plainOf(s string) string {
	return reMarkup.ReplaceAllString(s, "$1$2$3")
}

// setDepth indents the contents of folds and the options of asks; the
// contents of a section stay at its level.
func setDepth(ns []*node, depth int) {
	for _, n := range ns {
		n.depth = depth
		switch n.kind {
		case kSection:
			setDepth(n.kids, depth)
		default:
			setDepth(n.kids, depth+1)
		}
	}
}
