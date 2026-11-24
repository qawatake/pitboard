package page

import "testing"

const linksSrc = `<Title>題</Title>
<Section title="s">
  <Item id="a">見て [作業](#work) と [作業 again](#work) と [自分](#a)
    中身の [もう一つ](#other)
    - 箇条の [三つ目](#n1)
  </Item>
  <Item id="other">もう一つ</Item>
</Section>
<Notice id="n1">お知らせ</Notice>
<Fold title="外">
  <Fold title="中">
    <Item id="work">作業の行</Item>
  </Fold>
</Fold>
`

// LinksOf: in order, one per target, not to itself; Follow opens every
// Fold around the target; Thing spans carry Target and the digit.
func TestRowLinks(t *testing.T) {
	b := open(t, linksSrc)
	a := find(b.Page(false).Things, "a")
	if len(a.Links) != 3 || a.Links[0] != (Link{"作業", "work", 1}) || a.Links[1].Target != "other" || a.Links[2].Num != 3 {
		t.Fatalf("links %+v", a.Links)
	}
	var tgt Span
	for _, s := range a.Title {
		if s.Target == "work" {
			tgt = s
			break
		}
	}
	if tgt.Num != 1 || tgt.URL != "" || tgt.Style != StyleLink {
		t.Errorf("title span %+v", tgt)
	}
	f, ok := b.Follow("work")
	if !ok || f.Key == "" || len(f.Open) != 2 || f.Hidden {
		t.Errorf("follow %+v %v", f, ok)
	}
	if _, ok := b.Follow("nope"); ok {
		t.Error("followed a row that is not there")
	}
	// A target gone from the report (written since check ran) is plain.
	b2 := open(t, "<Item id=\"x\">[gone](#zz)</Item>\n")
	x := find(b2.Page(false).Things, "x")
	if len(x.Links) != 0 || x.Title[0].Style != StylePlain {
		t.Errorf("unknown target %+v %+v", x.Links, x.Title)
	}
}
