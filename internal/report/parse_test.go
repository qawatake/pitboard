package report

import (
	"strings"
	"testing"
)

func TestWaitAttribute(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<Item id="a" wait="you">x</Item>`, ""},
		{`<Item id="a" wait="me">x</Item>`, `wait "me"`},
		{`<Item id="a" state="done" wait="you">x</Item>`, "waits for nobody"},
		{`<Link id="a" path="p" wait="you">x</Link>`, "no attribute"},
	} {
		_, probs, _ := Parse(c.src)
		got := ""
		for _, p := range probs {
			got += p.Msg
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q (want %q)", c.src, got, c.want)
		}
	}
}

const noticeReport = `<Notice id="restart">pitboard を開き直してください。新しい版を入れました</Notice>
<Title>題</Title>

<Section title="s">
<Item id="a" ask="y">項目 a</Item>
</Section>
`

func TestNoticeParse(t *testing.T) {
	nodes, probs, _ := Parse(noticeReport)
	if len(probs) != 0 || nodes[0].Kind != KindNotice || nodes[0].Text != "pitboard を開き直してください。新しい版を入れました" {
		t.Fatalf("%v %+v", probs, nodes[0])
	}
	cases := []struct{ src, want string }{
		{"<Notice>x</Notice>", "needs id"},
		{"<Section title=\"s\">\n<Notice id=\"n\">x</Notice>\n</Section>", "cannot be inside <Section>"},
		{"<Fold title=\"f\">\n<Notice id=\"n\">x</Notice>\n</Fold>", "cannot be inside <Fold>"},
		{"<Notice id=\"a\">a</Notice>\n<Notice id=\"b\">b</Notice>\n<Notice id=\"c\">c</Notice>\n<Notice id=\"d\">d</Notice>", "at most 3"},
	}
	for _, c := range cases {
		_, probs, _ := Parse(c.src)
		if len(probs) != 1 || !strings.Contains(probs[0].Msg, c.want) {
			t.Errorf("%q: %v", c.src, probs)
		}
	}
	// Like an Item: the first line is the title, the rest (text and Code)
	// what opening it shows.
	nodes, probs, _ = Parse("<Notice id=\"n\">開き直してください\n  手順は次のとおり\n  <Code>\n  pitboard\n  </Code>\n</Notice>")
	if len(probs) != 0 || nodes[0].Text != "開き直してください" || len(nodes[0].Body) != 2 ||
		nodes[0].Body[0].Text != "手順は次のとおり" || nodes[0].Body[1].Kind != KindCode {
		t.Errorf("%v %+v", probs, nodes[0])
	}
}

func TestAskAttributeCheck(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<Item id="a" ask="y">x</Item>`, ""},
		{`<Link id="a" ask="y" path="p">x</Link>`, ""},
		{`<Item id="a" ask="n">x</Item>`, `ask "n"`},
		{`<Item id="a" state="done" ask="y">x</Item>`, "done"},
		{`<Item id="a" look="y">x</Item>`, ""},
		{`<Item id="a" ask="y" look="y">x</Item>`, ""},
		{`<Fold title="f" look="y">x</Fold>`, ""},
		{"<Ask id=\"a\" look=\"y\">x\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>", ""},
		{`<Item id="a" look="n">x</Item>`, `look "n"`},
		{"<Ask id=\"a\" ask=\"y\">x\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>", "no attribute"},
	} {
		_, probs, _ := Parse(c.src)
		got := ""
		for _, p := range probs {
			got += p.Msg
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q (want %q)", c.src, got, c.want)
		}
	}
}

var richItem = strings.Replace(`<Item id="where">データをどこに置くか
  おすすめは折衷。

  - **cwd に置く**
    - 良い点: 書ける
      続きの行
    - 悪い点: 見つからない
  - **折衷**: データは cwd

  **手順**
  1. まず cwd
  2. 次に XDG
    - 入れ子
  行末で改行する<BR>
  次の行\
  その次の行
  **語**: 見出し語の段落
</Item>`, "<BR>", "  ", 1)

