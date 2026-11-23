package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The report is written with named parts:
//
//	<Section title="...">
//	  <Item id="hook" state="doing" status="作業中">title
//	    contents
//	  </Item>
//	</Section>
//
// A line whose text starts with "<" and a capital letter opens a part and
// "</Name>" closes it; every other line is text (inline Markdown). Anything
// unknown or misplaced is a Problem: `pitboard check` prints them, and the
// viewer shows them above what it could read.

// Problem is one thing wrong with the report, at a 1-based line.
type Problem struct {
	Line int
	Msg  string
	// lost: something written could not be shown as written (a line
	// dropped, a part not read as a part).
	lost bool
}

func (p Problem) String() string { return fmt.Sprintf("%d: %s", p.Line, p.Msg) }

type partSpec struct {
	attrs    map[string]bool // attribute -> required
	parents  []string        // where it may appear ("" is the top level)
	oneLine  bool            // its content is a single line of text
	holdsAny bool            // holds parts and text (Section, Fold)
	para     bool            // its content is one paragraph of text (Notice)
}

var parts = map[string]partSpec{
	"Title":   {attrs: map[string]bool{}, parents: []string{""}, oneLine: true},
	"Section": {attrs: map[string]bool{"title": true}, parents: []string{""}, holdsAny: true},
	"Item":    {attrs: map[string]bool{"id": true, "state": false, "status": false, "note": false, "ask": false, "wait": false, "look": false}, parents: []string{"", "Section", "Fold"}},
	"Ask":     {attrs: map[string]bool{"id": true, "look": false}, parents: []string{"", "Section", "Fold"}},
	"Option":  {attrs: map[string]bool{"key": true}, parents: []string{"Ask"}, oneLine: true},
	"Link":    {attrs: map[string]bool{"id": true, "path": false, "url": false, "ask": false, "look": false}, parents: []string{"", "Section", "Fold"}, oneLine: true},
	"Fold":    {attrs: map[string]bool{"title": true, "id": false, "look": false}, parents: []string{"", "Section", "Fold"}, holdsAny: true},
	"Notice":  {attrs: map[string]bool{"id": true}, parents: []string{""}, para: true},
	"Code":    {attrs: map[string]bool{"lang": false}, parents: []string{"", "Section", "Fold", "Item"}},
}

var (
	reOpen     = regexp.MustCompile(`^<([A-Z][A-Za-z0-9]*)((?:\s+[A-Za-z_][A-Za-z0-9_-]*="[^"]*")*)\s*>(.*)$`)
	reOpenAny  = regexp.MustCompile(`^<([A-Z][A-Za-z0-9]*)`)
	reAttr     = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*)="([^"]*)"`)
	reClose    = regexp.MustCompile(`^</([A-Za-z][A-Za-z0-9]*)>\s*(.*)$`)
	reTail     = regexp.MustCompile(`^(.*?)</([A-Za-z][A-Za-z0-9]*)>\s*$`)
	reIDForm   = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)
	reKeyForm  = regexp.MustCompile(`^[a-z]$`)
	reTagName  = regexp.MustCompile(`^</?([A-Za-z][A-Za-z0-9]*)`)
	reNumbered = regexp.MustCompile(`^([0-9]{1,3}\.)\s+(.*)$`)
	reSubHead  = regexp.MustCompile(`^\*\*[^*]+\*\*$`)
	reHeadWord = regexp.MustCompile(`^\*\*[^*]+\*\*[:：]`)
)

// maxNotices is how many Notice parts a report may have.
const maxNotices = 3

// maxListLevel is how deep lists nest (levels 0, 1, 2).
const maxListLevel = 2

// viewerKeys are the keys the viewer uses for itself; an Option cannot use
// them as its key.
const viewerKeys = "cjkmnqrsuxy"

// viewerKeyList is viewerKeys spaced out for messages ("c j k ...").
func viewerKeyList() string { return strings.Join(strings.Split(viewerKeys, ""), " ") }

type frame struct {
	name   string // "" for the top level
	n      *node  // nil for an unknown part (its contents go to the parent)
	line   int
	titled bool  // Item / Ask: the first text line (the title) has been read
	para   *node // paragraph being read
	indent int   // Code: indentation of the opening tag, removed from its lines
	keys   map[string]bool
	// The text in the part: base is the indentation of its first text line
	// (-1 until there is one), list the list lines open at each level, brk
	// that the last line ended with a line break, gap that a blank line came
	// last.
	base int
	list []*node
	cols []int // where each open list line's mark was written (rel)
	brk  bool
	gap  bool
}

