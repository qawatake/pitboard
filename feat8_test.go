package main

import (
	"os"
	"strings"
	"testing"
)

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
	nodes, probs, _ := parseReport(richItem)
	if len(probs) > 0 {
		t.Fatalf("problems: %v", probs)
	}
	body := nodes[0].body
	var got []string
	var dump func(ns []*node, d int)
	dump = func(ns []*node, d int) {
		for _, n := range ns {
			s := strings.Repeat(" ", d)
			switch {
			case n.kind == kPlain:
				s += "L" + n.num + ":" + n.text
			case n.head:
				s += "H:" + n.text
			default:
				s += "P:" + n.text
			}
			if n.gap {
				s += " (gap)"
			}
			got = append(got, s)
			dump(n.subs, d+1)
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

func TestRichTextDrawn(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(richItem), 0o644)
	a := testApp(t, st)
	a.t.w = 44
	pv := a.top()
	a.render()
	a.handle(key{name: "enter"})
	s := shown(a)
	for _, want := range []string{
		"\n        おすすめは折衷。\n\n        · cwd に置く\n          - 良い点: 書ける続きの行\n",
		"\n        · 折衷: データは cwd\n\n        手順\n        1. まず cwd\n        2. 次に XDG\n           - 入れ子\n",
		"\n        行末で改行する\n        次の行\n        その次の行\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("want %q in\n%s", want, s)
		}
	}
	_ = pv
	// Wrapping keeps to where the text starts, past the number.
	os.WriteFile(st.reportPath(), []byte("<Item id=\"a\">t\n    1. "+strings.Repeat("あ", 40)+"\n      - "+strings.Repeat("い", 40)+"\n</Item>"), 0o644)
	a.reload(pv)
	pv.open["a"] = true
	s = shown(a)
	if !strings.Contains(s, "\n        1. あああ") || !strings.Contains(s, "\n           ああ") || !strings.Contains(s, "\n           - いい") || !strings.Contains(s, "\n             いい") {
		t.Errorf("wrapping:\n%s", s)
	}
}

func TestListErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"<Section title=\"s\">\n- a\n   - b\n</Section>", "3: list lines are indented by 2"},
		{"<Section title=\"s\">\n- a\n  - b\n    - c\n      - d\n</Section>", "5: lists nest 3 levels at most"},
		{"<Section title=\"s\">\n- a\n  - b\n    - c\n</Section>", ""},
	} {
		_, probs, _ := parseReport(c.src)
		got := ""
		for _, p := range probs {
			got += p.String()
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%q: %q (want %q)", c.src, got, c.want)
		}
	}
}

func TestNestedListHidesWithItsLine(t *testing.T) {
	src := "<Section title=\"s\">\n- 板はセッションごと\n  - 理由 A\n- ほか\n</Section>"
	nodes, _, _ := parseReport(src)
	top := nodes[0].kids[0]
	if k := hideKey(top); k != "-|板はセッションごと\n  -理由 A" {
		t.Errorf("key %q", k)
	}
	if k := hideKey(nodes[0].kids[1]); k != "-|ほか" {
		t.Errorf("key without nesting changed: %q", k)
	}
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(src), 0o644)
	a := testApp(t, st)
	pv := a.top()
	if keys := pv.navKeys(); len(keys) != 2 {
		t.Errorf("stops %q: nested lines must not be stops", keys)
	}
	a.render()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	if s := shown(a); strings.Contains(s, "理由 A") || strings.Contains(s, "板はセッションごと") {
		t.Errorf("hidden with its nested lines:\n%s", s)
	}
	// A numbered line is read only.
	nodes, _, _ = parseReport("<Section title=\"s\">\n1. a\n</Section>")
	if pv.navigable(nodes[0].kids[0]) {
		t.Error("a numbered line is a stop")
	}
}