func TestRichText(t *testing.T) {
	nodes, probs, _ := Parse(richItem)
	if len(probs) > 0 {
		t.Fatalf("problems: %v", probs)
	}
	body := nodes[0].Body
	var got []string
	var dump func(ns []*Node, d int)
	dump = func(ns []*Node, d int) {
		for _, n := range ns {
			s := strings.Repeat(" ", d)
			switch {
			case n.Kind == KindPlain:
				s += "L" + n.Num + ":" + n.Text
			case n.Head:
				s += "H:" + n.Text
			default:
				s += "P:" + n.Text
			}
			if n.Gap {
				s += " (gap)"
			}
			got = append(got, s)
			dump(n.Subs, d+1)
		}
	}
	dump(body, 0)
	want := []string{
		"P:おすすめは折衷。",
		"L:**cwd に置く** (gap)",
		" L:良い点: 書ける続きの行",
		" L:悪い点: 見つからない",
		"L:**折衷**: データは cwd",
		"H:**手順** (gap)",
		"L1.:まず cwd",
		"L2.:次に XDG",
		" L:入れ子",
		"P:行末で改行する\n次の行\nその次の行",
		"P:**語**: 見出し語の段落",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestListErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"<Section title=\"s\">\n- a\n   - b\n</Section>", "3: list lines are indented by 2"},
		{"<Section title=\"s\">\n- a\n  - b\n    - c\n      - d\n</Section>", "5: lists nest 3 levels at most"},
		{"<Section title=\"s\">\n- a\n  - b\n    - c\n</Section>", ""},
	} {
		_, probs, _ := Parse(c.src)
		got := ""
		for _, p := range probs {
			got += p.String()
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%q: %q (want %q)", c.src, got, c.want)
		}
	}
}

func TestViewerKeysForOptions(t *testing.T) {
	_, probs, _ := Parse("<Ask id=\"q\">q\n<Option key=\"x\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>")
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
		t.Errorf("x as an Option key: %v", probs)
	}
}