type reportParser struct {
	// indent is the indentation of the line being read (-1: text on the
	// line of a tag), brk whether it ends with a line break (two spaces or
	// a backslash).
	indent  int
	brk     bool
	root    frame
	stack   []*frame
	probs   []Problem
	ids     map[string]int
	notices int // how many Notice parts so far
	nodes   []*node
}

func (p *reportParser) errf(line int, format string, args ...any) {
	p.probs = append(p.probs, Problem{Line: line, Msg: fmt.Sprintf(format, args...)})
}

// errLost is errf for a problem that leaves something out of the screen.
func (p *reportParser) errLost(line int, format string, args ...any) {
	p.probs = append(p.probs, Problem{Line: line, Msg: fmt.Sprintf(format, args...), lost: true})
}

// partial says whether some of the report could not be shown as written.
func partial(probs []Problem) bool {
	for _, p := range probs {
		if p.lost {
			return true
		}
	}
	return false
}

// top is the innermost known part (unknown parts are transparent).
func (p *reportParser) top() *frame {
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i].n != nil {
			return p.stack[i]
		}
	}
	return &p.root
}

// add puts n into the part f holds it in.
func (p *reportParser) add(f *frame, n *node) {
	switch {
	case f.n == nil:
		p.nodes = append(p.nodes, n)
	case f.name == "Item":
		f.n.body = append(f.n.body, n)
	default:
		f.n.kids = append(f.n.kids, n)
	}
}

// parseReport reads the report. What cannot be read is reported as a
// Problem and the rest is still returned. err is set only if the parser
// itself failed (a bug); it never panics.
func parseReport(src string) (nodes []*node, probs []Problem, err error) {
	defer func() {
		if r := recover(); r != nil {
			nodes, probs, err = nil, nil, fmt.Errorf("could not read the report (pitboard bug): %v", r)
		}
	}()
	nodes, probs = parseReportUnsafe(src)
	return nodes, probs, nil
}

func parseReportUnsafe(src string) ([]*node, []Problem) {
	p := &reportParser{ids: map[string]int{}, root: frame{base: -1}}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		p.line(i+1, raw)
	}
	for len(p.stack) > 0 {
		f := p.stack[len(p.stack)-1]
		p.errLost(f.line, "<%s> is not closed", f.name)
		p.pop()
	}
	assignKeys(p.nodes)
	setDepth(p.nodes, 0)
	sort.SliceStable(p.probs, func(i, j int) bool { return p.probs[i].Line < p.probs[j].Line })
	return p.nodes, p.probs
}

func (p *reportParser) line(no int, raw string) {
	trim := strings.TrimSpace(raw)

	// Inside Code everything up to </Code> is kept as it is.
	if f := p.innermost(); f != nil && f.name == "Code" && f.n != nil {
		if trim == "</Code>" {
			p.pop()
			return
		}
		f.n.code = append(f.n.code, dedent(raw, f.indent))
		return
	}

	// A part name in the wrong case ("<item") is an error, read as the part.
	if m := reTagName.FindStringSubmatchIndex(trim); m != nil {
		name := trim[m[2]:m[3]]
		if canon := canonicalPart(name); canon != "" && canon != name {
			p.errf(no, "part names start with a capital letter: <%s>, not <%s>", canon, name)
			trim = trim[:m[2]] + canon + trim[m[3]:]
			raw = strings.Repeat(" ", len(raw)-len(strings.TrimLeft(raw, " \t"))) + trim
		}
	}

	p.indent = len(raw) - len(strings.TrimLeft(raw, " \t"))
	p.brk = strings.HasSuffix(raw, "  ") || strings.HasSuffix(trim, "\\")
	switch {
	case trim == "":
		f := p.top()
		f.para, f.list, f.cols, f.brk = nil, nil, nil, false
		f.gap = true
		return
	case strings.HasPrefix(trim, "</"):
		m := reClose.FindStringSubmatch(trim)
		if m == nil {
			p.errLost(no, "malformed closing tag")
			return
		}
		p.close(no, m[1])
		if strings.TrimSpace(m[2]) != "" {
			p.errLost(no, "text after </%s> on the same line", m[1])
		}
		return
	case reOpenAny.MatchString(trim):
		p.open(no, raw, trim)
		return
	}
	// A text line may end the part it is in: "title</Item>".
	if m := reTail.FindStringSubmatch(trim); m != nil {
		p.text(no, m[1])
		p.close(no, p.partName(no, m[2]))
		return
	}
	p.text(no, trim)
}

