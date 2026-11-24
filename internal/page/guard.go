package page

import (
	"slices"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/report"
)

// The guard: whether a key pressed on the cursor's line acts on it, and if
// not, what to tell the person. Every screen asks here, so the rules and
// their words are the same on all of them. A screen only reports the facts
// (Touch): how the cursor came to the line, and since when.
//
//   - Placed: the screen put the cursor on the line by itself (at start, or
//     when stops appeared on a page that had none). Hiding it, saying it is
//     read and writing on it work (Actions.Untouched); an option answers
//     the AI, so it waits until the person moves.
//   - Locked: the line was rewritten (or went) under the cursor. No key acts
//     on it until the person moves, and every one says so.
//   - Settling: the screen moved the cursor to the line (after x, H or u took
//     the person's line away, or space / g went there) and it has not been
//     on screen for SettleDelay. x and options are dropped, once: the
//     same key pressed again on the same line works. x works at once on a
//     line x itself moved the cursor to (Hid): x x x hides line after line,
//     and u brings them back one by one.

// SettleDelay is how long a line the screen moved the cursor to must have
// been on screen before x and options work on it.
const SettleDelay = 400 * time.Millisecond

// Why a key did nothing.
const (
	MsgBack          = "カーソルの行に戻りました"
	MsgPlacedAnswer  = "j k で動かすと答えられます"
	MsgSettlingReact = "いま移った行です。もう一度押すと付きます"
	MsgSettlingHide  = "いま移った行です。もう一度押すと隠れます"
	MsgSettlingShow  = "いま移った行です。もう一度押すと表示に戻ります"
	MsgNoMarked      = "印のある行はありません"
	MsgJumpLetter    = "飛ぶ先の字を押してください (esc やめる)"
)

// Touch is how the cursor came to be on its line, as the screen saw it.
type Touch struct {
	Placed bool // the screen put the cursor there; the person has not moved it
	Locked bool // the line was rewritten (or went) under the cursor
	// Auto: the screen moved the cursor there (after x, H or u took the
	// person's line away, or for space / g). Shown is how long the line
	// has been on screen since (0: not yet).
	Auto  bool
	Shown time.Duration
	// Hid: the screen moved the cursor there because x hid the person's
	// line. x is not dropped then: the person is hiding down the page.
	Hid  bool
	Open bool // the line is open on screen (an Ask's options are shown)
	// Again is the press last dropped while settling (Verdict.Again): the
	// screen keeps it and gives it back with the next press.
	Again string
}

// Settling says whether the line was just moved to by the screen and has
// not been on screen for SettleDelay.
func (t Touch) Settling() bool { return t.Auto && t.Shown < SettleDelay }

// Do is what a key does on a line.
type Do string

const (
	DoNothing Do = ""
	DoReact   Do = "react"   // record Verdict.Value (React)
	DoHide    Do = "hide"    // hide it, or show it again (Hide)
	DoRead    Do = "read"    // say it is read (Read)
	DoComment Do = "comment" // open the editor for a comment on it
	DoOpen    Do = "open"    // only open it: an Ask's options are not on screen
)

// Verdict is what a key does on the cursor's line.
type Verdict struct {
	Do    Do
	Value string // DoReact: y, n or the option's key
	// Msg is, when nothing is done, why ("" when the key means nothing on
	// this line: a slip of the finger is not told).
	Msg string
	// Again is the Touch.Again to keep for the next press.
	Again string
}

// ActKey says whether key is one that acts on a line (y n x r c, or an
// option of n): the keys a locked line refuses with MsgMoved.
func ActKey(n *report.Node, key string) bool {
	if len(key) == 1 && strings.Contains("ynxrc", key) {
		return true
	}
	return n != nil && n.Kind == report.KindQuestion && key != "" && n.Choice(key) != nil
}

// Allow says what key does on n, the line under the cursor (nil: none, or
// not on screen), given how the cursor came there.
func (b *Board) Allow(n *report.Node, key string, t Touch) Verdict {
	v := Verdict{Again: t.Again}
	if n == nil || !ActKey(n, key) {
		return v
	}
	if t.Locked {
		v.Msg = MsgMoved
		return v
	}
	a := b.ActionsOf(n)
	// drop says whether the press is dropped because the line is settling:
	// the first is; the same key again on the same line works.
	drop := func() bool {
		if !t.Settling() {
			return false
		}
		k := n.Key + "\x00" + key
		if t.Again == k {
			v.Again = ""
			return false
		}
		v.Again = k
		return true
	}
	switch key {
	case "x":
		if !a.Hide || (t.Placed && !a.Untouched().Hide) {
			return v
		}
		if !t.Hid && drop() {
			v.Msg = MsgSettlingHide
			if b.IsHidden(n) {
				v.Msg = MsgSettlingShow
			}
			return v
		}
		v.Do = DoHide
		return v
	case "r":
		if !a.Read || (t.Placed && !a.Untouched().Read) {
			return v
		}
		v.Do = DoRead
		return v
	}
	if n.Kind == report.KindQuestion && !t.Open && slices.Contains(a.Options, key) {
		// The options are not on screen: the key only opens the question.
		v.Do = DoOpen
		return v
	}
	switch {
	case slices.Contains(a.Options, key):
	case a.Comment && key == "c":
		v.Do = DoComment
		return v
	default:
		return v
	}
	if t.Placed {
		if u := a.Untouched(); !slices.Contains(u.Options, key) {
			v.Msg = MsgPlacedAnswer
			return v
		}
	}
	if drop() {
		v.Msg = MsgSettlingReact
		return v
	}
	v.Do, v.Value = DoReact, key
	return v
}

