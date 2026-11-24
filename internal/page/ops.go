package page

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// The person's actions. Each records on the board, reads it again, and
// returns what to tell the person or an error whose text says what went
// wrong. Every action says something, for a screen that reads it out; the
// terminal leaves out what its screen shows by itself (see each). The screen decides which line the
// person is acting on, and whether they may act on it at all (pointed at
// it, it is on screen); what is checked here holds on every screen.

// ErrChanged is returned when the line acted on no longer says what the
// person saw (Seen): nothing was recorded, and the screen should show the
// report as it is now (and MsgChanged).
var ErrChanged = errors.New(MsgChanged)

// What actions say.
const (
	MsgEmptyComment = "空だったので書きませんでした"
	MsgCommented    = "コメントを置きました (s で送る)"
	MsgEdited       = "コメントを直しました"
	MsgRetracted    = "取り消しました (u で戻す)"
	MsgTooLong      = "長すぎるので受け取れませんでした"
	MsgOffline      = "つながっていません。pitboard serve が出した URL を開き直してください"
)

// ErrNoComment is Edit or Retract of a comment that is not there.
var ErrNoComment = errors.New("そのコメントはありません")

// ErrNothingToUndo is Undo with nothing to take back.
var ErrNothingToUndo = errors.New("取り消せる操作がありません")

// errCannotWrite is an action without a session to record it in.
var errCannotWrite = errors.New("このセッションには記録できません")

// Done is one thing the person did, for Undo: an event recorded (N), or the
// lines marked as read (Viewed). Desc is what Undo says it took back.
type Done struct {
	N      int
	Viewed []string
	Desc   string
}

func (b *Board) did(e board.Event, desc string) {
	b.Hist = append(b.Hist, Done{N: e.N, Desc: desc})
}

// node is the line with key in the report as the screen has it.
func (b *Board) node(key string) *report.Node {
	var found *report.Node
	report.WalkAll(b.Report.Nodes, func(n *report.Node) {
		if found == nil && key != "" && n.Key == key {
			found = n
		}
	})
	return found
}

// React records the person's answer (value: an option's key) on the Ask
// seen, if it still says what they saw. The line is then read. What it
// says ("答え a を付けました: …") the terminal leaves out: the line shows it.
func (b *Board) React(seen Seen, value string) (string, error) {
	if !b.CanWrite() {
		return "", errCannotWrite
	}
	n := b.node(seen.Key)
	if n == nil || report.Target(n) == "" {
		return "", ErrChanged
	}
	// Read the report again: the reaction is recorded only if the line is
	// still what the person is looking at.
	fresh, err := b.Report.fresh()
	var notRead *readError
	if errors.As(err, &notRead) {
		return "", fmt.Errorf("記録できませんでした: %w", notRead.err)
	}
	now := report.ByID(fresh, report.Target(n))
	if err != nil || now == nil || now.Sig() != seen.Sig || n.Sig() != seen.Sig {
		return "", ErrChanged
	}
	choices, _ := KeysFor(now)
	if !slices.Contains(choices, value) {
		return "", fmt.Errorf("この行には %s で答えられません", value)
	}
	e := board.Event{Op: "react", ID: report.Target(now), Value: value, Line: now.Sig(), Title: now.Title()}
	if ch := now.Choice(value); ch != nil {
		e.Option = report.PlainOf(ch.Text)
	}
	if e.Option == "" {
		e.Option = value // never empty: an answer is not an older viewer's y / n (board.Event.Legacy)
	}
	rec, err := b.Store.Append(e)
	what := "答え " + value + " "
	if err == nil {
		b.did(rec, what+"を取り消しました: "+report.PlainOf(now.Title()))
	}
	b.MarkViewed(n)
	b.Load()
	if err != nil {
		return "", fmt.Errorf("記録できませんでした: %w", err)
	}
	return what + "を付けました: " + report.PlainOf(now.Title()) + " (u で戻す)", nil
}

// Hide hides the line seen, or shows it again when it is hidden, if it
// still says what the person saw. The report is not changed: the record
// says what the line says, and screens leave out lines that say that.
// hidden reports whether the line is hidden now; showHidden whether the
// screen shows hidden lines (what is said differs).
func (b *Board) Hide(seen Seen, showHidden bool) (msg string, hidden bool, err error) {
	if !b.CanWrite() {
		return "", false, errCannotWrite
	}
	n := b.node(seen.Key)
	if n == nil || report.HideKey(n) == "" {
		return "", false, ErrChanged
	}
	fresh, err := b.Report.fresh()
	var notRead *readError
	if errors.As(err, &notRead) {
		return "", false, fmt.Errorf("記録できませんでした: %w", notRead.err)
	}
	var now *report.Node
	report.WalkAll(fresh, func(x *report.Node) {
		if now == nil && x.Key == seen.Key {
			now = x
		}
	})
	if err != nil || now == nil || now.Sig() != seen.Sig || report.HideKey(now) != seen.HideKey || n.Sig() != seen.Sig {
		return "", false, ErrChanged
	}
	op, msg := "hide", "隠しました"
	if b.IsHidden(n) {
		op, msg = "unhide", "表示に戻しました"
	}
	b.MarkViewed(n)
	rec, err := b.Store.Append(board.Event{Op: op, Key: report.HideKey(n), Title: n.Title()})
	if err != nil {
		return "", false, fmt.Errorf("記録できませんでした: %w", err)
	}
	undoMsg := "隠したのを取り消しました: "
	if op == "unhide" {
		undoMsg = "表示に戻したのを取り消しました: "
	}
	b.did(rec, undoMsg+report.PlainOf(n.Title()))
	b.Load()
	if op == "hide" && !showHidden {
		msg += " (H で表示、u で戻す)"
	} else {
		msg += " (u で戻す)"
	}
	return msg, op == "hide", nil
}