func (p *reportParser) open(no int, raw, trim string) {
	p.top().para, p.top().list, p.top().cols = nil, nil, nil
	m := reOpen.FindStringSubmatch(trim)
	if m == nil {
		name := reOpenAny.FindStringSubmatch(trim)[1]
		gt := strings.IndexByte(trim, '>')
		if gt < 0 {
			p.errLost(no, "<%s tag is not closed with >", name)
			return
		}
		p.errLost(no, "malformed <%s> tag (attributes are name=\"value\")", name)
		// Keep reading as if it were an unknown part, so its closing tag
		// does not close something else; what it holds is still shown.
		p.stack = append(p.stack, &frame{name: name, line: no})
		p.rest(no, name, trim[gt+1:])
		return
	}
	name, attrText, rest := m[1], m[2], m[3]
	spec, known := parts[name]
	if !known {
		p.errLost(no, "unknown part <%s> (parts: Title Section Item Ask Option Link Fold Code)", name)
		p.stack = append(p.stack, &frame{name: name, line: no})
		p.rest(no, name, rest)
		return
	}

	attrs := map[string]string{}
	for _, a := range reAttr.FindAllStringSubmatch(attrText, -1) {
		if _, ok := spec.attrs[a[1]]; !ok {
			p.errf(no, "<%s> has no attribute %q", name, a[1])
			continue
		}
		if _, dup := attrs[a[1]]; dup {
			p.errf(no, "attribute %q given twice", a[1])
		}
		attrs[a[1]] = strings.NewReplacer("&quot;", `"`, "&amp;", "&").Replace(a[2])
	}
	for a, required := range spec.attrs {
		if required && attrs[a] == "" {
			p.errf(no, "<%s> needs %s=\"...\"", name, a)
		}
	}

	n := &node{line: no}
	switch name {
	case "Title":
		n.kind = kTitle
	case "Section":
		n.kind, n.text = kSection, attrs["title"]
	case "Fold":
		n.kind, n.text = kDetails, attrs["title"]
	case "Item":
		switch attrs["state"] {
		case "", "todo":
			n.kind = kTodo
		case "doing":
			n.kind = kDoing
		case "done":
			n.kind = kDone
		default:
			n.kind = kTodo
			p.errf(no, "state %q: use todo, doing or done", attrs["state"])
		}
		n.status, n.reason = attrs["status"], attrs["note"]
	case "Ask":
		n.kind = kQuestion
	case "Notice":
		n.kind = kNotice
		if p.notices++; p.notices > maxNotices {
			p.errf(no, "at most %d <Notice> parts: say only what the person must know first", maxNotices)
		}
	case "Option":
		n.kind, n.label = kChoice, attrs["key"]
	case "Link":
		n.kind, n.path, n.url = kLink, attrs["path"], attrs["url"]
		switch {
		case (n.path == "") == (n.url == ""):
			p.errf(no, "<Link> needs exactly one of path=\"...\" or url=\"...\"")
		case n.url != "" && !strings.HasPrefix(n.url, "http://") && !strings.HasPrefix(n.url, "https://"):
			p.errf(no, "url %q: use an http:// or https:// URL", n.url)
		}
	case "Code":
		n.kind, n.lang = kCode, attrs["lang"]
	}
	if v, ok := attrs["wait"]; ok {
		switch {
		case v != "you":
			p.errf(no, "wait %q: the only value is \"you\" (it needs the person to move on)", v)
		case n.kind == kDone:
			p.errf(no, "a done <Item> waits for nobody: drop wait=\"you\"")
		default:
			n.wait = v
		}
	}
	if v, ok := attrs["ask"]; ok {
		switch {
		case v != "y":
			p.errf(no, "ask %q: the only value is \"y\" (asking for y / n)", v)
		case n.kind == kDone:
			p.errf(no, "a done <Item> asks nothing: drop ask=\"y\"")
		default:
			n.ask = v
		}
	}
	if v, ok := attrs["look"]; ok {
		// On a line that asks (Ask, ask="y"), the yellow mark wins; not an
		// error.
		if v != "y" {
			p.errf(no, "look %q: the only value is \"y\" (asking the person to read the line)", v)
		} else {
			n.look = v
		}
	}
	if id := attrs["id"]; id != "" {
		switch {
		case !reIDForm.MatchString(id):
			p.errf(no, "id %q: use 1-16 of a-z 0-9 -", id)
		case p.ids[id] != 0:
			p.errf(no, "duplicate id %q (first on line %d)", id, p.ids[id])
		default:
			p.ids[id] = no
			n.id = id
		}
	}

	parent := p.top()
	if !contains(spec.parents, parent.name) {
		where := "at the top level"
		if parent.name != "" {
			where = "inside <" + parent.name + ">"
		}
		p.errf(no, "<%s> cannot be %s", name, where)
		// Put it in the nearest part that may hold it, or at the top.
		parent = &p.root
		for i := len(p.stack) - 1; i >= 0; i-- {
			if f := p.stack[i]; f.n != nil && contains(spec.parents, f.name) {
				parent = f
				break
			}
		}
	}
	if name == "Option" && parent.name == "Ask" {
		if parent.keys == nil {
			parent.keys = map[string]bool{}
		}
		switch k := n.label; {
		case k == "":
		case !reKeyForm.MatchString(k):
			p.errf(no, "Option key %q: use one letter a-z", k)
		case strings.Contains(viewerKeys, k):
			p.errf(no, "Option key %q is a viewer key; do not use any of %s", k, viewerKeyList())
		case parent.keys[k]:
			p.errf(no, "Option key %q used twice in this Ask", k)
		default:
			parent.keys[k] = true
		}
	}
	p.add(parent, n)

	indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
	p.stack = append(p.stack, &frame{name: name, n: n, line: no, indent: indent, base: -1})
	if name == "Code" {
		if r := strings.TrimSpace(rest); r != "" {
			if r == "</Code>" {
				p.pop()
			} else {
				p.errLost(no, "put the code on the lines after <Code>")
			}
		}
		return
	}
	p.rest(no, name, rest)
}