// Hint is a key offered on the cursor's line, and what it does there.
type Hint struct {
	// Do says which it is: open, close, read, answer, wait (placed: j k,
	// and why), comment, hide, link (g 1: where the line's one link goes), back.
	Do   string
	Key  string
	Word string
}

// Offer is the keys that work on the cursor's line now, in the order to
// show them: a screen never offers a key the guard refuses.
type Offer struct {
	Hints []Hint
	// Options is the keys of the options offered (the answer hint).
	Options []string
	// Recheck is, while the line settles, how long until the offer
	// changes (0: it does not by itself).
	Recheck time.Duration
}

// Offer is what works on n (the cursor's line, on screen; nil none) now.
func (b *Board) Offer(n *report.Node, t Touch) Offer {
	var o Offer
	if n == nil {
		return o
	}
	var enter, read, answer, write, hide []Hint
	switch {
	case b.CanOpen(n) && t.Open:
		enter = []Hint{{"close", "enter", "閉じる"}}
	case b.CanOpen(n):
		enter = []Hint{{"open", "enter", "開く"}}
	}
	askOpen := n.Kind == report.KindQuestion && t.Open
	a := b.ActionsOf(n)
	if !t.Locked && report.Target(n) != "" && b.CanWrite() {
		choices := a.Options
		if !askOpen {
			choices = nil // the options of a closed Ask are not on screen
		}
		if t.Placed {
			// Placed by the screen, not by the person: say how to react.
			if askOpen {
				answer = []Hint{{"wait", "j k", "動かすと答えられる"}}
			}
			choices = nil
		}
		if t.Settling() {
			// These keys are dropped for a moment, so they are not offered.
			choices = nil
		}
		if len(choices) > 0 {
			answer = []Hint{{"answer", strings.Join(choices, " "), "答える"}}
			o.Options = choices
		}
		if a.Comment {
			write = []Hint{{"comment", "c", "行に書く"}}
		}
	}
	if !t.Locked && a.Read && (!t.Placed || a.Untouched().Read) {
		read = []Hint{{"read", "r", "読んだ"}}
	}
	if !t.Locked && a.Hide && (!t.Placed || a.Untouched().Hide) && (!t.Settling() || t.Hid) {
		if b.IsHidden(n) {
			hide = []Hint{{"hide", "x", "表示に戻す"}}
		} else {
			hide = []Hint{{"hide", "x", "隠す"}}
		}
	}
	// A line with one row link says where g 1 goes (the person learns
	// links can be followed); with more, g lists them.
	var link []Hint
	if ls := b.LinksOf(n); !t.Locked && len(ls) == 1 {
		link = []Hint{{"link", "g 1", b.LinkHintWord(ls[0], LinkHintMax)}}
	}
	if askOpen {
		o.Hints = slices.Concat(answer, enter, read, write, hide, link)
	} else {
		o.Hints = slices.Concat(enter, read, answer, write, hide, link)
	}
	if t.Settling() && !t.Locked {
		o.Recheck = SettleDelay - t.Shown
	}
	return o
}

// Rewritten says whether the line under the cursor, old as the person
// pointed at it, is now another line: gone (now nil), another kind of part,
// or another title (a Fold's title is only a label; status, note and state
// are not what the line is). A screen locks the cursor then.
func Rewritten(old, now *report.Node) bool {
	if old == nil || now == nil {
		return true
	}
	return now.Part() != old.Part() || (old.Kind != report.KindDetails && now.Title() != old.Title())
}

// MsgReappeared heads what is said when a hidden line comes back.
const MsgReappeared = "隠した行が書き換わったので、また出しました: "

// HiddenKeys is the keys of the lines hidden now: what Reappeared needs from
// before the report is read again.
func (b *Board) HiddenKeys() map[string]bool {
	hidden := map[string]bool{}
	report.WalkAll(b.Report.Nodes, func(n *report.Node) {
		if n.Key != "" && b.IsHidden(n) {
			hidden[n.Key] = true
		}
	})
	return hidden
}

// Reappeared is what to say when a line that was hidden (before:
// HiddenKeys) is not hidden now: the AI rewrote it (hiding goes by what it
// says), and without a word it looks as if x did nothing. "" when none.
func (b *Board) Reappeared(before map[string]bool) string {
	msg := ""
	report.WalkAll(b.Report.Nodes, func(n *report.Node) {
		if msg == "" && before[n.Key] && !b.IsHidden(n) && report.HideKey(n) != "" {
			msg = MsgReappeared + report.PlainOf(n.Title())
		}
	})
	return msg
}

// Bar is the keys a screen offers after the line's own (Offer), in the
// order the terminal's hint line has them: more (u to take back, space to
// the next marked line, g to jump; the first to go when there is no room)
// and tail (s to send, m to write, ? for the keys; kept to the end).
func (b *Board) Bar(showHidden bool) (more, tail []Hint) {
	if b.CanWrite() && len(b.Hist) > 0 {
		more = append(more, Hint{"undo", "u", WordUndo})
	}
	if b.AnyMarked(showHidden) {
		more = append(more, Hint{"next", "space", "次へ"})
	}
	if b.Store != nil && !b.NoSession && !b.Report.Missing && len(b.Jumps(showHidden)) > 0 {
		more = append(more, Hint{"jump", "g", "飛ぶ"})
	}
	if b.CanWrite() && b.Unsent > 0 {
		tail = append(tail, Hint{"send", "s", "送る"})
	}
	if b.CanWrite() {
		tail = append(tail, Hint{"write", "m", "書く"})
	}
	tail = append(tail, Hint{"keys", "?", "キー"})
	return more, tail
}
