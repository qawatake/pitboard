package viewer

import (
	"github.com/qawatake/pitboard/internal/board"
)

// Messages on the bottom line.
const (
	msgChanged   = "報告が書き換わりました。もう一度押してください"
	msgMoved     = "報告が書き換わりました。カーソルの行を確かめてください"
	msgNoReport  = "AI はまだ報告を書いていません"
	msgEmpty     = "報告はまだ空です。AI が書くと、ここに出ます。"
	msgNoBoard   = "このディレクトリと親に pitboard の置き場 (.pitboard/) がありません。AI を動かしているディレクトリで開いてください"
	msgNoSession = "pitboard にはまだ報告がありません。AI が報告を書くと、ここに出ます"
)

// settleHead is the head word of what the AI wrote on a comment.
func settleHead(state string) string {
	switch state {
	case "doing":
		return "対応中: "
	case "hold":
		return "保留: "
	}
	return "対応: "
}

// statusWord is a comment's status in one word.
func statusWord(s board.Status) string {
	switch s {
	case board.StatusUnsent:
		return "未送信"
	case board.StatusSent:
		return "送信済"
	case board.StatusRead:
		return "既読"
	case board.StatusDoing:
		return "対応中"
	case board.StatusHold:
		return "保留"
	}
	return "対応済"
}

// statusOf is a comment's status in one word, and its color: only what
// needs the person (not sent) is in color. What the AI wrote is read in the
// list (tab, enter).
func (a *app) statusOf(t *board.Thought) (word, color string) {
	s := a.state.StatusOf(t)
	if s == board.StatusUnsent {
		return statusWord(s), yellow
	}
	return statusWord(s), dim
}
