package board

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestChangesAreCompact(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="hook">`+strings.Repeat("長い題", 20)+`</Item>
<Ask id="unit">問い
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>`), 0o644)
	long := strings.Repeat("長い題", 20)
	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: long})
	st.Append(Event{Op: "react", ID: "unit", Value: "a", Title: "問い", Option: "A"})
	st.Append(Event{Op: "hide", Key: "1", Title: "一つ目の行"})
	st.Append(Event{Op: "react", ID: "hook", Value: "n", Title: long})
	st.Append(Event{Op: "thought", ID: "hook", Title: long, Text: "これ"})
	st.Append(Event{Op: "react", ID: "unit", Value: "b", Title: "問い", Option: "B"})
	st.Append(Event{Op: "hide", Key: "2", Title: strings.Repeat("と", 30)})
	st.Append(Event{Op: "unhide", Key: "1", Title: "一つ目の行"})
	sendAll(st)
	lines, _ := st.Changes()
	cut30 := string([]rune(long)[:29]) + "…"
	want := []string{
		"n hook (" + long + ")",
		"thought 5 on hook (" + cut30 + "): これ",
		"answer unit = b (B)",
		"hide (2): 一つ目の行 / " + strings.Repeat("と", 19) + "…",
		"unhide (1): 一つ目の行",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestChangesInEnglish(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "gone", Value: "y", Title: "消えた行"})
	st.Append(Event{Op: "react", ID: "hook", Value: "n", Title: "前の題"})
	sendAll(st)
	lines, _ := st.Changes()
	want := "y gone (消えた行 → now gone)\nn hook (前の題 → now: hook を入れるか決める)"
	if strings.Join(lines, "\n") != want {
		t.Errorf("got\n%s", strings.Join(lines, "\n"))
	}
}

// ---- taking a thought back ----

func TestRetractBeforeDelivery(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "keep"})
	th, _ := st.Append(Event{Op: "thought", Text: "mistake"})
	st.Append(Event{Op: "retract", Thought: th.N})
	if n := Unsent(st.Events()); n != 1 {
		t.Errorf("unsent = %d, want 1 (the taken-back thought is not counted)", n)
	}
	lines, _ := sendChanges(st)
	if strings.Join(lines, "\n") != "thought 1: keep" {
		t.Errorf("changes: %q", lines)
	}
	if s := Replay(st.Events(), st.Seen()); len(s.Thoughts) != 1 || s.Thoughts[0].Text != "keep" {
		t.Errorf("the board must drop it: %+v", s.Thoughts)
	}
}

func TestRetractAfterDelivery(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", ID: "hook", Text: "とり下げる思いつき\n二行目"})
	sendChanges(st)
	st.Settle(th.N, "todo にした") // given a destination, then taken back
	st.Append(Event{Op: "retract", Thought: th.N})
	if n := Unsent(st.Events()); n != 1 {
		t.Errorf("unsent = %d", n)
	}
	lines, _ := sendChanges(st)
	if strings.Join(lines, "\n") != "retract thought 1: とり下げる思いつき 二行目" {
		t.Errorf("changes: %q", lines)
	}
	// settle on a taken-back thought is accepted silently.
	if err := st.Settle(th.N, "見送った"); err != nil {
		t.Errorf("settle after retract: %v", err)
	}
	// Taken back and undone before changes: nothing to tell.
	th2, _ := st.Append(Event{Op: "thought", Text: "two"})
	sendChanges(st)
	st.Append(Event{Op: "retract", Thought: th2.N})
	st.Append(Event{Op: "unretract", Thought: th2.N})
	if n := Unsent(st.Events()); n != 0 {
		t.Errorf("unsent after undo = %d", n)
	}
	if lines, _ := sendChanges(st); len(lines) != 0 {
		t.Errorf("changes after undo: %q", lines)
	}
	// A retract delivered, then undone: the AI is told it stands again.
	st.Append(Event{Op: "retract", Thought: th2.N})
	sendChanges(st)
	st.Append(Event{Op: "unretract", Thought: th2.N})
	if lines, _ := sendChanges(st); strings.Join(lines, "\n") != fmt.Sprintf("unretract thought %d: two", th2.N) {
		t.Errorf("changes after a delivered retract was undone: %q", lines)
	}
	// Undone before delivery: the thought goes as usual.
	th3, _ := st.Append(Event{Op: "thought", Text: "three"})
	st.Append(Event{Op: "retract", Thought: th3.N})
	st.Append(Event{Op: "unretract", Thought: th3.N})
	if lines, _ := sendChanges(st); strings.Join(lines, "\n") != fmt.Sprintf("thought %d: three", th3.N) {
		t.Errorf("changes: %q", lines)
	}
}

// A thought sent but not read yet, then taken back: the retract goes with
// the next send.
func TestRetractAfterSendBeforeRead(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "sent"})
	sendAll(st)
	st.Append(Event{Op: "retract", Thought: th.N})
	if n := Unsent(st.Events()); n != 1 {
		t.Errorf("unsent = %d, want 1 (the retract)", n)
	}
	// Read now: only what was sent, the thought.
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "thought 1: sent" {
		t.Errorf("changes: %q", lines)
	}
	if lines, _ := sendChanges(st); strings.Join(lines, "\n") != "retract thought 1: sent" {
		t.Errorf("changes after the next send: %q", lines)
	}
}

// ---- only what was sent is delivered ----

func TestChangesStopsAtTheLatestSend(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "thought", Text: "one"})
	if lines, _ := st.Changes(); len(lines) != 0 {
		t.Errorf("nothing sent, yet changes gave %q", lines)
	}
	if n := Unsent(st.Events()); n != 2 {
		t.Errorf("unsent = %d", n)
	}
	sendAll(st) // 3
	st.Append(Event{Op: "thought", Text: "after the send"})
	if n, m := Unsent(st.Events()), SentUnread(st.Events(), st.Seen()); n != 1 || m != 2 {
		t.Errorf("unsent %d (want 1), sent unread %d (want 2)", n, m)
	}
	lines, _ := st.Changes()
	if strings.Join(lines, "\n") != "y hook (hook を入れるか決める)\nthought 2: one" {
		t.Errorf("first send: %q", lines)
	}
	if st.Seen() != 3 {
		t.Errorf("seen = %d, want the send (3)", st.Seen())
	}
	// The thought after the send waits for the next one.
	if lines, _ := st.Changes(); len(lines) != 0 {
		t.Errorf("changes before the second send: %q", lines)
	}
	s := Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[1]); w != StatusUnsent {
		t.Errorf("status before the second send: %d", w)
	}
	sendAll(st)
	s = Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[1]); w != StatusSent {
		t.Errorf("status after the second send: %d", w)
	}
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "thought 4: after the send" {
		t.Errorf("second send: %q", lines)
	}
	s = Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[1]); w != StatusRead {
		t.Errorf("status after changes: %d", w)
	}
}

// ---- rewriting thoughts ----

func TestEditBeforeDelivery(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "書きか"})
	st.Append(Event{Op: "edit", Thought: th.N, Text: "書きかけを直した"})
	if n := Unsent(st.Events()); n != 1 {
		t.Errorf("unsent %d", n)
	}
	sendAll(st)
	lines, _ := st.Changes()
	if strings.Join(lines, "|") != "thought 1: 書きかけを直した" {
		t.Errorf("changes %q", lines)
	}
	// Sent but not read, then edited: still told as the thought.
	th2, _ := st.Append(Event{Op: "thought", Text: "b"})
	sendAll(st)
	st.Append(Event{Op: "edit", Thought: th2.N, Text: "b2"})
	sendAll(st)
	lines, _ = st.Changes()
	if strings.Join(lines, "|") != "thought 4: b2" {
		t.Errorf("changes %q", lines)
	}
	if _, err := st.Append(Event{Op: "edit", Thought: 99, Text: "x"}); err == nil {
		t.Error("edit of no thought recorded")
	}
}

func TestEditAfterDeliveryAndSettle(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "one"})
	sendAll(st)
	st.Changes()
	st.Settle(th.N, "todo にした")
	s := Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[0]); w != StatusDone {
		t.Fatalf("status %d", w)
	}
	st.Append(Event{Op: "edit", Thought: th.N, Text: "one, again"})
	st.Append(Event{Op: "edit", Thought: th.N, Text: "one, again\nand more"})
	evs := st.Events()
	if n := Unsent(evs); n != 1 {
		t.Errorf("unsent %d (one per thought)", n)
	}
	s = Replay(evs, st.Seen())
	if w := s.StatusOf(s.Thoughts[0]); w != StatusUnsent || s.Thoughts[0].Text != "one, again\nand more" {
		t.Errorf("status %d text %q", w, s.Thoughts[0].Text)
	}
	sendAll(st)
	lines, _ := st.Changes()
	if strings.Join(lines, "|") != "edit thought 1: one, again\n  and more" {
		t.Errorf("changes %q", lines)
	}
	s = Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[0]); w != StatusRead {
		t.Errorf("status after delivery %d (the destination was for the old text)", w)
	}
}

// 5: taking back a rewritten comment tells the text the AI last got.
func TestRetractTellsTheLatestText(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "ひとこと"})
	changes(t, st)
	st.Append(Event{Op: "edit", Thought: th.N, Text: "直した"})
	changes(t, st)
	st.Append(Event{Op: "retract", Thought: th.N})
	if got := changes(t, st); got != fmt.Sprintf("retract thought %d: 直した", th.N) {
		t.Errorf("changes %q", got)
	}
	// Rewritten but not sent, then taken back: the AI last got the first.
	th2, _ := st.Append(Event{Op: "thought", Text: "最初"})
	changes(t, st)
	st.Append(Event{Op: "edit", Thought: th2.N, Text: "送っていない"})
	st.Append(Event{Op: "retract", Thought: th2.N})
	if got := changes(t, st); got != fmt.Sprintf("retract thought %d: 最初", th2.N) {
		t.Errorf("changes %q", got)
	}
}

func TestChangesAndSettle(t *testing.T) {
	st := testStore(t)
	mustAppend := func(e Event) Event {
		t.Helper()
		e, err := st.Append(e)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	mustAppend(Event{Op: "react", ID: "hook", Value: "y"})
	mustAppend(Event{Op: "react", ID: "unit", Value: "b"})
	t1 := mustAppend(Event{Op: "thought", ID: "hook", Text: "post-tool だけ入れたい"})
	t2 := mustAppend(Event{Op: "thought", Text: "経過は要らないかも\n2 行目"})
	mustAppend(Event{Op: "react", ID: "gone", Value: "n"})
	sendAll(st)

	got, err := st.Changes()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"y hook (hook を入れるか決める)",
		"answer unit = b (プロジェクトごと)",
		"thought 3 on hook (hook を入れるか決める): post-tool だけ入れたい",
		"thought 4: 経過は要らないかも\n  2 行目",
		"n gone (gone)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if again, _ := st.Changes(); len(again) != 0 {
		t.Errorf("second changes = %q, want nothing", again)
	}

	// Thought states: received, then settled.
	if n := SentUnread(st.Events(), st.Seen()); n != 0 {
		t.Errorf("unread = %d after changes", n)
	}
	if err := st.Settle(t1.N, "todo にした"); err != nil {
		t.Fatal(err)
	}
	if err := st.Settle(99, "x"); err == nil {
		t.Error("settling a thought that does not exist must fail")
	}
	if err := st.Settle(1, "x"); err == nil {
		t.Error("settling a reaction (not a thought) must fail")
	}
	s := Replay(st.Events(), st.Seen())
	if w := s.StatusOf(s.Thoughts[0]); w != StatusDone {
		t.Errorf("settled thought status = %d", w)
	}
	if w := s.StatusOf(s.Thoughts[1]); w != StatusRead {
		t.Errorf("received thought status = %d", w)
	}
	// A settle is the AI's own; changes does not repeat it.
	if again, _ := st.Changes(); len(again) != 0 {
		t.Errorf("changes after settle = %q", again)
	}
	// A new thought is not sent until the person sends it.
	mustAppend(Event{Op: "thought", Text: "new"})
	if n := Unsent(st.Events()); n != 1 {
		t.Errorf("unsent = %d", n)
	}
	_ = t2
}

func TestChangesTellsTheTitleSeen(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "react", ID: "unit", Value: "a", Title: "ページの単位", Option: "セッションごと"})
	st.Append(Event{Op: "thought", ID: "hook", Text: "x", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "react", ID: "old", Value: "n", Title: "古い行"})
	os.WriteFile(st.ReportPath(), []byte(`<Ask id="unit">板の単位
<Option key="a">セッションごと</Option>
<Option key="b">プロジェクトごと</Option>
</Ask>
<Item id="hook">hook は入れない (決定)</Item>`), 0o644)
	sendAll(st)
	got, _ := st.Changes()
	want := []string{
		"y hook (hook を入れるか決める → now: hook は入れない (決定))",
		"answer unit = a (セッションごと; question: ページの単位 → now: 板の単位)",
		"thought 3 on hook (hook を入れるか決める → now: hook は入れない (決定)): x",
		"n old (古い行 → now gone)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
