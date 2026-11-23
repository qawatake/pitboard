package report

import (
	"fmt"
	"regexp"
	"strings"
)

// The report (named parts, see parse.go) is read into a tree of nodes.

type Kind int

const (
	KindTitle    Kind = iota // Title / # 題
	KindSection              // Section / ## 見出し
	KindTodo                 // Item state=todo
	KindDone                 // Item state=done
	KindDoing                // Item state=doing
	KindQuestion             // Ask
	KindChoice               // Option
	KindPlain                // "- " list line (text)
	KindLink                 // Link
	KindDetails              // Fold / <details>
	KindPara                 // paragraph
	KindCode                 // Code / ```
	KindNotice               // Notice: fixed at the top of the screen
)

type Node struct {
	Kind   Kind
	Text   string // inline Markdown (code spans, links, bold)
	ID     string
	Label  string // Option key: "a"
	Status string // short word on the right end
	Reason string // note: why, shown when opened
	Ask    string // Item / Link: "y" when the AI asks for y / n on it
	Wait   string // Item: "you" when it cannot move on without the person
	Look   string // Item / Link / Fold (/ Ask): "y" when the AI wants it read
	Path   string // Link: a file path, as written
	URL    string // Link: an http(s) URL
	Lang   string // Code language ("diff" is colored)
	Code   []string
	Body   []*Node // shown when an item is opened: paragraphs, list lines, code
	Kids   []*Node // Section / Fold contents, Ask options
	line   int     // 1-based line in the file where the part starts
	Depth  int     // indentation level: Fold contents and Ask options are nested
	Key    string  // cursor identity: the id, or "fold:<n>" for a Fold
	// A list line (KindPlain): num is its number as written ("1.") when it is
	// numbered, level its nesting (0-2), subs the lines nested under it.
	// Nested lines are read only: the cursor never stops at them.
	Num   string
	Level int
	Subs  []*Node
	Head  bool // KindPara: a line of only **…**, a small heading
	Gap   bool // a blank line came before it in the text
}

func (n *Node) item() bool {
	return n.Kind == KindTodo || n.Kind == KindDone || n.Kind == KindDoing
}

// Part is the kind of part, with an Item's states counted as one.
func (n *Node) Part() string {
	switch {
	case n.item():
		return "Item"
	case n.Kind == KindQuestion:
		return "Ask"
	case n.Kind == KindLink:
		return "Link"
	case n.Kind == KindDetails:
		return "Fold"
	case n.Kind == KindNotice:
		return "Notice"
	}
	return fmt.Sprint(n.Kind)
}

// Title is the line's text as the person reads it.
func (n *Node) Title() string { return PlainOf(n.Text) }

// Sig is the line as the person saw it when reacting. If the AI rewrites the
// line (its text, state, status or note, a Link's target, or the options of
// an Ask), an
// earlier reaction no longer applies and is not shown.
func (n *Node) Sig() string {
	s := fmt.Sprintf("%d|%s|%s|%s|%s%s", n.Kind, n.Text, n.Status, n.Reason, n.Path, n.URL)
	if n.Kind == KindQuestion {
		for _, c := range n.Kids {
			s += "|" + c.Label + "=" + c.Text
		}
	}
	return s
}

// Choice returns the option of an Ask with the given key.
func (n *Node) Choice(label string) *Node {
	for _, c := range n.Kids {
		if c.Kind == KindChoice && c.Label == label {
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
func assignKeys(nodes []*Node) {
	nf := 0
	plain := map[string]int{}
	WalkAll(nodes, func(n *Node) {
		switch {
		case n.ID != "":
			n.Key = n.ID
		case n.Kind == KindDetails:
			nf++
			n.Key = fmt.Sprintf("fold:%d", nf)
		case n.Kind == KindPlain:
			plain[n.Text]++
			n.Key = fmt.Sprintf("-%d:%s", plain[n.Text], n.Text)
		}
	})
}

// HideKey is what a line says, for hiding it: the person hides the line as
// it is now, and when the AI changes it, it is a new line and shows again.
// "" for lines that cannot be hidden.
//
//	plain line      its text (two lines with the same text hide together)
//	Item Ask Link   kind, id, title, state, status
//	Notice          kind, id, text
//	Fold            kind, and its id or title
func HideKey(n *Node) string {
	switch {
	case n.Kind == KindPlain:
		return "-|" + n.Text + subsText(n.Subs)
	case n.item() || n.Kind == KindQuestion || n.Kind == KindLink || n.Kind == KindNotice:
		if n.ID == "" {
			return ""
		}
		return strings.Join([]string{n.Part(), n.ID, n.Text, n.state(), n.Status}, "|")
	case n.Kind == KindDetails:
		if n.ID != "" {
			return "Fold|id=" + n.ID
		}
		return "Fold|title=" + n.Text
	}
	return ""
}

// subsText is the nested lines under a list line, for its keys ("" when
// there are none, so keys of lines without nesting stay as they were).
func subsText(ns []*Node) string {
	s := ""
	for _, n := range ns {
		s += "\n" + strings.Repeat("  ", n.Level) + n.Num + "-" + n.Text + subsText(n.Subs)
	}
	return s
}

// state is an Item's state attribute, "" for other parts.
func (n *Node) state() string {
	switch n.Kind {
	case KindTodo:
		return "todo"
	case KindDoing:
		return "doing"
	case KindDone:
		return "done"
	}
	return ""
}

// WalkAll visits every node, opened or not.
func WalkAll(ns []*Node, f func(*Node)) {
	for _, n := range ns {
		f(n)
		WalkAll(n.Kids, f)
	}
}

// ByID finds the node with the given id.
func ByID(ns []*Node, id string) *Node {
	var found *Node
	WalkAll(ns, func(n *Node) {
		if found == nil && id != "" && n.ID == id {
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
	if len(ra) > 0 && len(rb) > 0 && (wide(ra[len(ra)-1]) || wide(rb[0])) {
		return a + b
	}
	return a + " " + b
}

// wide says whether r takes two terminal cells: CJK, kana, full-width
// forms and emoji (the same ranges as the viewer's runeWidth).
func wide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f, r >= 0x2e80 && r <= 0xa4cf, r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff, r >= 0xfe30 && r <= 0xfe4f, r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6, r >= 0x1f300 && r <= 0x1faff, r >= 0x20000 && r <= 0x3fffd:
		return true
	}
	return false
}

var reMarkup = regexp.MustCompile("`([^`]+)`|\\[([^\\]]+)\\]\\([^)]*\\)|\\*\\*([^*]+)\\*\\*")

// PlainOf drops inline markup, for text that goes to the AI or a title.
func PlainOf(s string) string {
	return reMarkup.ReplaceAllString(s, "$1$2$3")
}

// setDepth indents the contents of folds and the options of asks; the
// contents of a section stay at its level.
func setDepth(ns []*Node, depth int) {
	for _, n := range ns {
		n.Depth = depth
		switch n.Kind {
		case KindSection:
			setDepth(n.Kids, depth)
		default:
			setDepth(n.Kids, depth+1)
		}
	}
}
