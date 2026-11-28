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

	// Why a key did nothing (page's guard says which).
	msgBack          = page.MsgBack
	msgPlacedAnswer  = page.MsgPlacedAnswer
	msgSettlingReact = page.MsgSettlingReact
	msgSettlingHide  = page.MsgSettlingHide
	msgSettlingShow  = page.MsgSettlingShow
	msgReappeared    = page.MsgReappeared
)

// settleHead is the head word of what the AI wrote on a comment.
func settleHead(state string) string { return page.SettleHead(state) }

// statusOf is a comment's status in one word, and its color: only what
// needs the person (not sent) is in color. What the AI wrote is read on the
// comments screen (l; o for all of it).
func (a *app) statusOf(t *board.Thought) (word, color string) {
	w := a.b.ThoughtWord(t)
	return w.Text, toneColor(w.Tone)
}
