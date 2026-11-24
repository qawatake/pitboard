// Package guardtable is the table of what a key does on the cursor's line
// (page's guard), for the tests of every screen: page asks the guard, the
// terminal presses the keys, the browser posts them. One table, so the
// screens cannot drift apart.
package guardtable

import "github.com/qawatake/pitboard/internal/page"

// Report is the report the cases run on.
const Report = `<Notice id="restart">開き直してください</Notice>
<Title>題</Title>

<Section title="s">
  <Item id="a">項目 a</Item>
  <Item id="b" state="doing" look="y">項目 b</Item>
  <Ask id="q">どちら
    <Option key="a">こちら</Option>
    <Option key="b">あちら</Option>
  </Ask>
  - ただの行
</Section>
`

// How the cursor came to its line.
const (
	Person   = "person"   // the person moved it there
	Placed   = "placed"   // the screen put it there at start
	Locked   = "locked"   // the line was rewritten under it
	Settling = "settling" // the screen just moved it there (after x, space, g)
	Settled  = "settled"  // the same, SettleDelay ago
	Hid      = "hid"      // the screen just moved it there because x hid the line
)

// Case is one key on one line.
type Case struct {
	Name string
	Line string // the cursor's line (key)
	How  string
	Open bool   // the line is open (an Ask's options shown)
	Key  string // pressed once, or twice (Twice: the second is what counts)
	// Twice: the same key is pressed twice on the line.
	Twice bool
	Do    page.Do
	Msg   string // what the guard says when it does nothing
	// Op is what is recorded when it is done (react, hide; "" nothing).
	Op string
}

// Cases is the table.
var Cases = []Case{
	{Name: "placed notice x hides", Line: "restart", How: Placed, Key: "x", Do: page.DoHide, Op: "hide"},
	{Name: "placed notice y does nothing", Line: "restart", How: Placed, Key: "y"},
	{Name: "placed item r reads", Line: "b", How: Placed, Key: "r", Do: page.DoRead},
	{Name: "placed item c writes", Line: "a", How: Placed, Key: "c", Do: page.DoComment},
	{Name: "placed item n does nothing", Line: "a", How: Placed, Key: "n"},
	{Name: "placed open ask waits", Line: "q", How: Placed, Open: true, Key: "a", Msg: page.MsgPlacedAnswer},
	{Name: "placed closed ask opens", Line: "q", How: Placed, Key: "b", Do: page.DoOpen},
	{Name: "locked x", Line: "a", How: Locked, Key: "x", Msg: page.MsgMoved},
	{Name: "locked r", Line: "a", How: Locked, Key: "r", Msg: page.MsgMoved},
	{Name: "locked c", Line: "a", How: Locked, Key: "c", Msg: page.MsgMoved},
	{Name: "locked option", Line: "q", How: Locked, Key: "a", Msg: page.MsgMoved},
	{Name: "settling x", Line: "a", How: Settling, Key: "x", Msg: page.MsgSettlingHide},
	{Name: "settling option", Line: "q", How: Settling, Open: true, Key: "b", Msg: page.MsgSettlingReact},
	{Name: "settling option twice", Line: "q", How: Settling, Open: true, Key: "b", Twice: true, Do: page.DoReact, Op: "react"},
	{Name: "settling x twice", Line: "a", How: Settling, Key: "x", Twice: true, Do: page.DoHide, Op: "hide"},
	{Name: "settling r reads", Line: "b", How: Settling, Key: "r", Do: page.DoRead},
	{Name: "settling c writes", Line: "a", How: Settling, Key: "c", Do: page.DoComment},
	{Name: "hid x hides at once", Line: "a", How: Hid, Key: "x", Do: page.DoHide, Op: "hide"},
	{Name: "hid option waits", Line: "q", How: Hid, Open: true, Key: "b", Msg: page.MsgSettlingReact},
	{Name: "settled option", Line: "q", How: Settled, Open: true, Key: "a", Do: page.DoReact, Op: "react"},
	{Name: "person y on an item does nothing", Line: "a", How: Person, Key: "y"},
	{Name: "person n on an item does nothing", Line: "a", How: Person, Key: "n"},
	{Name: "person x", Line: "a", How: Person, Key: "x", Do: page.DoHide, Op: "hide"},
	{Name: "person option", Line: "q", How: Person, Open: true, Key: "b", Do: page.DoReact, Op: "react"},
	{Name: "person list x", Line: "-1:ただの行", How: Person, Key: "x", Do: page.DoHide, Op: "hide"},
	{Name: "person list y means nothing", Line: "-1:ただの行", How: Person, Key: "y"},
	{Name: "person stray key", Line: "a", How: Person, Key: "z"},
	{Name: "person r on a read line", Line: "a", How: Person, Key: "r"},
}
