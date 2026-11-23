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
