package main

import (
	"strings"
	"testing"
)

const sample = `<Title>ita を作る</Title>

inbox を広げた板を作る。いまは報告の書き方を
決めている。

<Section title="あなたへ">
  <Ask id="unit">ページの単位
    <Option key="a">セッションごと</Option>
    <Option key="b">プロジェクトごと</Option>
  </Ask>
  <Item id="hook">hook を入れるか決める
    入れると、書いたことが AI の作業中にも届く。` + "`~/.claude/settings.json`" + ` を触る。
    - 箇条書きの中身
  </Item>
</Section>

<Section title="進行中">
  <Item id="impl" state="doing" status="作業中">最小版を実装する</Item>
  <Item id="parts" status="あなた待ち" note="上の問いの答え">部品の一覧を決める</Item>
</Section>

<Section title="見せたいもの">
  <Link id="spec" path="pitboard/SPEC.md">最小版の仕様</Link>
  <Code lang="diff">
  -old
    +new indented
  </Code>
</Section>

<Fold title="済んだこと (2)">
  <Item id="tui" state="done">viewer を 1 列にする</Item>
  - id の要らない、ただの行
</Fold>
`

func TestParseReport(t *testing.T) {
	nodes, probs, _ := parseReport(sample)
	if len(probs) > 0 {
		t.Fatalf("problems in a good report: %v", probs)
	}
	if len(nodes) != 6 {
		t.Fatalf("top-level parts = %d, want 6 (title, paragraph, 3 sections, fold)", len(nodes))
	}
	if nodes[0].kind != kTitle || nodes[0].text != "ita を作る" {
		t.Errorf("title = %+v", nodes[0])
	}
	if nodes[1].kind != kPara || nodes[1].text != "inbox を広げた板を作る。いまは報告の書き方を決めている。" {
		t.Errorf("paragraph = %q", nodes[1].text)
	}

	ask := byID(nodes, "unit")
	if ask == nil || ask.kind != kQuestion || ask.text != "ページの単位" || len(ask.kids) != 2 {
		t.Fatalf("ask = %+v", ask)
	}
	if c := ask.choice("b"); c == nil || c.text != "プロジェクトごと" {
		t.Errorf("option b = %+v", c)
	}

	hook := byID(nodes, "hook")
	if hook.kind != kTodo || hook.text != "hook を入れるか決める" || len(hook.body) != 2 {
		t.Fatalf("hook = %+v (body %d)", hook, len(hook.body))
	}
	if hook.body[0].kind != kPara || hook.body[1].kind != kPlain || hook.body[1].text != "箇条書きの中身" {
		t.Errorf("hook body = %+v %+v", hook.body[0], hook.body[1])
	}

	impl, parts := byID(nodes, "impl"), byID(nodes, "parts")
	if impl.kind != kDoing || impl.status != "作業中" {
		t.Errorf("impl = %+v", impl)
	}
	if parts.status != "あなた待ち" || parts.reason != "上の問いの答え" {
		t.Errorf("parts = %+v", parts)
	}

	link := byID(nodes, "spec")
	if link.kind != kLink || link.path != "pitboard/SPEC.md" || link.text != "最小版の仕様" {
		t.Errorf("link = %+v", link)
	}
	var code *node
	walkAll(nodes, func(n *node) {
		if n.kind == kCode {
			code = n
		}
	})
	if code == nil || code.lang != "diff" || strings.Join(code.code, "|") != "-old|  +new indented" {
		t.Errorf("code = %+v", code)
	}

	fold := nodes[5]
	if fold.kind != kDetails || fold.text != "済んだこと (2)" || len(fold.kids) != 2 || fold.key == "" {
		t.Fatalf("fold = %+v", fold)
	}
	if fold.kids[0].kind != kDone || fold.kids[0].depth != 1 || fold.kids[1].kind != kPlain {
		t.Errorf("fold contents = %+v %+v", fold.kids[0], fold.kids[1])
	}
	// The contents of a Section stay at its level; a Fold's are indented.
	if hook.depth != 0 || ask.kids[0].depth != 1 {
		t.Errorf("depths: hook %d, option %d", hook.depth, ask.kids[0].depth)
	}
}

