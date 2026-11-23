package main

import (
	"strings"
	"testing"
)

// parseAll reads every key in b.
func parseAll(b string) []key {
	var out []key
	pasting := false
	buf := []byte(b)
	for len(buf) > 0 {
		k, n := parseKey(buf, &pasting)
		if n == 0 {
			break
		}
		buf = buf[n:]
		if k != (key{}) {
			out = append(out, k)
		}
	}
	return out
}

func keyName(k key) string {
	if k.name != "" {
		return k.name
	}
	return string(k.r)
}

// Every key the viewer uses reads the same with the terminal's key
// extensions off (legacy bytes), with the kitty protocol on (disambiguate),
// and with modifyOtherKeys.
func TestKeysSameWithExtensions(t *testing.T) {
	for _, c := range []struct {
		want   string
		legacy string
		kitty  string
		mok    string // modifyOtherKeys, where it changes the bytes
	}{
		{"esc", "\x1b", "\x1b[27u", ""},
		{"enter", "\r", "\r", ""},
		{"newline", "\n", "\x1b[106;5u", "\x1b[27;5;106~"},   // ctrl+j
		{"newline", "", "\x1b[13;2u", "\x1b[27;2;13~"},       // shift+enter
		{"newline", "\x1b\r", "\x1b[13;3u", "\x1b[27;3;13~"}, // alt+enter
		{"tab", "\t", "\t", ""},
		{"shift+tab", "\x1b[Z", "\x1b[9;2u", ""},
		{"backspace", "\x7f", "\x7f", ""},
		{"backspace", "\x08", "\x1b[104;5u", ""}, // ctrl+h
		{"alt+backspace", "\x1b\x7f", "\x1b[127;3u", ""},
		{"ctrl+c", "\x03", "\x1b[99;5u", "\x1b[27;5;99~"},
		{"ctrl+a", "\x01", "\x1b[97;5u", ""},
		{"ctrl+e", "\x05", "\x1b[101;5u", ""},
		{"ctrl+g", "\x07", "\x1b[103;5u", ""},
		{"ctrl+k", "\x0b", "\x1b[107;5u", ""},
		{"ctrl+n", "\x0e", "\x1b[110;5u", ""},
		{"ctrl+p", "\x10", "\x1b[112;5u", ""},
		{"ctrl+s", "\x13", "\x1b[115;5u", ""},
		{"ctrl+u", "\x15", "\x1b[117;5u", ""},
		{"ctrl+w", "\x17", "\x1b[119;5u", ""},
		{"alt+b", "\x1bb", "\x1b[98;3u", ""},
		{"alt+f", "\x1bf", "\x1b[102;3u", ""},
		{"up", "\x1b[A", "\x1b[A", ""},
		{"down", "\x1b[B", "\x1b[B", ""},
		{"left", "\x1b[D", "\x1b[D", ""},
		{"right", "\x1b[C", "\x1b[C", ""},
		{"alt+left", "\x1b[1;3D", "\x1b[1;3D", ""},
		{"alt+right", "\x1b[1;3C", "\x1b[1;3C", ""},
		{"pgup", "\x1b[5~", "\x1b[5~", ""},
		{"pgdn", "\x1b[6~", "\x1b[6~", ""},
		{"home", "\x1b[H", "\x1b[H", ""},
		{"end", "\x1b[F", "\x1b[F", ""},
		{"delete", "\x1b[3~", "\x1b[3~", ""},
		{"j", "j", "j", ""},
		{"A", "A", "\x1b[97;2u", ""},
		{" ", " ", " ", ""},
		{"ｊ", "ｊ", "ｊ", ""},
	} {
		for form, b := range map[string]string{"legacy": c.legacy, "kitty": c.kitty, "modifyOtherKeys": c.mok} {
			if b == "" {
				continue
			}
			ks := parseAll(b)
			if len(ks) != 1 || keyName(ks[0]) != c.want {
				var got []string
				for _, k := range ks {
					got = append(got, keyName(k))
				}
				t.Errorf("%s %q: got %q, want %q", form, b, got, c.want)
			}
		}
	}
}