// rest is what follows an opening tag on its line: the first line of text,
// possibly with the closing tag.
func (p *reportParser) rest(no int, name, rest string) {
	p.indent = -1
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return
	}
	if m := reTail.FindStringSubmatch(rest); m != nil {
		p.text(no, m[1])
		p.close(no, p.partName(no, m[2]))
		return
	}
	p.text(no, rest)
}

// partName is a part name as written in a closing tag after text
// ("x</item>"): spelled in the wrong case, it is an error and read as the
// part, just as at the start of a line.
func (p *reportParser) partName(no int, name string) string {
	if canon := canonicalPart(name); canon != "" && canon != name {
		p.errf(no, "part names start with a capital letter: </%s>, not </%s>", canon, name)
		return canon
	}
	return name
}

func (p *reportParser) close(no int, name string) {
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i].name != name {
			continue
		}
		for len(p.stack)-1 > i {
			f := p.stack[len(p.stack)-1]
			p.errLost(f.line, "<%s> is not closed (</%s> on line %d closes the part around it)", f.name, name, no)
			p.pop()
		}
		p.pop()
		return
	}
	p.errf(no, "</%s> without an open <%s>", name, name)
}

// pop ends the innermost part and checks what it must hold.
func (p *reportParser) pop() {
	f := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	n := f.n
	if n == nil {
		return
	}
	switch f.name {
	case "Title", "Item", "Ask", "Option", "Link":
		if strings.TrimSpace(n.text) == "" {
			p.errf(f.line, "<%s> has no text", f.name)
		}
	}
	if f.name == "Ask" {
		c := 0
		for _, k := range n.kids {
			if k.kind == kChoice {
				c++
			}
		}
		if c < 2 {
			p.errf(f.line, "<Ask> needs 2 or more <Option>")
		}
	}
}