func TestLowercaseClosingTagOnTheSameLine(t *testing.T) {
	nodes, probs, _ := Parse(`<Section title="あなたへ">
<Item id="hook">hook を入れるか決める</item>
<Item id="impl">最小版を実装する</Item>
</Section>
<fold title="f"></fold>`)
	var got []string
	for _, p := range probs {
		got = append(got, p.String())
	}
	want := []string{
		"2: part names start with a capital letter: </Item>, not </item>",
		"5: part names start with a capital letter: <Fold>, not <fold>",
		"5: part names start with a capital letter: </Fold>, not </fold>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	sec := nodes[0]
	if len(sec.Kids) != 2 || sec.Kids[1].ID != "impl" || len(sec.Kids[0].Body) != 0 {
		t.Errorf("hook must close on its line; section holds %d parts", len(sec.Kids))
	}
}

func TestRIsAViewerKey(t *testing.T) {
	_, probs, _ := Parse("<Ask id=\"q\">q\n<Option key=\"r\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>")
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
		t.Errorf("r as an Option key: %v", probs)
	}
}

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
	nodes, probs, _ := Parse(sample)
	if len(probs) > 0 {
		t.Fatalf("problems in a good report: %v", probs)
	}
	if len(nodes) != 6 {
		t.Fatalf("top-level parts = %d, want 6 (title, paragraph, 3 sections, fold)", len(nodes))
	}
	if nodes[0].Kind != KindTitle || nodes[0].Text != "ita を作る" {
		t.Errorf("title = %+v", nodes[0])
	}
	if nodes[1].Kind != KindPara || nodes[1].Text != "inbox を広げた板を作る。いまは報告の書き方を決めている。" {
		t.Errorf("paragraph = %q", nodes[1].Text)
	}

	ask := ByID(nodes, "unit")
	if ask == nil || ask.Kind != KindQuestion || ask.Text != "ページの単位" || len(ask.Kids) != 2 {
		t.Fatalf("ask = %+v", ask)
	}
	if c := ask.Choice("b"); c == nil || c.Text != "プロジェクトごと" {
		t.Errorf("option b = %+v", c)
	}

	hook := ByID(nodes, "hook")
	if hook.Kind != KindTodo || hook.Text != "hook を入れるか決める" || len(hook.Body) != 2 {
		t.Fatalf("hook = %+v (body %d)", hook, len(hook.Body))
	}
	if hook.Body[0].Kind != KindPara || hook.Body[1].Kind != KindPlain || hook.Body[1].Text != "箇条書きの中身" {
		t.Errorf("hook body = %+v %+v", hook.Body[0], hook.Body[1])
	}

	impl, parts := ByID(nodes, "impl"), ByID(nodes, "parts")
	if impl.Kind != KindDoing || impl.Status != "作業中" {
		t.Errorf("impl = %+v", impl)
	}
	if parts.Status != "あなた待ち" || parts.Reason != "上の問いの答え" {
		t.Errorf("parts = %+v", parts)
	}

	link := ByID(nodes, "spec")
	if link.Kind != KindLink || link.Path != "pitboard/SPEC.md" || link.Text != "最小版の仕様" {
		t.Errorf("link = %+v", link)
	}
	var code *Node
	WalkAll(nodes, func(n *Node) {
		if n.Kind == KindCode {
			code = n
		}
	})
	if code == nil || code.Lang != "diff" || strings.Join(code.Code, "|") != "-old|  +new indented" {
		t.Errorf("code = %+v", code)
	}

	fold := nodes[5]
	if fold.Kind != KindDetails || fold.Text != "済んだこと (2)" || len(fold.Kids) != 2 || fold.Key == "" {
		t.Fatalf("fold = %+v", fold)
	}
	if fold.Kids[0].Kind != KindDone || fold.Kids[0].Depth != 1 || fold.Kids[1].Kind != KindPlain {
		t.Errorf("fold contents = %+v %+v", fold.Kids[0], fold.Kids[1])
	}
	// The contents of a Section stay at its level; a Fold's are indented.
	if hook.Depth != 0 || ask.Kids[0].Depth != 1 {
		t.Errorf("depths: hook %d, option %d", hook.Depth, ask.Kids[0].Depth)
	}
}

func TestParseReportOneLineAndAttributes(t *testing.T) {
	nodes, probs, _ := Parse(`<Item id="q" status="say &quot;hi&quot;">a <b and **c**</Item>`)
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := ByID(nodes, "q")
	if n.Text != "a <b and **c**" || n.Status != `say "hi"` {
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
			_, probs, _ := Parse(c.src)
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
	nodes, probs, _ := Parse("<Card>\n<Item id=\"a\">x</Item>\n</Card>\n<Item id=\"b\">y</Item>")
	if len(probs) != 1 {
		t.Errorf("problems = %v", probs)
	}
	if ByID(nodes, "a") == nil || ByID(nodes, "b") == nil {
		t.Errorf("items after an error were lost")
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
			_, probs, err := Parse(c.src)
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
		if _, _, err := Parse(src); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLowercaseAndViewerKeys(t *testing.T) {
	nodes, probs, _ := Parse("<section title=\"s\">\n<item id=\"a\">x</item>\n</Section>\n<Ask id=\"q\">?\n<Option key=\"y\">yes</Option>\n<Option key=\"a\">ok</Option>\n</Ask>")
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
	if ByID(nodes, "a") == nil {
		t.Error("a lowercase part is still read")
	}
}

// The message for an unknown part lists every part, Notice too.
func TestPartNamesListEveryPart(t *testing.T) {
	if len(partNames) != len(parts) {
		t.Errorf("partNames %v, parts has %d", partNames, len(parts))
	}
	for _, n := range partNames {
		if _, ok := parts[n]; !ok {
			t.Errorf("%s is not a part", n)
		}
	}
	_, probs, _ := Parse("<Bogus>x</Bogus>\n")
	if len(probs) == 0 || !strings.Contains(probs[0].Msg, "Fold Notice Code") {
		t.Errorf("problems %v", probs)
	}
}