func TestParseReportOneLineAndAttributes(t *testing.T) {
	nodes, probs, _ := parseReport(`<Item id="q" status="say &quot;hi&quot;">a <b and **c**</Item>`)
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := byID(nodes, "q")
	if n.text != "a <b and **c**" || n.status != `say "hi"` {
		t.Errorf("item = %+v", n)
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name, src, want string // want: a line number and part of the message
	}{
		{"unknown part", "<Card id=\"a\">x</Card>", "1: unknown part <Card>"},
		{"unknown attribute", `<Item id="a" color="red">x</Item>`, `1: <Item> has no attribute "color"`},
		{"missing id", "<Item>x</Item>", `1: <Item> needs id="..."`},
		{"missing title", "<Section>\n</Section>", `1: <Section> needs title="..."`},
		{"bad id", `<Item id="Hook!">x</Item>`, `1: id "Hook!"`},
		{"duplicate id", "<Item id=\"a\">x</Item>\n<Link id=\"a\" path=\"p\">y</Link>", `2: duplicate id "a" (first on line 1)`},
		{"not closed", "<Section title=\"s\">\n<Item id=\"a\">x", "2: <Item> is not closed"},
		{"stray close", "</Fold>", "1: </Fold> without an open <Fold>"},
		{"section in section", "<Section title=\"a\">\n<Section title=\"b\">\n</Section>\n</Section>", "2: <Section> cannot be inside <Section>"},
		{"item in item", "<Item id=\"a\">x\n<Item id=\"b\">y</Item>\n</Item>", "2: <Item> cannot be inside <Item>"},
		{"one option", "<Ask id=\"a\">q\n<Option key=\"a\">x</Option>\n</Ask>", "1: <Ask> needs 2 or more <Option>"},
		{"option key", "<Ask id=\"a\">q\n<Option key=\"ab\">x</Option>\n<Option key=\"b\">y</Option>\n</Ask>", `2: Option key "ab"`},
		{"same option key", "<Ask id=\"a\">q\n<Option key=\"a\">x</Option>\n<Option key=\"a\">y</Option>\n</Ask>", `3: Option key "a" used twice`},
		{"state", `<Item id="a" state="wip">x</Item>`, `1: state "wip"`},
		{"code not closed", "<Code>\nx", "1: <Code> is not closed"},
		{"link with both", `<Link id="a" path="p" url="https://x">t</Link>`, "1: <Link> needs exactly one of"},
		{"link with neither", `<Link id="a">t</Link>`, "1: <Link> needs exactly one of"},
		{"link url scheme", `<Link id="a" url="javascript:alert(1)">t</Link>`, `1: url "javascript:alert(1)"`},
		{"text in ask", "<Ask id=\"a\">q\nmore\n<Option key=\"a\">x</Option>\n<Option key=\"b\">y</Option>\n</Ask>", "2: <Ask> holds its question"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, probs, _ := parseReport(c.src)
			var got []string
			for _, p := range probs {
				got = append(got, p.String())
			}
			for _, g := range got {
				if strings.HasPrefix(g, c.want) {
					return
				}
			}
			t.Errorf("want %q among %q", c.want, got)
		})
	}
}

func TestCheckKeepsReading(t *testing.T) {
	// An error does not lose the rest of the report.
	nodes, probs, _ := parseReport("<Card>\n<Item id=\"a\">x</Item>\n</Card>\n<Item id=\"b\">y</Item>")
	if len(probs) != 1 {
		t.Errorf("problems = %v", probs)
	}
	if byID(nodes, "a") == nil || byID(nodes, "b") == nil {
		t.Errorf("items after an error were lost")
	}
}

func TestSigChangesWithTheLine(t *testing.T) {
	a, _, _ := parseReport(`<Item id="x" status="作業中">do it</Item>`)
	b, _, _ := parseReport(`<Item id="x" status="あなた待ち">do it</Item>`)
	c, _, _ := parseReport("\n\n" + `<Item id="x" status="作業中">do it</Item>`)
	if byID(a, "x").sig() == byID(b, "x").sig() {
		t.Error("a changed status must change the signature")
	}
	if byID(a, "x").sig() != byID(c, "x").sig() {
		t.Error("moving the line must not change the signature")
	}
}