// text is one line of text inside the innermost part.
func (p *reportParser) text(no int, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	f := p.top()
	spec := parts[f.name]
	switch {
	case spec.para:
		f.n.text = joinLines(f.n.text, s)
		return
	case spec.oneLine:
		if f.n.text != "" {
			p.errf(no, "<%s> takes one line of text", f.name)
		}
		f.n.text = joinLines(f.n.text, s)
		return
	case (f.name == "Item" || f.name == "Ask") && !f.titled:
		f.n.text, f.titled = s, true
		return
	case f.name == "Ask":
		p.errLost(no, "<Ask> holds its question and <Option> parts only")
		return
	}
	p.block(no, f, s)
}

// block is a line of text in a part that holds text (not a title): a
// paragraph line, a list line, or the continuation of one.
func (p *reportParser) block(no int, f *frame, s string) {
	brk := p.brk
	if brk && strings.HasSuffix(s, "\\") {
		s = strings.TrimSpace(strings.TrimSuffix(s, "\\"))
	}
	// Indentation counts from the part's first text line.
	rel := 0
	if p.indent >= 0 {
		if f.base < 0 {
			f.base = p.indent
		}
		rel = max(p.indent-f.base, 0)
	}
	gap := f.gap
	f.gap = false
	join := func(n *node) {
		// After a line break, or after a list line that is only a bold
		// head word ("- **XDG**"), the next line starts a line of its own.
		if f.brk || (n.kind == kPlain && reSubHead.MatchString(n.text)) {
			n.text += "\n" + s
		} else {
			n.text = joinLines(n.text, s)
		}
		f.brk = brk
	}

	num, body, isList := "", "", false
	switch {
	case strings.HasPrefix(s, "- "):
		body, isList = strings.TrimSpace(s[2:]), true
	case s == "-":
		isList = true
	default:
		if m := reNumbered.FindStringSubmatch(s); m != nil {
			num, body, isList = m[1], strings.TrimSpace(m[2]), true
		}
	}
	if isList {
		// The level: a sibling of an open list line written at the same
		// place; a child when indented to where the text of the line above
		// starts (under "1. " that is 3, as Markdown writes it); else 2
		// spaces per level.
		level, placed := 0, false
		for j, c := range f.cols {
			if c == rel {
				level, placed = j, true
			}
		}
		if k := len(f.list); !placed && k > 0 && f.list[k-1].num != "" && rel == f.cols[k-1]+len(f.list[k-1].num)+1 {
			level, placed = k, true
		}
		if !placed {
			if rel%2 != 0 {
				p.errf(no, "list lines are indented by 2 spaces per level (this one by %d)", rel)
			}
			level = rel / 2
		}
		if level > maxListLevel {
			p.errf(no, "lists nest %d levels at most", maxListLevel+1)
			level = maxListLevel
		}
		level = min(level, len(f.list)) // no skipping a level
		n := &node{kind: kPlain, text: body, num: num, level: level, line: no, gap: gap}
		if level == 0 {
			p.add(f, n)
		} else {
			parent := f.list[level-1]
			parent.subs = append(parent.subs, n)
		}
		f.list = append(f.list[:level], n)
		f.cols = append(f.cols[:level], rel)
		f.para, f.brk = nil, brk
		return
	}
	// Under a list line, a line indented past its mark continues it.
	if k := len(f.list); k > 0 && rel > f.cols[k-1] {
		join(f.list[k-1])
		return
	}
	f.list, f.cols = nil, nil
	if reSubHead.MatchString(s) {
		p.add(f, &node{kind: kPara, text: s, head: true, line: no, gap: gap})
		f.para, f.brk = nil, false
		return
	}
	// A paragraph goes on, unless the line starts with a head word
	// ("**word**: ..."), which starts a paragraph of its own.
	if f.para != nil && !reHeadWord.MatchString(s) {
		join(f.para)
		return
	}
	f.para = &node{kind: kPara, text: s, line: no, gap: gap}
	f.brk = brk
	p.add(f, f.para)
}

func (p *reportParser) innermost() *frame {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1]
}

// canonicalPart is the part name spelled as it should be, or "".
func canonicalPart(name string) string {
	for k := range parts {
		if strings.EqualFold(k, name) {
			return k
		}
	}
	return ""
}

// dedent removes up to n leading spaces or tabs.
func dedent(s string, n int) string {
	i := 0
	for i < len(s) && i < n && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