// Comment records a comment: on the line target (title: what the line said
// when the person began writing), or on no line (target ""). Empty text
// records nothing (MsgEmptyComment). A comment on a line makes the line
// read. The terminal leaves out what it says: the comment shows below.
func (b *Board) Comment(target, title, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return MsgEmptyComment, nil
	}
	if !b.CanWrite() {
		return "", errCannotWrite
	}
	e := board.Event{Op: "thought", ID: target, Text: text, Title: title}
	rec, err := b.Store.Append(e)
	if err != nil {
		return "", fmt.Errorf("記録できませんでした: %w", err)
	}
	b.did(rec, "コメントを取り消しました: "+TextHead(text))
	if target != "" {
		if n := report.ByID(b.Report.Nodes, target); n != nil && n.Title() == title {
			if n.Kind == report.KindDetails {
				// A Fold's mark is what it holds too: it goes, as with Read.
				b.MarkViewed(b.UnreadIn(n)...)
			} else {
				b.MarkViewed(n)
			}
		}
	}
	b.Load()
	return MsgCommented, nil
}

// Edit rewrites comment n. The same text records nothing; a comment that
// is not there is ErrNoComment. The terminal leaves out what it says.
func (b *Board) Edit(n int, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return MsgEmptyComment, nil // to take a comment back, Retract
	}
	if !b.CanWrite() {
		return "", errCannotWrite
	}
	t := b.Thought(n)
	if t == nil {
		return "", ErrNoComment
	}
	if strings.TrimSpace(t.Text) == text {
		return MsgEdited, nil
	}
	rec, err := b.Store.Append(board.Event{Op: "edit", Thought: n, Text: text})
	if err != nil {
		return "", fmt.Errorf("記録できませんでした: %w", err)
	}
	b.did(rec, "コメントの直しを取り消しました: "+TextHead(text))
	b.Load()
	return MsgEdited, nil
}

// Retract takes comment n back (one that is not there: ErrNoComment).
func (b *Board) Retract(n int) (string, error) {
	if !b.CanWrite() {
		return "", errCannotWrite
	}
	t := b.Thought(n)
	if t == nil {
		return "", ErrNoComment
	}
	head := TextHead(t.Text)
	rec, err := b.Store.Append(board.Event{Op: "retract", Thought: n})
	if err != nil {
		return "", fmt.Errorf("取り消せませんでした: %w", err)
	}
	b.did(rec, "取り消したコメントを戻しました: "+head)
	b.Load()
	return MsgRetracted, nil
}

// Read says the line key is read: its MarkNew goes (for a Fold, of what it
// holds too). how is what the screen calls the act, for what Undo says
// ("r (読んだ)"). The terminal says what it says only on a line it just
// moved the cursor to (elsewhere the mark going says it).
func (b *Board) Read(key, how string) (string, error) {
	n := b.node(key)
	if n == nil || !b.CanWrite() || b.MarkOf(n) != MarkNew {
		return "", nil
	}
	if hs := b.MarkViewed(b.UnreadIn(n)...); len(hs) > 0 {
		b.Hist = append(b.Hist, Done{Viewed: hs, Desc: how + " を取り消しました: " + report.PlainOf(n.Title())})
	}
	return "読んだことにしました: " + report.PlainOf(n.Title()) + " (u で戻す)", nil
}

// ReadAll says every line now in the report is read. how is what the
// screen calls the act, for what Undo says.
func (b *Board) ReadAll(how string) (string, error) {
	if !b.CanWrite() {
		return "", nil
	}
	var ns []*report.Node
	report.WalkAll(b.Report.Nodes, func(n *report.Node) { ns = append(ns, n) })
	if hs := b.MarkViewed(ns...); len(hs) > 0 {
		b.Hist = append(b.Hist, Done{Viewed: hs, Desc: how + " を取り消しました"})
	}
	return "いまある行をすべて読んだことにしました", nil
}

// Undo takes back the newest thing the person did since the screen
// opened. The record only grows: an undo is recorded, never a line
// removed. It returns what was taken back.
func (b *Board) Undo() (string, error) {
	if !b.CanWrite() || len(b.Hist) == 0 {
		return "", ErrNothingToUndo
	}
	h := b.Hist[len(b.Hist)-1]
	if h.Viewed != nil {
		if err := b.Store.RemoveViewed(h.Viewed); err != nil {
			return "", fmt.Errorf("記録できませんでした: %w", err)
		}
		b.LoadViewed()
	} else if _, err := b.Store.Append(board.Event{Op: "undo", Undo: h.N}); err != nil {
		return "", fmt.Errorf("記録できませんでした: %w", err)
	}
	b.Hist = b.Hist[:len(b.Hist)-1]
	b.Load()
	return h.Desc, nil
}

// TextHead is the start of a comment, on one line, for messages.
func TextHead(s string) string { return board.CutTitle(s, 30) }
