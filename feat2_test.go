package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- sending ----

// noSendTarget makes sure no test sends anywhere real.
func noSendTarget(t *testing.T) {
	t.Helper()
	t.Setenv("PITBOARD_SEND", "")
	t.Setenv("HERDR_ENV", "")
	t.Setenv("PATH", "/usr/bin:/bin") // no herdr (sh is still there)
}

// fakeHerdr puts a stand-in for herdr first on PATH: 'agent list' prints
// list, 'agent prompt' writes its arguments to the returned file.
func fakeHerdr(t *testing.T, list string) string {
	t.Helper()
	noSendTarget(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "list.json"), []byte(list), 0o644)
	prompted := filepath.Join(dir, "prompted")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1 $2" = "agent list" ]; then cat %q; exit 0; fi
if [ "$1 $2" = "agent prompt" ]; then printf '%%s\n%%s\n' "$3" "$4" > %q; exit 0; fi
echo "unexpected: $*" >&2; exit 2
`, filepath.Join(dir, "list.json"), prompted)
	os.WriteFile(filepath.Join(dir, "herdr"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// HERDR_ENV is left unset: herdr on PATH is enough (the viewer may run
	// outside herdr).
	return prompted
}

const herdrList = `{"id":"cli:agent:list","result":{"agents":[
 {"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"other"},"pane_id":"w1:p1"},
 {"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"s1"},"pane_id":"w1:p2"}
],"type":"agent_list"}}`

func TestSendThroughHerdr(t *testing.T) {
	prompted := fakeHerdr(t, herdrList)
	st := testStore(t)
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if a.msg != "未送信のものはありません" {
		t.Errorf("nothing to send: %q", a.msg)
	}
	if _, err := os.Stat(prompted); err == nil {
		t.Fatal("sent with nothing to send")
	}
	st.Append(Event{Op: "thought", Text: "a"})
	st.Append(Event{Op: "hide", Key: "-|x", Title: "x"})
	a.handle(key{r: 's'})
	if a.msg != "送りました" {
		t.Fatalf("msg %q", a.msg)
	}
	b, _ := os.ReadFile(prompted)
	want := "w1:p2\n" + wakeMessage(st, 2) + "\n"
	if string(b) != want {
		t.Errorf("prompted %q, want %q", b, want)
	}
	if !strings.Contains(want, `pitboard: 2 sent. Run: pitboard -s s1 changes`) {
		t.Errorf("message: %q", want)
	}
	if st.Seen() != 0 {
		t.Error("sending must not mark anything as delivered")
	}
	if evs := st.Events(); evs[len(evs)-1].Op != "send" {
		t.Errorf("s must record a send: %+v", evs[len(evs)-1])
	}
}

func TestSendHerdrWithoutTheSession(t *testing.T) {
	prompted := fakeHerdr(t, `{"result":{"agents":[{"agent_session":{"value":"other"},"pane_id":"w1:p1"}]}}`)
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if !strings.Contains(a.msg, "送りました。ただ") || !strings.Contains(a.msg, "herdr にこのセッション (s1) の AI がいません。AI を herdr の中で動かすか、PITBOARD_SEND を設定してください") {
		t.Errorf("msg %q", a.msg)
	}
	if _, err := os.Stat(prompted); err == nil {
		t.Error("prompted another agent")
	}
}

func TestSendNowhere(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	a.handle(key{r: 's'})
	if a.msg != "送りました。ただ AI が待っていないので、チャットで一言伝えてください。\n"+`AI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")` {
		t.Errorf("msg %q", a.msg)
	}
	// The mark stays: the AI gets the thought from changes.
	if lines, _ := st.Changes(); strings.Join(lines, "\n") != "thought 1: a" {
		t.Errorf("changes after a failed wake: %q", lines)
	}
	// Wrapped, never cut: how to fix it can be read at any width.
	a.t.w = 50
	if b := plainLines(a.bottom()); strings.Contains(b, "…") || !strings.Contains(b, `"Send")`) {
		t.Errorf("failure message lines: %q", b)
	}
}

func TestSendCommand(t *testing.T) {
	noSendTarget(t)
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("PITBOARD_SEND", `printf '%s|%s' "$PITBOARD_SESSION" "$PITBOARD_MESSAGE" > `+out)
	t.Setenv("HERDR_ENV", "1") // PITBOARD_SEND comes first
	st := testStore(t)
	a := testApp(t, st)
	// ctrl+s in the editor does nothing; s sends once written.
	a.handle(key{r: 'm'})
	for _, r := range "思いつき" {
		a.handle(key{r: r})
	}
	a.handle(key{name: "ctrl+s"})
	if a.in == nil || len(st.Events()) != 0 {
		t.Fatalf("ctrl+s in the editor must do nothing: %+v", st.Events())
	}
	a.handle(key{name: "enter"})
	a.handle(key{r: 's'})
	if a.in != nil || len(st.Events()) != 2 || st.Events()[0].Text != "思いつき" || st.Events()[1].Op != "send" {
		t.Fatalf("enter then s must write and send: %+v", st.Events())
	}
	b, _ := os.ReadFile(out)
	if string(b) != "s1|"+wakeMessage(st, 1) {
		t.Errorf("PITBOARD_SEND got %q", b)
	}
	if a.msg != "送りました" {
		t.Errorf("msg %q", a.msg)
	}
	// enter only writes.
	os.Remove(out)
	a.handle(key{r: 'm'})
	a.handle(key{r: 'x'})
	a.handle(key{name: "enter"})
	if _, err := os.Stat(out); err == nil || len(st.Events()) != 3 {
		t.Error("enter must write without sending")
	}
}

