package web

import (
	"testing"
)

const fix22Src = `<Title>t</Title>
<Section title="s">
<Item id="a">A</Item>
<Item id="b">B</Item>
<Item id="c">C</Item>
<Item id="d">D</Item>
</Section>
`

// FIX22: x x x hides a, b and c: after the first, each press is on the line
// the page moved the cursor to (how "hid"), at once. A press still on the
// line just hidden (sent before the page came back) does not show it again.
// u takes back the last only.
func TestHideRowsInARowWeb(t *testing.T) {
	f := setupWith(t, fix22Src)
	f.page("tab1")
	cur := Cur{How: "person"}
	for _, k := range []string{"a", "b", "c"} {
		tok := f.thing(k).Token
		cur.Sel = tok
		r := f.act("tab1", Act{Action: "key", Key: "x", Target: k, Seen: tok, Cur: cur})
		if !r.OK || !r.Hidden {
			t.Fatalf("x on %s: %+v", k, r)
		}
		// Pressed again before the page came back: still on k.
		if r := f.act("tab1", Act{Action: "key", Key: "x", Target: k, Seen: tok, Cur: cur}); r.OK {
			t.Fatalf("x again on hidden %s: %+v", k, r)
		}
		f.page("tab1")
		cur = Cur{How: "hid", Ago: 0}
	}
	if got := f.ops(); len(got) != 3 || got[0] != "hide" || got[1] != "hide" || got[2] != "hide" {
		t.Fatalf("ops %v", got)
	}
	if r := f.act("tab1", Act{Action: "undo"}); !r.OK {
		t.Fatal(r)
	}
	f.page("tab1")
	if f.things["c"] == nil || f.things["b"] != nil || f.things["a"] != nil {
		t.Errorf("u must bring back c only: a %v b %v c %v", f.things["a"] != nil, f.things["b"] != nil, f.things["c"] != nil)
	}
}
