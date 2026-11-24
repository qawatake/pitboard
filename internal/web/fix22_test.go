package web

import (
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

const fix22Src = `<Title>t</Title>
<Section title="s">
<Item id="a">A</Item>
<Item id="b">B</Item>
<Item id="c">C</Item>
<Item id="d">D</Item>
</Section>
`

// FIX22, FIX23: hiding a, b and c is x, then x x on each line the page
// moved the cursor to: the first x there is dropped with the settling
// message, the second hides it. A press still on the line just hidden (sent
// before the page came back) does not show it again. u takes back the last
// only.
func TestHideRowsInARowWeb(t *testing.T) {
	f := setupWith(t, fix22Src)
	f.page("tab1")
	cur := Cur{How: "person"}
	for i, k := range []string{"a", "b", "c"} {
		tok := f.thing(k).Token
		cur.Sel = tok
		if i > 0 {
			r := f.act("tab1", Act{Action: "key", Key: "x", Target: k, Seen: tok, Cur: cur})
			if r.OK || r.Message != page.MsgSettlingHide {
				t.Fatalf("first x on moved-to %s: %+v", k, r)
			}
		}
		r := f.act("tab1", Act{Action: "key", Key: "x", Target: k, Seen: tok, Cur: cur})
		if !r.OK || !r.Hidden {
			t.Fatalf("x on %s: %+v", k, r)
		}
		// Pressed again before the page came back: still on k.
		if r := f.act("tab1", Act{Action: "key", Key: "x", Target: k, Seen: tok, Cur: cur}); r.OK {
			t.Fatalf("x again on hidden %s: %+v", k, r)
		}
		f.page("tab1")
		cur = Cur{How: "app", Ago: 0}
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