func TestSendFailsAndTimesOut(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	st.Append(Event{Op: "thought", Text: "a"})
	a := testApp(t, st)
	t.Setenv("PITBOARD_SEND", "echo first line >&2; echo second >&2; exit 3")
	a.handle(key{r: 's'})
	if !strings.Contains(a.msg, "送れませんでした") || !strings.Contains(a.msg, "first line") || strings.Contains(a.msg, "second") {
		t.Errorf("failure: %q", a.msg)
	}
	prev := sendTimeout
	sendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sendTimeout = prev })
	t.Setenv("PITBOARD_SEND", "sleep 5")
	st.Append(Event{Op: "thought", Text: "b"})
	start := time.Now()
	a.handle(key{r: 's'})
	if d := time.Since(start); d > 2*time.Second || !strings.Contains(a.msg, "打ち切りました") {
		t.Errorf("timeout after %v: %q", d, a.msg)
	}
}

// screen draws a and returns the screen lines without styles, and the raw
// output.
func screen(t *testing.T, a *app) ([]string, string) {
	t.Helper()
	var buf bytes.Buffer
	a.t.out = &buf
	a.t.prev = nil // the whole frame
	a.draw()
	raw := buf.String()
	body := strings.TrimPrefix(raw, "\x1b[H")
	var lines []string
	for _, l := range strings.Split(body, "\r\n") {
		l = strings.ReplaceAll(l, "\x1b[2K", "")
		lines = append(lines, reEsc.ReplaceAllString(l, ""))
	}
	return lines, raw
}

func TestUnsentMark(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	title := strings.Repeat("とても長い題", 12)
	report := "<Title>" + title + "</Title>\n\n<Section title=\"s\">\n"
	for i := range 40 {
		report += fmt.Sprintf("<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.reportPath(), []byte(report+"</Section>\n"), 0o644)
	a := testApp(t, st)
	if ls, _ := screen(t, a); strings.Contains(strings.Join(ls, "\n"), "未送信") {
		t.Error("nothing unsent, no mark")
	}
	st.Append(Event{Op: "thought", Text: "a"})
	st.Append(Event{Op: "react", ID: "i1", Value: "y"})
	st.Append(Event{Op: "hide", Key: "k", Title: "x"})
	a.loadEvents()
	// The top line is the blank padding: the mark goes on its right.
	ls, _ := screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") || visibleWidth(ls[0]) != lgutter+a.textWidth() {
		t.Errorf("top line %q", ls[0])
	}
	// Scrolled so the title is on top: the title is cut to make room.
	a.top().top = 1
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") || !strings.Contains(ls[0], "とても長い題") || !strings.Contains(ls[0], "…") || visibleWidth(ls[0]) > lgutter+a.textWidth() {
		t.Errorf("title line %q", ls[0])
	}
	// Scrolled further: still on the top line.
	a.top().top = 10
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") {
		t.Errorf("scrolled: %q", ls[0])
	}
	// The thoughts list shows it too; the editor's text is not counted.
	a.list = true
	a.in = &input{label: "x", ed: newEdit("書きかけ")}
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") {
		t.Errorf("list: %q", ls[0])
	}
	a.in, a.list = nil, false
	// s clears it: what was sent waits, faint, until the AI reads it.
	t.Setenv("PITBOARD_SEND", "true")
	a.handle(key{r: 's'})
	ls, _ = screen(t, a)
	if strings.Contains(ls[0], "未送信") || !strings.HasSuffix(ls[0], "送信済 3") {
		t.Errorf("after s: %q", ls[0])
	}
	// Something new after the send: both.
	st.Append(Event{Op: "thought", Text: "b"})
	a.loadEvents()
	if ls, _ := screen(t, a); !strings.HasSuffix(ls[0], "送信済 3  ● 未送信 1") {
		t.Errorf("sent and unsent: %q", ls[0])
	}
	st.Changes()
	a.loadEvents()
	if ls, _ := screen(t, a); strings.Contains(ls[0], "送信済") || !strings.HasSuffix(ls[0], "● 未送信 1") {
		t.Errorf("after changes: %q", ls[0])
	}
}

// ---- the m hint ----

