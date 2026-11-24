package page

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
)

func open(t *testing.T, src string) *Board {
	t.Helper()
	st := board.Open(t.TempDir(), "s1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, st, src)
	return Open(st)
}

func write(t *testing.T, st *board.Store, src string) {
	t.Helper()
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

const src = `<Notice id="re" at="2026-11-23 21:46">開き直してください</Notice>
<Title>題</Title>

<Section title="あなたへ">
  <Ask id="unit">ページの単位
    <Option key="a">セッションごと</Option>
    <Option key="b">プロジェクトごと</Option>
  </Ask>
  <Item id="hook">hook を **入れる** か決める
    中身の ` + "`code`" + `
  </Item>
  <Item id="impl" state="doing" look="y">実装する</Item>
  <Item id="w" wait="you" status="あなた待ち">待つ</Item>
  - ただの行
</Section>

<Fold title="決まったこと">
  <Item id="old" state="done" look="y">古い</Item>
</Fold>
`

func find(ts []*Thing, key string) *Thing {
	for _, t := range ts {
		if t.Key == key {
			return t
		}
		for _, l := range [][]*Thing{t.Kids, t.Body, t.Subs} {
			if f := find(l, key); f != nil {
				return f
			}
		}
	}
	return nil
}

// The page says, for each line, what it is, what can be done with it, its
// marks and the words at its right end.
func TestPage(t *testing.T) {
	b := open(t, src)
	p := b.Page(false)
	if len(p.Notices) != 1 || p.Notices[0].At.IsZero() || len(p.Things) != 3 {
		t.Fatalf("notices %d things %d", len(p.Notices), len(p.Things))
	}
	if p.Waiting != 1 || p.Unread != 2 || !p.CanWrite || p.CanSend || p.CanUndo {
		t.Errorf("counts: %+v", p)
	}
	ask := find(p.Things, "unit")
	if ask.Kind != KindAsk || ask.Mark != MarkWaiting || len(ask.Kids) != 2 || ask.Kids[0].OptionKey != "a" || strings.Join(ask.Actions.Options, "") != "ab" {
		t.Errorf("ask %+v", ask)
	}
	hook := find(p.Things, "hook")
	if hook.Kind != KindItem || !hook.CanOpen || len(hook.Actions.Options) != 0 || hook.Mark != MarkNone || len(hook.Body) != 1 {
		t.Errorf("hook %+v", hook)
	}
	if len(hook.Title) != 3 || hook.Title[1] != (Span{Text: "入れる", Style: StyleBold}) {
		t.Errorf("title spans %+v", hook.Title)
	}
	impl := find(p.Things, "impl")
	if impl.Right.Status != (Word{WorkingWord, ToneMove}) || impl.Mark != MarkNew || impl.CanOpen || impl.State != "doing" {
		t.Errorf("impl %+v", impl)
	}
	w := find(p.Things, "w")
	if !w.You || w.Right.Status != (Word{"あなた待ち", ToneNeed}) {
		t.Errorf("w %+v", w)
	}
	plain := find(p.Things, "-1:ただの行")
	if plain == nil || plain.Kind != KindList || !plain.Stop || !plain.Actions.Hide || plain.Actions.Comment {
		t.Errorf("plain %+v", plain)
	}
	fold := p.Things[2]
	if fold.Kind != KindFold || fold.Mark != MarkNew || fold.Target != "Fold|title=決まったこと" {
		t.Errorf("fold %+v", fold)
	}
	if len(p.Jumps) != 3 || p.Jumps[0].Kind != KindNotice || p.Jumps[2] != fold {
		t.Errorf("jumps %d", len(p.Jumps))
	}
}

// Actions carry the line as the person saw it: when it says something
// else now, nothing is recorded.
func TestReactOnlyOnWhatWasSeen(t *testing.T) {
	b := open(t, src)
	hook := find(b.Page(false).Things, "hook")
	// An Item takes no answer: y / n were removed (FEAT25).
	for _, k := range []string{"x", "y", "n"} {
		if _, err := b.React(hook.Seen, k); err == nil {
			t.Errorf("%s is no answer to an Item", k)
		}
	}
	ask := find(b.Page(false).Things, "unit")
	write(t, b.Store, strings.Replace(src, "ページの単位", "板の単位", 1))
	if _, err := b.React(ask.Seen, "a"); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed line: %v", err)
	}
	write(t, b.Store, src)
	b.Report.Load()
	p := b.Page(false)
	ask = find(p.Things, "unit")
	if _, err := b.React(ask.Seen, "a"); err != nil {
		t.Fatal(err)
	}
	p = b.Page(false)
	ask = find(p.Things, "unit")
	if ask.Right.Reaction != "a" || ask.Right.Input != (Word{"未送信", ToneNeed}) || ask.Mark != MarkNone || !p.CanUndo || !p.CanSend {
		t.Errorf("after a: %+v", ask.Right)
	}
	if _, err := b.React(ask.Seen, "b"); err != nil {
		t.Fatal(err)
	}
	ask = find(b.Page(false).Things, "unit")
	if ask.Right.Option != "プロジェクトごと" || !ask.Kids[1].Chosen || ask.Kids[0].Chosen {
		t.Errorf("answer: %+v", ask.Right)
	}
	if msg, err := b.Undo(); err != nil || msg != "答え b を取り消しました: ページの単位" {
		t.Errorf("undo: %q %v", msg, err)
	}
}

// Hiding leaves the line out of the page (or marks it Hidden with
// showHidden), and Undo brings it back.
func TestHideAndUndo(t *testing.T) {
	b := open(t, src)
	impl := find(b.Page(false).Things, "impl")
	msg, hidden, err := b.Hide(impl.Seen, false)
	if err != nil || !hidden || msg != "隠しました (H で表示、u で戻す)" {
		t.Fatalf("hide: %q %v %v", msg, hidden, err)
	}
	if find(b.Page(false).Things, "impl") != nil {
		t.Error("a hidden line is on the page")
	}
	p := b.Page(true)
	if h := find(p.Things, "impl"); h == nil || !h.Hidden || h.Mark != MarkNone || p.Hidden != 1 {
		t.Errorf("showHidden: %+v", h)
	}
	if msg, hidden, err := b.Hide(find(p.Things, "impl").Seen, true); err != nil || hidden || msg != "表示に戻しました (u で戻す)" {
		t.Errorf("unhide: %q %v %v", msg, hidden, err)
	}
	b.Undo()
	b.Undo()
	if find(b.Page(false).Things, "impl") == nil {
		t.Error("undo did not bring it back")
	}
}

// Comments: on a line (it is then read), edited, retracted, and sent.
func TestCommentsAndSend(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	b := open(t, src)
	impl := find(b.Page(false).Things, "impl")
	if _, err := b.Comment(impl.Target, "実装する", "まだ？ [Image #2]"); err != nil {
		t.Fatal(err)
	}
	p := b.Page(false)
	impl = find(p.Things, "impl")
	if len(impl.Comments) != 1 || impl.Mark != MarkNone || !impl.CanOpen || impl.Right.Input.Text != "未送信" {
		t.Fatalf("impl %+v", impl)
	}
	c := impl.Comments[0]
	if c.About != "実装する" || c.Note != "まだ送っていません (s で送る)" || len(c.Images) != 1 || c.Images[0] != 2 || len(p.Comments) != 1 {
		t.Errorf("comment %+v", c)
	}
	if _, err := b.Edit(c.N, "まだですか"); err != nil {
		t.Fatal(err)
	}
	if msg, err := b.Send(nil); err != nil || msg != "送りました" {
		t.Errorf("send: %q %v", msg, err)
	}
	if msg, _ := b.Send(nil); msg != "未送信のものはありません" {
		t.Errorf("send again: %q", msg)
	}
	if c := b.Page(false).Comments[0]; c.Text != "まだですか" || c.Word.Text != "送信済" {
		t.Errorf("sent comment %+v", c)
	}
	if msg, err := b.Retract(c.N); err != nil || msg != "取り消しました (u で戻す)" || len(b.Page(false).Comments) != 0 {
		t.Errorf("retract: %q %v", msg, err)
	}
}

// Read and ReadAll clear the unread marks; Undo puts them back.
func TestRead(t *testing.T) {
	b := open(t, src)
	b.Read("fold:1", "r (読んだ)")
	if p := b.Page(false); p.Things[2].Mark != MarkNone || p.Unread != 1 {
		t.Errorf("after read: %d", p.Unread)
	}
	if msg, _ := b.Undo(); msg != "r (読んだ) を取り消しました: 決まったこと" || b.Page(false).Unread != 2 {
		t.Errorf("undo read: %q", msg)
	}
	b.ReadAll("A")
	if p := b.Page(false); p.Unread != 0 || p.Waiting != 1 {
		t.Errorf("after read all: %+v", p)
	}
}

// Without a session or a report, the page says why there is nothing.
func TestPageMessages(t *testing.T) {
	if p := Open(nil).Page(false); p.Message != MsgNoSession || p.CanWrite {
		t.Errorf("no session: %+v", p)
	}
	st := board.Open(t.TempDir(), "s9")
	if p := Open(st).Page(false); p.Message != MsgNoSuchSession("s9") {
		t.Errorf("no such session: %q", p.Message)
	}
	os.MkdirAll(st.Dir, 0o755)
	if p := Open(st).Page(false); p.Message != MsgNoReport {
		t.Errorf("no report: %q", p.Message)
	}
	write(t, st, "")
	if p := Open(st).Page(false); p.Message != MsgEmpty {
		t.Errorf("empty: %q", p.Message)
	}
	write(t, st, "<Item id=\"a\">x</Item>\n<Bogus>\n")
	if p := Open(st).Page(false); len(p.Problems) == 0 || !p.Partial {
		t.Errorf("problems: %+v", p.Problems)
	}
}

func TestEmptySectionNone(t *testing.T) {
	b := open(t, `<Section title="空"></Section>
<Section title="ある">
  <Item id="a">一</Item>
</Section>
`)
	p := b.Page(false)
	if len(p.Things) != 2 {
		t.Fatalf("things: %d", len(p.Things))
	}
	if got := p.Things[0].None; got != MsgNone {
		t.Errorf("empty Section: None %q, want %q", got, MsgNone)
	}
	if got := p.Things[1].None; got != "" {
		t.Errorf("Section with an Item: None %q", got)
	}
}
