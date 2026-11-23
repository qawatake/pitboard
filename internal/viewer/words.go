package viewer

import (
	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

// Messages on the bottom line: the words are internal/page's, so every
// screen says the same.
const (
	msgChanged   = page.MsgChanged
	msgMoved     = page.MsgMoved
	msgNoReport  = page.MsgNoReport
	msgEmpty     = page.MsgEmpty
	msgNoBoard   = page.MsgNoBoard
	msgNoSession = page.MsgNoSession

	// Why a key did nothing.
	msgBack          = "カーソルの行に戻りました"
	msgPlacedYN      = "j k で動かすと y n が効きます"
	msgPlacedAnswer  = "j k で動かすと答えられます"
	msgSettlingReact = "いま移った行です。もう一度押すと付きます"
	msgSettlingHide  = "いま移った行です。もう一度押すと隠れます"
	msgSettlingShow  = "いま移った行です。もう一度押すと表示に戻ります"
	msgReappeared    = "隠した行が書き換わったので、また出しました: "
)

// settleHead is the head word of what the AI wrote on a comment.
func settleHead(state string) string { return page.SettleHead(state) }

// statusOf is a comment's status in one word, and its color: only what
// needs the person (not sent) is in color. What the AI wrote is read in the
// list (tab, enter).
func (a *app) statusOf(t *board.Thought) (word, color string) {
	w := a.b.ThoughtWord(t)
	return w.Text, toneColor(w.Tone)
}
