package page

import (
	"fmt"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// What a screen says, in the person's language. Every screen uses these.
const (
	MsgChanged = "報告が書き換わりました。もう一度押してください"
	MsgMoved   = "報告が書き換わりました。カーソルの行を確かめてください"
	// MsgAlreadyHidden refuses x on a line already hidden while hidden
	// lines are not shown (a press that came late): pressing again does
	// the same.
	MsgAlreadyHidden = "この行はもう隠れています"
	MsgNoReport      = "AI はまだ報告を書いていません"
	MsgEmpty         = "報告はまだ空です。AI が書くと、ここに出ます"
	MsgNoBoard       = "このディレクトリと親に pitboard の置き場 (.pitboard/) がありません。AI を動かしているディレクトリで開いてください"
	MsgNoSession     = "pitboard にはまだ報告がありません。AI が報告を書くと、ここに出ます"
	MsgProblems      = "報告に書き方の誤りがあります"
	MsgPartial       = " (読めた所までを出しています)"
	// MsgFixing is the quiet line over the last good version, shown while
	// report.md has problems (Report.Fixing).
	MsgFixing  = "AI が報告を直しています"
	MsgNotSent = "まだ送っていません (s で送る)"
	MsgNone    = "なし" // under the heading of a Section that holds nothing (faint)
)

// Whose turn it is, first at the top right (Board.AITurn).
const (
	WordAITurn   = "AI の番"
	WordYourTurn = "あなたの番"
)

// MsgNoSuchSession is what is shown for a session that has no board yet.
func MsgNoSuchSession(session string) string {
	return fmt.Sprintf("セッション %s の報告はまだありません", session)
}

// ProblemLine is one problem of the report, as shown.
func ProblemLine(p report.Problem) string { return fmt.Sprintf("%d 行目: %s", p.Line, p.Msg) }

// Tone is what a color means; each screen picks the color.
type Tone int

const (
	ToneQuiet  Tone = iota // faint: waits, or is done
	ToneNeed               // yellow: needs the person
	ToneMove               // green: moves (a doing Item)
	ToneNormal             // the normal text color: no meaning (あなたの番)
)

// Word is a word at the right end of a line, and what its color means.
type Word struct {
	Text string
	Tone Tone
}

// SettleHead is the head word of what the AI wrote on a comment.
func SettleHead(state string) string {
	switch state {
	case "doing":
		return "対応中: "
	case "hold":
		return "保留: "
	}
	return "対応: "
}

// StatusWord is a comment's status in one word.
func StatusWord(s board.Status) string {
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

// DoneFoldWord is the one line the n comments the AI is done with on a
// line fold into, under it (after ✎, faint).
func DoneFoldWord(n int) string { return fmt.Sprintf("対応済 %d 件", n) }

// ThoughtWord is a comment's status in one word, in the palette's
// meaning: not sent needs the person, 対応中 moves (the AI works on it),
// the rest are quiet.
func (b *Board) ThoughtWord(t *board.Thought) Word {
	switch s := b.State.StatusOf(t); s {
	case board.StatusUnsent:
		return Word{StatusWord(s), ToneNeed}
	case board.StatusDoing:
		return Word{StatusWord(s), ToneMove}
	default:
		return Word{StatusWord(s), ToneQuiet}
	}
}

// ThoughtNote is what an opened comment says about how far it got, when
// the AI has not written on it. MsgNotSent is the one for a comment not
// sent (a list of comments may say it once for all).
func (b *Board) ThoughtNote(t *board.Thought) string {
	switch {
	case t.LastN > b.State.Sent:
		return MsgNotSent
	case t.LastN > b.State.Seen:
		return "送りました。AI はまだ読んでいません"
	}
	return "AI は読みました。対応はまだです"
}

// Right is what goes at the right end of a line: the person's answer on
// what the line says now and the state of their latest input on it, then
// the AI's status.
type Right struct {
	// Reaction is the person's answer on an Ask: the option's key ("" for
	// none, or when it is being taken back).
	Reaction string
	// Option is, on an Ask with an answer, the chosen option's text, and
	// OptionHead its head as a closed Ask shows it (AnswerHead cells).
	Option     string
	OptionHead string
	// Count is how many comments the line has, when the latest input is a
	// comment and there are more than one (0 otherwise).
	Count int
	// Input is the state of the latest input (未送信, 送信済, 既読, the
	// AI's state for a comment, 取り消し 未送信); "" when there is none.
	Input Word
	// Status is the AI's status (or 作業中 for a doing Item without one).
	Status Word
	// At is, on a Notice, when the AI put it up (shown in place of
	// Status; see AtWord).
	At time.Time
	// AtText is At as shown (AtWord, now); set by Page.
	AtText string
}

// WorkingWord is what a doing Item without a status says.
const WorkingWord = "作業中"

// UndoingWord is what a line says while a sent answer (or hide) on it is
// taken back and the taking back not sent.
const UndoingWord = "取り消し 未送信"

// RightOf is the right end of n.
func (b *Board) RightOf(n *report.Node) Right {
	var r Right
	switch {
	case n.Status == "" && n.Kind == report.KindDoing:
		r.Status = Word{WorkingWord, ToneMove}
	case n.Status == "":
	case n.Wait == "you":
		r.Status = Word{n.Status, ToneNeed}
	case n.Kind == report.KindDoing:
		r.Status = Word{n.Status, ToneMove}
	default:
		r.Status = Word{n.Status, ToneQuiet}
	}
	if n.Kind == report.KindNotice && !n.At.IsZero() {
		r.Status, r.At = Word{}, n.At
	}
	id := report.Target(n)
	if id == "" {
		return r
	}
	e, reacted := b.State.React[id]
	reacted = reacted && e.Line == n.Sig()
	if reacted {
		r.Reaction = e.Value
		if ch := n.Choice(e.Value); n.Kind == report.KindQuestion && ch != nil {
			r.Option = report.PlainOf(ch.Text)
			r.OptionHead = Cut(r.Option, AnswerHead)
		}
	}
	cs := b.State.On(id)
	var last *board.Thought
	for _, t := range cs {
		if last == nil || t.LastN > last.LastN {
			last = t
		}
	}
	switch {
	case b.State.Undoing[id] || b.State.UndoingKey[report.HideKey(n)]:
		r.Reaction, r.Option, r.Input = "", "", Word{UndoingWord, ToneNeed}
	case reacted && (last == nil || e.N > last.LastN):
		w := StatusWord(b.State.SentStatus(e.N))
		r.Input = Word{w, ToneQuiet}
		if w == "未送信" {
			r.Input.Tone = ToneNeed
		}
	case last != nil:
		r.Input = b.ThoughtWord(last)
		if len(cs) > 1 {
			r.Count = len(cs)
		}
	}
	return r
}

// Clock is the time a screen shows things at (the gallery's server fixes
// it, so a page reads the same as its fixture).
var Clock = time.Now

// AtWord is when a notice was put up: the time today, else the day too.
func AtWord(t, now time.Time) string {
	if y, m, d := now.Date(); t.Year() == y && t.Month() == m && t.Day() == d {
		return t.Format("15:04")
	}
	return t.Format("1/2 15:04")
}
