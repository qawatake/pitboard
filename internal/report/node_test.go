package report

import (
	"strings"
	"testing"
)

func TestHideKey(t *testing.T) {
	nodes, _, _ := Parse(`<Section title="決まったこと">
- 板はセッションごと
- 板はセッションごと
<Item id="a" state="doing" status="作業中">題</Item>
<Ask id="q">問い
<Option key="a">x</Option>
<Option key="b">y</Option>
</Ask>
<Link id="l" path="p">リンク</Link>
<Fold title="済んだこと (2)">x</Fold>
<Fold id="f" title="t">x</Fold>
段落
</Section>`)
	var keys []string
	WalkAll(nodes, func(n *Node) { keys = append(keys, HideKey(n)) })
	want := []string{
		"", // Section
		"-|板はセッションごと",
		"-|板はセッションごと",
		"Item|a|題|doing|作業中",
		"Ask|q|問い||", "", "",
		"Link|l|リンク||",
		"Fold|title=済んだこと (2)", "",
		"Fold|id=f", "",
		"", // paragraph
	}
	if strings.Join(keys, "\n") != strings.Join(want, "\n") {
		t.Errorf("keys:\n%q\nwant\n%q", keys, want)
	}
}

func TestSigChangesWithTheLine(t *testing.T) {
	a, _, _ := Parse(`<Item id="x" status="作業中">do it</Item>`)
	b, _, _ := Parse(`<Item id="x" status="あなた待ち">do it</Item>`)
	c, _, _ := Parse("\n\n" + `<Item id="x" status="作業中">do it</Item>`)
	if ByID(a, "x").Sig() == ByID(b, "x").Sig() {
		t.Error("a changed status must change the signature")
	}
	if ByID(a, "x").Sig() != ByID(c, "x").Sig() {
		t.Error("moving the line must not change the signature")
	}
}

func TestFoldKeys(t *testing.T) {
	a, _, _ := Parse("<Fold title=\"Done (2)\">\n</Fold>\n<Fold id=\"old\" title=\"Old (1)\">\n</Fold>")
	b, _, _ := Parse("<Fold title=\"Done (3)\">\n</Fold>\n<Fold title=\"New\">\n</Fold>\n<Fold id=\"old\" title=\"Old (2)\">\n</Fold>")
	if a[0].Key != b[0].Key {
		t.Errorf("retitling a fold changed its key: %q -> %q", a[0].Key, b[0].Key)
	}
	if a[1].Key != "old" || b[2].Key != "old" {
		t.Errorf("a fold with an id is tracked by it: %q %q", a[1].Key, b[2].Key)
	}
}

// Lines of text (not rows) are keyed by what they say, those of a body
// under their row's key, and recorded by the row they are in.
func TestLineKeysAndTargets(t *testing.T) {
	nodes, _, _ := Parse("<Title>題</Title>\n\n段落。\n\n<Item id=\"a\">項目\n  段落。\n  1. 番号\n</Item>\n<Fold title=\"畳み\">\n- 行\n  - 下\n</Fold>\n")
	var got []string
	WalkDeep(nodes, func(n *Node) {
		if Row(n) || Line(n) {
			got = append(got, n.Key+" => "+strings.ReplaceAll(Target(n), "\n", "/"))
		}
	})
	want := []string{
		"t1:題 => Line|/題",
		"p1:段落。 => Line|/段落。",
		"a => a",
		"a/p1:段落。 => Line|a/段落。",
		"a/n1:1. 番号 => Line|a/番号",
		"fold:1 => Fold|title=畳み",
		"-1:行 => Line|Fold|title=畳み/行",
		"-1:行/-1:下 => Line|Fold|title=畳み/下",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	anchor, text, ok := LineOf("Line|Fold|title=畳み\n下")
	if !ok || anchor != "Fold|title=畳み" || text != "下" {
		t.Errorf("LineOf: %q %q %v", anchor, text, ok)
	}
	if _, _, ok := LineOf("a"); ok {
		t.Error("an id is a line")
	}
}
