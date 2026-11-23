package main

import (
	"os"
	"strings"
	"testing"
)

// ---- only what was sent is delivered ----

func TestChangesStopsAtTheLatestSend(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "thought", Text: "one"})
	if lines, _ := st.Changes(); len(lines) != 0 {
		t.Errorf("nothing sent, yet changes gave %q", lines)
	}
	if n := unsent(st.Events()); n != 2 {
		t.Errorf("unsent = %d", n)
	}
	sendAll(st) // 3
	st.Append(Event{Op: "thought", Text: "after the send"})
	if n, m := unsent(st.Events()), sentUnread(st.Events(), st.Seen()); n != 1 || m != 2 {
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
	s := replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[1]); w != "未送信" {
		t.Errorf("status before the second send: %q", w)
	}
	sendAll(st)
	s = replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[1]); w != "送信済" {
		t.Errorf("status after the second send: %q", w)
	}
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "thought 4: after the send" {
		t.Errorf("second send: %q", lines)
	}
	s = replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[1]); w != "既読" {
		t.Errorf("status after changes: %q", w)
	}
}

func TestSendRecordsOnlyWhenThereIsSomething(t *testing.T) {
	noSendTarget(t)
	t.Setenv("PITBOARD_SEND", "true")
	st := testStore(t)
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if len(st.Events()) != 0 || a.msg != "未送信のものはありません" {
		t.Errorf("s with nothing: %d events, %q", len(st.Events()), a.msg)
	}
	st.Append(Event{Op: "thought", Text: "a"})
	a.handle(key{r: 's'})
	a.handle(key{r: 's'})
	n := 0
	for _, e := range st.Events() {
		if e.Op == "send" {
			n++
		}
	}
	if n != 1 || a.msg != "未送信のものはありません" {
		t.Errorf("sends %d, msg %q", n, a.msg)
	}
	// A thought taken back before it was sent: nothing to send.
	th, _ := st.Append(Event{Op: "thought", Text: "b"})
	st.Append(Event{Op: "retract", Thought: th.N})
	a.handle(key{r: 's'})
	if a.msg != "未送信のものはありません" {
		t.Errorf("only a taken-back thought: %q", a.msg)
	}
}

// ---- keys that work with an input method on ----

func TestKeysWithInputMethod(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'ｊ'}) // full-width j
	a.render()
	if pv.cur != "hook" || pv.placed {
		t.Fatalf("ｊ must move like j: cur %q placed %v", pv.cur, pv.placed)
	}
	a.handle(key{r: 'ｙ'})
	if evs := st.Events(); len(evs) != 1 || evs[0].Value != "y" {
		t.Errorf("ｙ must react like y: %+v", evs)
	}
	a.handle(key{name: "ctrl+p"})
	a.render()
	if pv.cur != "unit" {
		t.Errorf("ctrl+p must move up: %q", pv.cur)
	}
	a.handle(key{name: "ctrl+n"})
	if pv.cur != "hook" {
		t.Errorf("ctrl+n must move down: %q", pv.cur)
	}
	// In the editor, full-width letters stay letters.
	a.handle(key{r: 'ｍ'})
	if a.in == nil {
		t.Fatal("ｍ must open the editor")
	}
	a.handle(key{r: 'ｊ'})
	a.handle(key{r: '　'})
	if got := a.in.ed.String(); got != "ｊ　" {
		t.Errorf("editor text %q", got)
	}
}

// ---- opened contents are in the normal color ----

func TestOpenedContentsColor(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="a" status="待ち" note="上の答え待ち">項目
中身の文
</Item>
<Item id="d" state="done">済んだ項目
済んだ中身
</Item>`), 0o644)
	th, _ := st.Append(Event{Op: "thought", ID: "a", Text: "思いつき"})
	st.Settle(th.N, "todo にした")
	a := testApp(t, st)
	pv := a.top()
	pv.open["a"], pv.open["d"] = true, true
	var raw []string
	for _, r := range a.rows(pv) {
		raw = append(raw, r.line)
	}
	s := strings.Join(raw, "\n")
	for _, c := range []struct{ want, not string }{
		{"中身の文", dim + "中身の文"},
		{dim + "理由: ", magenta + "上の答え待ち"},
		{"上の答え待ち", dim + "上の答え待ち"},
		{dim + "対応: ", dim + "対応: todo"},
		{dim + "済んだ中身", ""},
	} {
		if !strings.Contains(s, c.want) || (c.not != "" && strings.Contains(s, c.not)) {
			t.Errorf("want %q, not %q in:\n%q", c.want, c.not, s)
		}
	}
}
