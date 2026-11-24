package page

import (
	"strconv"

	"github.com/qawatake/pitboard/internal/board"
)

// The palette: the colors a screen may use, each with one meaning. A
// screen maps each name to a color of its own (the browser: a CSS token
// of the same name; the terminal: an escape) and uses a color only for
// its meaning. Everything else is the normal text color, and hierarchy
// is spacing and weight.
const (
	ColorNeed      = "need"      // yellow: needs the person (waiting ●, 反応待ち, 未送信, a refusal)
	ColorMove      = "move"      // green: moving (作業中, 対応中), the unread ●, an added diff line
	ColorQuiet     = "quiet"     // faint: done, waiting on the AI, secondary notes, a diff's @@
	ColorStructure = "structure" // blue: headings, links, the cursor and the focus, g's letters
	ColorYours     = "yours"     // teal: what the person entered (an answer's key, the chosen option)
	ColorBad       = "bad"       // red: errors, a removed diff line
)

// Palette is every color name with what it means, as the keys list says
// it, in order (SPEC and each screen's mapping follow it; tests check both
// screens map every name).
var Palette = []struct{ Name, Label string }{
	{ColorNeed, "黄: あなたの手が要る (人待ちの ●・未送信・断り)"},
	{ColorMove, "緑: 動いている (作業中・対応中)・未読の ●・足した行"},
	{ColorQuiet, "薄い: 済んだ・AI 待ち・添え書き"},
	{ColorStructure, "青: 見出し・リンク・カーソル・g の字"},
	{ColorYours, "青緑: あなたが入れたもの (答えのキー・選んだ答え)"},
	{ColorBad, "赤: 誤り・消した行"},
}

// ToneColor is the palette name of a Tone.
func ToneColor(t Tone) string {
	switch t {
	case ToneNeed:
		return ColorNeed
	case ToneMove:
		return ColorMove
	}
	return ColorQuiet
}

// DiffColor is the palette name of a diff line ("" for context): + move,
// - bad, @@ quiet, all at the same weight.
func DiffColor(line string) string {
	switch {
	case len(line) >= 2 && line[:2] == "@@":
		return ColorQuiet
	case len(line) >= 1 && line[0] == '+':
		return ColorMove
	case len(line) >= 1 && line[0] == '-':
		return ColorBad
	}
	return ""
}

// PendingCount is how many inputs the person has not sent, as they see
// them: one per line with an answer, hide or taking back pending on it
// (answering a, then b on one Ask is one), and one per comment (written,
// edited, taken back: one). Both screens' 未送信 N.
func PendingCount(unsent []board.Event) int {
	seen := map[string]bool{}
	for _, e := range unsent {
		var k string
		switch e.Op {
		case "react":
			k = "r:" + e.ID
		case "hide", "unhide":
			k = "h:" + e.Key
		case "undo": // a reaction's or a hide's, taken back after it was sent
			if e.ID != "" {
				k = "r:" + e.ID
			} else {
				k = "h:" + e.Key
			}
		case "thought":
			k = "c:" + strconv.Itoa(e.N)
		case "edit", "retract", "unretract":
			k = "c:" + strconv.Itoa(e.Thought)
		default:
			k = "e:" + strconv.Itoa(e.N)
		}
		seen[k] = true
	}
	return len(seen)
}