// Broken input never panics, and what is wrong is reported.
func TestParseReportBrokenInput(t *testing.T) {
	deep := strings.Repeat("<Fold title=\"f\">\n", 5000) + "<Item id=\"x\">deep</Item>\n" + strings.Repeat("</Fold>\n", 5000)
	cases := map[string]struct {
		src      string
		problems bool
	}{
		"code with an unquoted attribute": {"<Code lang=diff>\nx\n</Code>", true},
		"unquoted attribute":              {"<Item id=hook>x</Item>", true},
		"unclosed quote":                  {"<Item id=\"hook>x</Item>", true},
		"unclosed quote, no >":            {"<Item id=\"hook", true},
		"a lone <":                        {"<\n<\n", false},
		"< and a capital only":            {"<A", true},
		"empty attribute":                 {"<Item id=\"\">x</Item>", true},
		"empty file":                      {"", false},
		"only closing tags":               {"</Item>\n</Code>\n</Section>", true},
		"deep nesting":                    {deep, false},
		"deep nesting, never closed":      {strings.Repeat("<Fold title=\"f\">\n", 5000), true},
		"code inside an unknown part":     {"<Card>\n<Code>\nx\n</Card>", true},
		"control characters":              {"<Item id=\"a\">a\x1b[31mred\x00</Item>", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, probs, err := parseReport(c.src)
			if err != nil {
				t.Fatalf("parser failed: %v", err)
			}
			if c.problems != (len(probs) > 0) {
				t.Errorf("problems = %v", probs)
			}
		})
	}
}

func FuzzParseReport(f *testing.F) {
	f.Add(sample)
	f.Add("<Code lang=diff>\nx\n</Code>")
	f.Add("<Ask id=\"a\">q\n<Option key=\"a\">x</Option>\n</Ask>")
	f.Fuzz(func(t *testing.T, src string) {
		if _, _, err := parseReport(src); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLowercaseAndViewerKeys(t *testing.T) {
	nodes, probs, _ := parseReport("<section title=\"s\">\n<item id=\"a\">x</item>\n</Section>\n<Ask id=\"q\">?\n<Option key=\"y\">yes</Option>\n<Option key=\"a\">ok</Option>\n</Ask>")
	var got []string
	for _, p := range probs {
		got = append(got, p.String())
	}
	for _, want := range []string{"1: part names start with a capital letter", "2: part names start with a capital letter", `5: Option key "y" is a viewer key`} {
		found := false
		for _, g := range got {
			found = found || strings.HasPrefix(g, want)
		}
		if !found {
			t.Errorf("want %q among %q", want, got)
		}
	}
	if byID(nodes, "a") == nil {
		t.Error("a lowercase part is still read")
	}
}

func TestFoldKeys(t *testing.T) {
	a, _, _ := parseReport("<Fold title=\"Done (2)\">\n</Fold>\n<Fold id=\"old\" title=\"Old (1)\">\n</Fold>")
	b, _, _ := parseReport("<Fold title=\"Done (3)\">\n</Fold>\n<Fold title=\"New\">\n</Fold>\n<Fold id=\"old\" title=\"Old (2)\">\n</Fold>")
	if a[0].key != b[0].key {
		t.Errorf("retitling a fold changed its key: %q -> %q", a[0].key, b[0].key)
	}
	if a[1].key != "old" || b[2].key != "old" {
		t.Errorf("a fold with an id is tracked by it: %q %q", a[1].key, b[2].key)
	}
}

func TestPrintable(t *testing.T) {
	s := plainText("a\nb\r\tc\x1b[2Jd\x00", "").String()
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("control character %q reaches the terminal: %q", r, s)
		}
	}
}

func TestLinkURL(t *testing.T) {
	nodes, probs, _ := parseReport(`<Link id="pr" url="https://example.com/a?b=1">PR</Link>`)
	if len(probs) > 0 || byID(nodes, "pr").url != "https://example.com/a?b=1" {
		t.Fatalf("link: %v %+v", probs, byID(nodes, "pr"))
	}
	s := plainText("ab", "").hyperlink("https://x.test/\x1b]8;;evil\x07").String()
	if !strings.HasPrefix(s, "\x1b]8;;https://x.test/]8;;evil\x1b\\ab") || !strings.HasSuffix(s, "\x1b]8;;\x1b\\") {
		t.Errorf("hyperlink = %q", s)
	}
}
