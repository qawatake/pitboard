package page

import (
	"reflect"
	"slices"

	"github.com/qawatake/pitboard/internal/board"
)

// What both screens say around the lines: the words of the keys that are
// not a line's own, when a message on the bottom line goes, and the names
// of every value a screen draws (for a screen's catalog of its states).

// The words of keys on comments and in the editor. x on a comment takes
// it back (取り消す); u undoes the person's last act (元に戻す): two words,
// so the bar never shows the same word for two keys.
const (
	WordUndo    = "元に戻す"
	WordRetract = "取り消す"
	WordEdit    = "直す"
	WordGo      = "行へ" // a comment in the browser's comment panel: to its line
	WordClose   = "閉じる"
	WordPlace   = "置く"
	WordStop    = "やめる"
	WordNewline = "改行"
)

// CommentHints is the keys on a comment (in a list of comments): e to
// edit and x to take it back, when the person's acts can be recorded.
func CommentHints(canWrite bool) []Hint {
	if !canWrite {
		return nil
	}
	return []Hint{{"edit", "e", WordEdit}, {"retract", "x", WordRetract}}
}

// EditorHints is the keys while writing: only these (no key of the line
// under it, which would be typed into the text). newline is the key the
// screen takes for a new line (shift+enter, or ctrl+j on a terminal that
// cannot tell it from enter).
func EditorHints(newline string) []Hint {
	return []Hint{{"place", "enter", WordPlace}, {"stop", "esc", WordStop}, {"newline", newline, WordNewline}}
}

// MsgEvent is something that happens on a screen while a message is on
// its bottom line.
type MsgEvent int

const (
	OnKey     MsgEvent = iota // the person pressed a key (any, even one that does nothing)
	OnEditor                  // the editor opened
	OnPanel                   // a panel or list opened or closed (comments, keys, an image)
	OnReload                  // the page was read again (the board changed on disk)
	OnPointer                 // the person moved the cursor with the pointer
)

// MsgClears says whether e takes the message off the bottom line. A
// message is about the last thing the person did: it stays until they do
// the next (a key, or moving the cursor), or the screen changes under it
// (an editor or panel takes the place it was about). A reload alone keeps
// it: the board changing on disk is not the person's doing.
func MsgClears(e MsgEvent) bool { return e != OnReload }

// Enums is every value of what a screen draws differently, by name: the
// kinds of line, inline styles, tones, marks, an Item's states, a
// comment's statuses, the AI's settle states, and the actions a line can
// take. A screen's catalog of its states covers each (see
// docs/spec/web-states.md).
func Enums() map[string][]string {
	var status []string
	for s := board.StatusUnsent; s <= board.StatusDone; s++ {
		status = append(status, StatusWord(s))
	}
	var actions []string
	at := reflect.TypeFor[Actions]()
	for i := range at.NumField() {
		actions = append(actions, at.Field(i).Name)
	}
	var hints []string
	for _, h := range []string{"open", "close", "read", "answer", "wait", "y", "n", "comment", "hide",
		"undo", "next", "jump", "send", "write", "keys", "edit", "retract", "place", "stop", "newline"} {
		hints = append(hints, h)
	}
	var refusals []string
	for k := range Refusals {
		refusals = append(refusals, k)
	}
	slices.Sort(refusals)
	return map[string][]string{
		"refusal": refusals,
		"kind":    append([]string(nil), kindNames...),
		"style":   append([]string(nil), styleNames...),
		"tone":    append([]string(nil), toneNames...),
		"mark":    append([]string(nil), markNames...),
		"state":   {"todo", "doing", "done"},
		"status":  status,
		"settle":  {"doing", "hold", "done"},
		"action":  actions,
		"hint":    hints,
	}
}

// Refusals is, by name, every message that says why a key did nothing
// (the screens show them as needing the person).
var Refusals = map[string]string{
	"back": MsgBack, "placedYN": MsgPlacedYN, "placedAnswer": MsgPlacedAnswer,
	"settlingReact": MsgSettlingReact, "settlingHide": MsgSettlingHide, "settlingShow": MsgSettlingShow,
	"noMarked": MsgNoMarked, "changed": MsgChanged, "moved": MsgMoved,
	"emptyComment": MsgEmptyComment, "tooLong": MsgTooLong,
}

// The order a screen leaves keys out in when its line of keys has no room,
// lowest first; a key without a rank is kept: u, then x and space, then m
// and ?, g, enter, n, y (and r, an answer), c. enter and g outlast m and ?
// (on a phone the line's own keys matter more than the page's). The
// terminal keeps x longer before the person moves (see its hints).
const (
	RankUndo = iota + 1
	RankHide
	RankPage
	RankJump
	RankEnter
	RankN
	RankY
	RankComment
)

// DropRank is the rank of the key that does do (0: kept).
func DropRank(do string) int {
	switch do {
	case "undo":
		return RankUndo
	case "hide", "next":
		return RankHide
	case "write", "keys":
		return RankPage
	case "jump":
		return RankJump
	case "open", "close":
		return RankEnter
	case "n":
		return RankN
	case "y", "read", "answer":
		return RankY
	case "comment":
		return RankComment
	}
	return 0
}

// DropRanks is DropRank for every key that has a rank.
func DropRanks() map[string]int {
	out := map[string]int{}
	for _, d := range Enums()["hint"] {
		if r := DropRank(d); r > 0 {
			out[d] = r
		}
	}
	return out
}