func TestKeyboardAnswer(t *testing.T) {
	ks := parseAll("\x1b[?1uj")
	if len(ks) != 2 || ks[0].name != "kbd" || ks[1].r != 'j' {
		t.Fatalf("keys %+v", ks)
	}
	st := testStore(t)
	a := testApp(t, st)
	a.msg = "残す"
	a.handle(ks[0])
	if !a.kbd || a.msg != "残す" {
		t.Errorf("kbd %v msg %q: the answer is not a key press", a.kbd, a.msg)
	}
	a.handle(key{r: 'm'})
	if s := reEsc.ReplaceAllString(strings.Join(a.editorHints(), ""), ""); !strings.Contains(s, "shift+enter 改行") {
		t.Errorf("hints with the extension: %q", s)
	}
	a.kbd = false
	if s := reEsc.ReplaceAllString(strings.Join(a.editorHints(), ""), ""); !strings.Contains(s, "ctrl+j 改行") || strings.Contains(s, "ctrl+s") {
		t.Errorf("hints without: %q", s)
	}
	// A paste stays text even when it holds key-like sequences.
	pasting := false
	parseKey([]byte("\x1b[200~"), &pasting)
	if k, _ := parseKey([]byte("\x1b[13;2u"), &pasting); k.name != "" {
		t.Errorf("key inside a paste: %+v", k)
	}
}

// ---- rewriting thoughts ----

func TestEditBeforeDelivery(t *testing.T) {
	st := testStore(t)
	th, _ := st.Append(Event{Op: "thought", Text: "書きか"})
	st.Append(Event{Op: "edit", Thought: th.N, Text: "書きかけを直した"})
	if n := unsent(st.Events()); n != 1 {
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
	s := replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[0]); w != "対応済" {
		t.Fatalf("status %q", w)
	}
	st.Append(Event{Op: "edit", Thought: th.N, Text: "one, again"})
	st.Append(Event{Op: "edit", Thought: th.N, Text: "one, again\nand more"})
	evs := st.Events()
	if n := unsent(evs); n != 1 {
		t.Errorf("unsent %d (one per thought)", n)
	}
	s = replayEvents(evs, st.Seen())
	if w, _ := s.statusOf(s.thoughts[0]); w != "未送信" || s.thoughts[0].Text != "one, again\nand more" {
		t.Errorf("status %q text %q", w, s.thoughts[0].Text)
	}
	sendAll(st)
	lines, _ := st.Changes()
	if strings.Join(lines, "|") != "edit thought 1: one, again\n  and more" {
		t.Errorf("changes %q", lines)
	}
	s = replayEvents(st.Events(), st.Seen())
	if w, _ := s.statusOf(s.thoughts[0]); w != "既読" {
		t.Errorf("status after delivery %q (the destination was for the old text)", w)
	}
}

func TestEditKey(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "途中で出し"})
	a := testApp(t, st)
	a.handle(key{name: "tab"})
	a.render()
	if s := hintLine(a); !strings.Contains(s, "enter 開く  e 直す  x 取り消す") {
		t.Errorf("list hints %q", s)
	}
	a.handle(key{r: 'e'})
	if a.in == nil || a.in.ed.String() != "途中で出し" {
		t.Fatalf("editor %+v", a.in)
	}
	a.handle(key{name: "esc"})
	if len(st.Events()) != 1 {
		t.Error("esc recorded something")
	}
	// Empty, or unchanged: nothing.
	a.handle(key{r: 'e'})
	a.handle(key{name: "ctrl+u"})
	a.handle(key{name: "enter"})
	a.handle(key{r: 'e'})
	a.handle(key{name: "enter"})
	if len(st.Events()) != 1 {
		t.Errorf("empty or unchanged text recorded: %+v", st.Events())
	}
	a.handle(key{r: 'e'})
	for _, r := range "た文" {
		a.handle(key{r: r})
	}
	a.handle(key{name: "enter"})
	evs := st.Events()
	if len(evs) != 2 || evs[1].Op != "edit" || evs[1].Text != "途中で出した文" || evs[1].Thought != 1 {
		t.Errorf("events %+v", evs)
	}
	if a.state.thoughts[0].Text != "途中で出した文" {
		t.Errorf("shown text %q", a.state.thoughts[0].Text)
	}
}

// ---- opened items ----

func TestOpenTitleIsBold(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{name: "enter"}) // the Ask
	rows := a.rows(pv)
	var line string
	for _, r := range rows {
		if strings.Contains(r.line, "ページの単位") {
			line = r.line
		}
	}
	if !strings.Contains(line, bold) {
		t.Errorf("open title not bold: %q", line)
	}
	s := shown(a)
	if !strings.Contains(s, "プロジェクトごと\n\n") {
		t.Errorf("no blank line after the open Ask:\n%s", s)
	}
	a.handle(key{name: "enter"})
	for _, r := range a.rows(pv) {
		if strings.Contains(r.line, "ページの単位") && strings.Contains(r.line, bold) {
			t.Errorf("closed title bold: %q", r.line)
		}
	}
}
