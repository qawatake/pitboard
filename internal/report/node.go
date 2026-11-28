package report

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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
	Wait   string // Item: "you" when it cannot move on without the person
	Look   string // Item / Link / Fold (/ Ask): "y" when the AI wants it read
	// At is when the AI put up a Notice (local time; zero when not given).
	// Not part of what the line is: it never shows a hidden notice again.
	At    time.Time
	Path  string // Link: a file path, as written
	URL   string // Link: an http(s) URL
	Lang  string // Code language ("diff" is colored)
	Code  []string
	Body  []*Node // shown when an item is opened: paragraphs, list lines, code
	Kids  []*Node // Section / Fold contents, Ask options
	line  int     // 1-based line in the file where the part starts
	Depth int     // indentation level: Fold contents and Ask options are nested
	Key   string  // cursor identity: the id, or "fold:<n>" for a Fold
	// A list line (KindPlain): num is its number as written ("1.") when it is
	// numbered, level its nesting (0-2), subs the lines nested under it.
	// Nested lines are read only: the cursor never stops at them.
	Num   string
	Level int
	Subs  []*Node
	Head  bool // KindPara: a line of only **…**, a small heading
	Gap   bool // a blank line came before it in the text
	// Up is the part n is in (a Section, Fold or Ask for what it holds;
	// the line whose Body or Subs it is in), nil at the top. Inner says it
	// is in a Body or Subs: what an opened line shows, or a nested list
	// line.
	Up    *Node
	Inner bool
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

// Title is the line's text as the person reads it (a code block's: its
// first line that is not blank).
func (n *Node) Title() string {
	if n.Kind == KindCode && n.Text == "" {
		return codeHead(n.Code)
	}
	return PlainOf(n.Text)
}

