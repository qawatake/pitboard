package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sendAll marks everything so far as sent (what s does on the board).
func sendAll(st *Store) {
	st.Append(Event{Op: "send"})
}

func testStore(t *testing.T) *Store {
	t.Helper()
	st := newStore(t.TempDir(), "s1")
	if err := os.MkdirAll(st.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	report := `<Section title="あなたへ">
<Ask id="unit">ページの単位
<Option key="a">セッションごと</Option>
<Option key="b">プロジェクトごと</Option>
</Ask>
<Item id="hook">hook を入れるか決める</Item>
</Section>`
	if err := os.WriteFile(st.reportPath(), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestAppendNumbers(t *testing.T) {
	st := testStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Append(Event{Op: "thought", Text: "x"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	evs := st.Events()
	if len(evs) != 20 {
		t.Fatalf("events = %d", len(evs))
	}
	for i, e := range evs {
		if e.N != i+1 {
			t.Fatalf("event %d has n=%d", i, e.N)
		}
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
	if n := sentUnread(st.Events(), st.Seen()); n != 0 {
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
	s := replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[0]); w != "対応済" {
		t.Errorf("settled thought status = %q", w)
	}
	if w, _ := s.statusOf(s.thoughts[1]); w != "既読" {
		t.Errorf("received thought status = %q", w)
	}
	// A settle is the AI's own; changes does not repeat it.
	if again, _ := st.Changes(); len(again) != 0 {
		t.Errorf("changes after settle = %q", again)
	}
	// A new thought is not sent until the person sends it.
	mustAppend(Event{Op: "thought", Text: "new"})
	if n := unsent(st.Events()); n != 1 {
		t.Errorf("unsent = %d", n)
	}
	_ = t2
}

func TestLatestReactionWins(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y"})
	st.Append(Event{Op: "react", ID: "hook", Value: "n"})
	s := replayEvents(st.Events(), 0)
	if s.react["hook"].Value != "n" {
		t.Errorf("reaction = %q", s.react["hook"].Value)
	}
	if len(st.Events()) != 2 {
		t.Error("both reactions must stay in the record")
	}
}

func TestSessions(t *testing.T) {
	base := t.TempDir()
	for _, s := range []string{"abc123", "abd456"} {
		os.MkdirAll(filepath.Join(base, ".pitboard", s), 0o755)
	}
	if s, err := resolveSession(base, "abc"); err != nil || s != "abc123" {
		t.Errorf("prefix: %q %v", s, err)
	}
	if _, err := resolveSession(base, "ab"); err == nil {
		t.Error("an ambiguous prefix must fail")
	}
	if s, err := resolveSession(base, "new-one"); err != nil || s != "new-one" {
		t.Errorf("new: %q %v", s, err)
	}
	if _, err := resolveSession(base, "../x"); err == nil {
		t.Error("a bad name must fail")
	}
}

func TestCheckCommand(t *testing.T) {
	st := testStore(t)
	t.Chdir(st.base)
	var out strings.Builder
	if err := run([]string{"-s", "s1", "check"}, &out); err != nil {
		t.Fatalf("good report: %v %s", err, out.String())
	}
	os.WriteFile(st.reportPath(), []byte("<Item id=\"a\">x</Item>\n<Item id=\"a\">y</Item>\n"), 0o644)
	out.Reset()
	err := run([]string{"-s", "s1", "check"}, &out)
	if err != exitError(1) || !strings.Contains(out.String(), ":2: duplicate id") {
		t.Errorf("bad report: %v %q", err, out.String())
	}
}

func TestChangesTellsTheTitleSeen(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "react", ID: "unit", Value: "a", Title: "ページの単位", Option: "セッションごと"})
	st.Append(Event{Op: "thought", ID: "hook", Text: "x", Title: "hook を入れるか決める"})
	st.Append(Event{Op: "react", ID: "old", Value: "n", Title: "古い行"})
	os.WriteFile(st.reportPath(), []byte(`<Ask id="unit">板の単位
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

func TestSessionFlagAfterCommand(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	var out strings.Builder
	if err := run([]string{"path", "-s", "abc"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.String()), filepath.Join(".pitboard", "abc", "report.md")) {
		t.Errorf("path = %q", out.String())
	}
	// Writing to a session that does not exist creates nothing.
	if _, err := newStore(base, "nope").Append(Event{Op: "thought", Text: "x"}); err == nil {
		t.Error("appending to a missing session must fail")
	}
	if _, err := os.Stat(filepath.Join(base, ".pitboard", "nope")); err == nil {
		t.Error("a missing session was created")
	}
}
