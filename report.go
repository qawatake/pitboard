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
}

func (p Problem) String() string { return fmt.Sprintf("%d: %s", p.Line, p.Msg) }

type partSpec struct {
	attrs    map[string]bool // attribute -> required
	parents  []string        // where it may appear ("" is the top level)
	oneLine  bool            // its content is a single line of text
	holdsAny bool            // holds parts and text (Section, Fold)
}

var parts = map[string]partSpec{
	"Title":   {attrs: map[string]bool{}, parents: []string{""}, oneLine: true},
	"Section": {attrs: map[string]bool{"title": true}, parents: []string{""}, holdsAny: true},
	"Item":    {attrs: map[string]bool{"id": true, "state": false, "status": false, "note": false, "ask": false}, parents: []string{"", "Section", "Fold"}},
	"Ask":     {attrs: map[string]bool{"id": true}, parents: []string{"", "Section", "Fold"}},
	"Option":  {attrs: map[string]bool{"key": true}, parents: []string{"Ask"}, oneLine: true},
	"Link":    {attrs: map[string]bool{"id": true, "path": false, "url": false, "ask": false}, parents: []string{"", "Section", "Fold"}, oneLine: true},
	"Fold":    {attrs: map[string]bool{"title": true, "id": false}, parents: []string{"", "Section", "Fold"}, holdsAny: true},
	"Code":    {attrs: map[string]bool{"lang": false}, parents: []string{"", "Section", "Fold", "Item"}},
}

var (
	reOpen    = regexp.MustCompile(`^<([A-Z][A-Za-z0-9]*)((?:\s+[A-Za-z_][A-Za-z0-9_-]*="[^"]*")*)\s*>(.*)$`)
	reOpenAny = regexp.MustCompile(`^<([A-Z][A-Za-z0-9]*)`)
	reAttr    = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*)="([^"]*)"`)
	reClose   = regexp.MustCompile(`^</([A-Za-z][A-Za-z0-9]*)>\s*(.*)$`)
	reTail    = regexp.MustCompile(`^(.*?)</([A-Za-z][A-Za-z0-9]*)>\s*$`)
	reIDForm  = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)
	reKeyForm = regexp.MustCompile(`^[a-z]$`)
	reTagName = regexp.MustCompile(`^</?([A-Za-z][A-Za-z0-9]*)`)
)

// viewerKeys are the keys the viewer uses for itself; an Option cannot use
// them as its key.
const viewerKeys = "cjkmnqsxy"

type frame struct {
	name   string // "" for the top level
	n      *node  // nil for an unknown part (its contents go to the parent)
	line   int
	titled bool  // Item / Ask: the first text line (the title) has been read
	para   *node // paragraph being read
	indent int   // Code: indentation of the opening tag, removed from its lines
	keys   map[string]bool
}

type reportParser struct {
	root  frame
	stack []*frame
	probs []Problem
	ids   map[string]int
	nodes []*node
}

func (p *reportParser) errf(line int, format string, args ...any) {
	p.probs = append(p.probs, Problem{line, fmt.Sprintf(format, args...)})
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
	p := &reportParser{ids: map[string]int{}}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		p.line(i+1, raw)
	}
	for len(p.stack) > 0 {
		f := p.stack[len(p.stack)-1]
		p.errf(f.line, "<%s> is not closed", f.name)
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

	switch {
	case trim == "":
		p.top().para = nil
		return
	case strings.HasPrefix(trim, "</"):
		m := reClose.FindStringSubmatch(trim)
		if m == nil {
			p.errf(no, "malformed closing tag")
			return
		}
		p.close(no, m[1])
		if strings.TrimSpace(m[2]) != "" {
			p.errf(no, "text after </%s> on the same line", m[1])
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
	p.top().para = nil
	m := reOpen.FindStringSubmatch(trim)
	if m == nil {
		name := reOpenAny.FindStringSubmatch(trim)[1]
		gt := strings.IndexByte(trim, '>')
		if gt < 0 {
			p.errf(no, "<%s tag is not closed with >", name)
			return
		}
		p.errf(no, "malformed <%s> tag (attributes are name=\"value\")", name)
		// Keep reading as if it were an unknown part, so its closing tag
		// does not close something else; what it holds is still shown.
		p.stack = append(p.stack, &frame{name: name, line: no})
		p.rest(no, name, trim[gt+1:])
		return
	}
	name, attrText, rest := m[1], m[2], m[3]
	spec, known := parts[name]
	if !known {
		p.errf(no, "unknown part <%s> (parts: Title Section Item Ask Option Link Fold Code)", name)
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
			p.errf(no, "Option key %q is a viewer key; do not use any of %s", k, strings.Join(strings.Split(viewerKeys, ""), " "))
		case parent.keys[k]:
			p.errf(no, "Option key %q used twice in this Ask", k)
		default:
			parent.keys[k] = true
		}
	}
	p.add(parent, n)

	indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
	p.stack = append(p.stack, &frame{name: name, n: n, line: no, indent: indent})
	if name == "Code" {
		if r := strings.TrimSpace(rest); r != "" {
			if r == "</Code>" {
				p.pop()
			} else {
				p.errf(no, "put the code on the lines after <Code>")
			}
		}
		return
	}
	p.rest(no, name, rest)
}

// rest is what follows an opening tag on its line: the first line of text,
// possibly with the closing tag.
func (p *reportParser) rest(no int, name, rest string) {
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
			p.errf(f.line, "<%s> is not closed (</%s> on line %d closes the part around it)", f.name, name, no)
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
		p.errf(no, "<Ask> holds its question and <Option> parts only")
		return
	}
	if strings.HasPrefix(s, "- ") {
		p.add(f, &node{kind: kPlain, text: strings.TrimSpace(s[2:]), line: no})
		f.para = nil
		return
	}
	if f.para != nil {
		f.para.text = joinLines(f.para.text, s)
		return
	}
	f.para = &node{kind: kPara, text: s, line: no}
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