// codeHead is the first line of code that is not blank, trimmed.
func codeHead(code []string) string {
	for _, l := range code {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

// Row says whether n is a row: a Fold, a part with an id, or a plain "- "
// line of the report itself (not numbered, not in a body, not nested).
// The other lines the cursor stops at (Line) are text: a comment can
// point at them, nothing else acts on them.
func Row(n *Node) bool {
	return n.Kind == KindDetails || n.ID != "" || (n.Kind == KindPlain && n.Num == "" && !n.Inner)
}

// Line says whether n is a line of text the cursor stops at that is not
// a Row: the title, a heading, a paragraph, a code block, a list line
// (numbered, nested, or in an opened line's body). Not an Ask's option
// (that is part of its Ask's row).
func Line(n *Node) bool {
	if Row(n) {
		return false
	}
	switch n.Kind {
	case KindTitle, KindSection, KindPara, KindCode, KindPlain:
		return true
	}
	return false
}

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
//
// The other lines of text (Line) are keyed the same way, by a letter for
// their kind (t title, h heading, p paragraph, c code, - list), and those
// in a Body or Subs under the key of the line they are in.
func assignKeys(nodes []*Node) {
	nf := 0
	plain := map[string]int{}
	var inner func(up *Node, ns []*Node)
	inner = func(up *Node, ns []*Node) {
		seen := map[string]int{}
		for _, n := range ns {
			n.Up, n.Inner = up, true
			k := lineKey(n)
			seen[k]++
			n.Key = fmt.Sprintf("%s/%s%d:%s", up.Key, k[:1], seen[k], k[1:])
			inner(n, n.Subs)
		}
	}
	var rec func(up *Node, ns []*Node)
	rec = func(up *Node, ns []*Node) {
		for _, n := range ns {
			n.Up = up
			switch {
			case n.ID != "":
				n.Key = n.ID
			case n.Kind == KindDetails:
				nf++
				n.Key = fmt.Sprintf("fold:%d", nf)
			case n.Kind == KindPlain && n.Num == "":
				plain[n.Text]++
				n.Key = fmt.Sprintf("-%d:%s", plain[n.Text], n.Text)
			case Line(n):
				k := lineKey(n)
				plain[k]++
				n.Key = fmt.Sprintf("%s%d:%s", k[:1], plain[k], k[1:])
			}
			rec(n, n.Kids)
			inner(n, n.Body)
			inner(n, n.Subs)
		}
	}
	rec(nil, nodes)
}

// lineKey is a line of text's kind letter and what it says, for its key.
func lineKey(n *Node) string {
	switch n.Kind {
	case KindTitle:
		return "t" + n.Text
	case KindSection:
		return "h" + n.Text
	case KindCode:
		return "c" + strings.Join(n.Code, "\n")
	case KindPlain:
		if n.Num != "" {
			return "n" + n.Num + " " + n.Text
		}
		return "-" + n.Text
	}
	return "p" + n.Text
}

// HideKey is what a line says, for hiding it: the person hides the line as
// it is now, and when the AI changes it, it is a new line and shows again.
// "" for lines that cannot be hidden.
//
//	plain line      its text (two lines with the same text hide together)
//	Item Ask Link   kind, id, title, state, status (and re, when given)
//	Notice          kind, id, text
//	Fold            kind, and its id or title
func HideKey(n *Node) string {
	switch {
	case n.Kind == KindPlain && Row(n):
		return "-|" + n.Text + subsText(n.Subs)
	case n.Kind == KindPlain:
		return "" // numbered, nested or in a body: not hidden on its own
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

// WalkDeep visits every node, opened or not, and the lines in their
// bodies and nested under list lines too.
func WalkDeep(ns []*Node, f func(*Node)) {
	for _, n := range ns {
		f(n)
		WalkDeep(n.Kids, f)
		WalkDeep(n.Body, f)
		WalkDeep(n.Subs, f)
	}
}

// ByID finds the node with the given target (see Target).
func ByID(ns []*Node, id string) *Node {
	var found *Node
	WalkDeep(ns, func(n *Node) {
		if found == nil && id != "" && Target(n) == id {
			found = n
		}
	})
	return found
}

// foldTarget starts the target of a Fold without an id.
const foldTarget = "Fold|title="

// Target is what reactions and comments on n are recorded against: its id,
// or for a Fold without one, its title (as hiding goes by: see HideKey),
// which can never be an id. A line of text (Line, and a "- " line) is
// recorded by the row it is in and what it says: "Line|", the target of the nearest row around
// it ("" for none), a newline, its text (a code block's first line). ""
// when n takes none.
func Target(n *Node) string {
	switch {
	case n.ID != "":
		return n.ID
	case n.Kind == KindDetails:
		return foldTarget + n.Text
	case Line(n) || n.Kind == KindPlain:
		text := n.Text
		if n.Kind == KindCode {
			text = codeHead(n.Code)
		}
		return lineTarget + Anchor(n) + "\n" + text
	}
	return ""
}

// lineTarget starts the target of a line of text (Line).
const lineTarget = "Line|"

// Anchor is the target of the nearest row around n that is a part (an
// Item, Ask, Notice, Link or Fold; "" for none).
func Anchor(n *Node) string {
	for u := n.Up; u != nil; u = u.Up {
		if Row(u) && u.Kind != KindPlain {
			return Target(u)
		}
	}
	return ""
}

// LineOf is, for the target of a line of text, the target of the row it
// is in ("" for none) and its text; ok false when target is not one.
func LineOf(target string) (anchor, text string, ok bool) {
	rest, ok := strings.CutPrefix(target, lineTarget)
	if !ok {
		return "", "", false
	}
	anchor, text, _ = strings.Cut(rest, "\n")
	return anchor, text, true
}

// FoldTitle is the title in the target of a Fold without an id, and
// whether target is one.
func FoldTitle(target string) (string, bool) {
	return strings.CutPrefix(target, foldTarget)
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

// RowLink is an inline link to a row of the report: [Text](#Target).
type RowLink struct{ Text, Target string }

// reRowLink finds inline links whose target is "#…"; a code span is
// matched first so that a link inside one is not taken.
var reRowLink = regexp.MustCompile("`[^`]+`|\\[([^\\]]+)\\]\\(#([^)]*)\\)")

// RowLinks is the row links in text (inline Markdown), in order.
func RowLinks(text string) []RowLink {
	var out []RowLink
	for _, m := range reRowLink.FindAllStringSubmatch(text, -1) {
		if m[1] != "" || m[2] != "" {
			out = append(out, RowLink{Text: m[1], Target: m[2]})
		}
	}
	return out
}