func TestHintsShowM(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.render()
	a.handle(key{r: 'j'}) // the Item
	a.render()
	s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	if s != "      y よし  n だめ  c 行に書く  x 隠す  space 次へ  m 書く  ? キー" {
		t.Errorf("hints: %q", s)
	}
	// Dropped in order x, enter, n, y, c; m and ? stay.
	for _, c := range []struct {
		w    int
		want string
	}{
		{54, "      y よし  n だめ  c 行に書く  m 書く  ? キー"},
		{48, "      y よし  c 行に書く  m 書く  ? キー"},
		{40, "      c 行に書く  m 書く  ? キー"},
		{26, "      m 書く  ? キー"},
	} {
		a.t.w = c.w
		if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != c.want {
			t.Errorf("width %d: %q, want %q", c.w, s, c.want)
		}
	}
	a.t.w = 80
	a.list = true
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "m 書く  ? キー") {
		t.Errorf("list hints: %q", s)
	}
}

// ---- the input position ----

func TestEditorCursor(t *testing.T) {
	st := testStore(t)
	for _, c := range []struct {
		text string
		last string // the last line of the text as drawn
	}{
		{"abc", "abc"},
		{"日本語", "日本語"},
		{strings.Repeat("あ", 40), strings.Repeat("あ", 4)}, // 72 cells a line
		{"ab\nいう", "いう"},
		{"abc\n", ""},
	} {
		a := testApp(t, st)
		a.in = &input{label: "思いつきを書く", ed: newEdit(c.text)}
		ls, raw := screen(t, a)
		row := -1
		for i, l := range ls {
			if strings.TrimRight(l, " ") == strings.TrimRight("      "+c.last, " ") {
				row = i
			}
		}
		if row < 0 {
			t.Fatalf("%q: last line %q not on screen:\n%s", c.text, c.last, strings.Join(ls, "\n"))
		}
		want := fmt.Sprintf("\x1b[%d;%dH\x1b[?25h", row+1, lgutter+markW+width(c.last)+1)
		if !strings.HasSuffix(raw, want) {
			t.Errorf("%q: frame ends %q, want %q", c.text, raw[max(len(raw)-20, 0):], want)
		}
		if strings.Contains(raw, rev) {
			t.Errorf("%q: the input position must not be drawn reversed", c.text)
		}
		// Closing the editor hides the cursor again.
		a.handle(key{name: "esc"})
		if _, raw := screen(t, a); !strings.HasSuffix(raw, "\x1b[?25l") {
			t.Errorf("%q: cursor not hidden after closing", c.text)
		}
	}
}

// ---- a question opens to its options ----

func TestAskOpensToItsOptions(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	if s := shown(a); strings.Contains(s, "セッションごと") {
		t.Errorf("a closed Ask shows its options:\n%s", s)
	}
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'}) // on the Ask, pointed at
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != "      enter 開く  c 行に書く  x 隠す  space 次へ  m 書く  ? キー" {
		t.Errorf("closed hints: %q", s)
	}
	a.handle(key{r: 'a'})
	if len(st.Events()) != 0 || !pv.open["unit"] {
		t.Fatalf("an option key on a closed Ask must only open it (events %d)", len(st.Events()))
	}
	a.render()
	if s := shown(a); !strings.Contains(s, "セッションごと") {
		t.Errorf("an open Ask shows its options:\n%s", s)
	}
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != "      a b 答える  enter 閉じる  c 行に書く  x 隠す  space 次へ  m 書く  ? キー" {
		t.Errorf("open hints: %q", s)
	}
	a.handle(key{r: 'a'})
	if evs := st.Events(); len(evs) != 1 || evs[0].Value != "a" {
		t.Fatalf("answer: %+v", evs)
	}
	if !pv.open["unit"] {
		t.Error("answering must keep it open")
	}
	// Kept open across a reload.
	os.Chtimes(st.reportPath(), time.Now().Add(time.Second), time.Now().Add(time.Second))
	a.reload(pv)
	if !pv.open["unit"] {
		t.Error("reload closed the Ask")
	}
	// Closed and answered: the key and the head of the option.
	a.handle(key{name: "enter"})
	a.render()
	s := shown(a)
	if !strings.Contains(s, "? ページの単位") || !strings.Contains(s, "a セッション…") || strings.Contains(s, "プロジェクトごと") {
		t.Errorf("closed, answered:\n%s", s)
	}
	// Too narrow for the option: the key only.
	a.t.w = 30
	if s := shown(a); strings.Contains(s, "a セッション") || !strings.Contains(s, " a") {
		t.Errorf("narrow, answered:\n%s", s)
	}
}

func TestSkillIsThin(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"--skill"}, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.HasPrefix(s, "---\nname: pitboard\ndescription: ") || !strings.Contains(s, "pitboard -h") || strings.Contains(s, "hook post-tool") {
		t.Errorf("skill:\n%s", s)
	}
	out.Reset()
	run([]string{"-h"}, &out)
	for _, w := range []string{"pitboard hook post-tool", "settle", "scratchpad", "PITBOARD_SEND", "herdr agent prompt", "→ now:"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("-h lacks %q", w)
		}
	}
}
