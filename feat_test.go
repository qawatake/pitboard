package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// ---- hook ----

func hookInput(cwd, session string, extra string) string {
	b, _ := json.Marshal(map[string]any{"session_id": session, "cwd": cwd, "hook_event_name": "x"})
	s := string(b)
	if extra != "" {
		s = strings.TrimSuffix(s, "}") + "," + extra + "}"
	}
	return s
}

func runHook(t *testing.T, event, input string) string {
	t.Helper()
	var out strings.Builder
	if err := hook(event, strings.NewReader(input), &out); err != nil {
		t.Fatalf("hook %s: %v", event, err)
	}
	return out.String()
}

func TestHookWithoutBoardDoesNothing(t *testing.T) {
	dir := t.TempDir()
	for _, in := range []string{
		hookInput(dir, "nosuch", ""),
		"not json",
		"",
		`{"cwd":"` + dir + `"}`,
		hookInput(dir, "../x", ""),
		hookInput("", "s1", ""),
	} {
		for _, ev := range []string{"prompt-submit", "post-tool", "stop", "bogus"} {
			if got := runHook(t, ev, in); got != "" {
				t.Errorf("%s with %q printed %q", ev, in, got)
			}
		}
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("hook created files: %v", ents)
	}
	// The whole command, as Claude Code runs it, exits 0 too.
	hookStdin = strings.NewReader(hookInput(dir, "nosuch", ""))
	defer func() { hookStdin = os.Stdin }()
	var out strings.Builder
	if err := run([]string{"hook", "stop"}, &out); err != nil || out.Len() != 0 {
		t.Errorf("run hook: %v %q", err, out.String())
	}
}

func TestHookOutputs(t *testing.T) {
	st := testStore(t)
	in := hookInput(st.base, "s1", "")
	const settleLine = `Settle each thought with "pitboard -s s1 settle N TEXT [N TEXT ...]" once you know where it goes.`

	// Nothing to deliver: nothing printed.
	for _, ev := range []string{"prompt-submit", "post-tool", "stop"} {
		if got := runHook(t, ev, in); got != "" {
			t.Errorf("%s with nothing new: %q", ev, got)
		}
	}

	st.Append(Event{Op: "react", ID: "hook", Value: "y", Title: "hook を入れるか決める"})
	// Not sent yet: nothing is delivered.
	if got := runHook(t, "prompt-submit", in); got != "" {
		t.Errorf("delivered before send: %q", got)
	}
	sendAll(st)
	got := runHook(t, "prompt-submit", in)
	want := "pitboard (the person acted on the board):\ny hook (hook を入れるか決める)\n" + settleLine + "\n"
	if got != want {
		t.Errorf("prompt-submit:\n%q\nwant\n%q", got, want)
	}
	if again := runHook(t, "prompt-submit", in); again != "" {
		t.Errorf("delivered twice: %q", again)
	}

	st.Append(Event{Op: "thought", Text: "経過は要らないかも"})
	sendAll(st)
	var post struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(runHook(t, "post-tool", in)), &post); err != nil {
		t.Fatal(err)
	}
	if post.HookSpecificOutput.HookEventName != "PostToolUse" ||
		!strings.Contains(post.HookSpecificOutput.AdditionalContext, "thought 3: 経過は要らないかも\n"+settleLine) {
		t.Errorf("post-tool: %+v", post)
	}

	st.Append(Event{Op: "thought", Text: "止まる前に"})
	sendAll(st)
	// Already kept from stopping once: no block, and nothing is used up.
	if got := runHook(t, "stop", hookInput(st.base, "s1", `"stop_hook_active":true`)); got != "" {
		t.Errorf("stop with stop_hook_active: %q", got)
	}
	var stop struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(runHook(t, "stop", in)), &stop); err != nil {
		t.Fatal(err)
	}
	if stop.Decision != "block" || !strings.Contains(stop.Reason, "thought 5: 止まる前に") {
		t.Errorf("stop: %+v", stop)
	}
	if got := runHook(t, "stop", in); got != "" {
		t.Errorf("stop delivered twice: %q", got)
	}
}

// ---- hiding lines ----

// wait moves the viewer's clock on by d.
func wait(t *testing.T, d time.Duration) {
	t.Helper()
	base := clock()
	prev := clock
	clock = func() time.Time { return base.Add(d) }
	t.Cleanup(func() { clock = prev })
}

func TestHideKey(t *testing.T) {
	nodes, _, _ := parseReport(`<Section title="決まったこと">
- 板はセッションごと
- 板はセッションごと
<Item id="a" state="doing" status="作業中">題</Item>
<Ask id="q">問い
<Option key="a">x</Option>
<Option key="b">y</Option>
</Ask>
<Link id="l" path="p">リンク</Link>
<Fold title="済んだこと (2)">x</Fold>
<Fold id="f" title="t">x</Fold>
段落
</Section>`)
	var keys []string
	walkAll(nodes, func(n *node) { keys = append(keys, hideKey(n)) })
	want := []string{
		"", // Section
		"-|板はセッションごと",
		"-|板はセッションごと",
		"Item|a|題|doing|作業中",
		"Ask|q|問い||", "", "",
		"Link|l|リンク||",
		"Fold|title=済んだこと (2)", "",
		"Fold|id=f", "",
		"", // paragraph
	}
	if strings.Join(keys, "\n") != strings.Join(want, "\n") {
		t.Errorf("keys:\n%q\nwant\n%q", keys, want)
	}
}

const hideReport = `<Title>t</Title>
<Section title="決まったこと">
- 板はセッションごと
<Item id="a" status="作業中">題 A</Item>
</Section>
<Section title="ほか">
<Item id="b">題 B</Item>
</Section>
`

// shown is the report as drawn, without styles.
func shown(a *app) string {
	var out []string
	for _, r := range a.rows(a.top()) {
		out = append(out, reEsc.ReplaceAllString(r.line, ""))
	}
	return strings.Join(out, "\n")
}

func TestHideAndUnhide(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.draw()
	if pv.cur != "-1:板はセッションごと" {
		t.Fatalf("the cursor must stop at the plain line first: %q", pv.cur)
	}
	// A cursor the person never moved does not hide.
	a.handle(key{r: 'x'})
	if len(st.Events()) != 0 {
		t.Fatal("x worked on a placed cursor")
	}
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.draw()
	// y n c do nothing on a plain line.
	for _, r := range "ync" {
		a.handle(key{r: r})
	}
	if len(st.Events()) != 0 || a.in != nil {
		t.Fatal("y n c acted on a plain line")
	}
	a.handle(key{r: 'x'})
	a.draw()
	if s := shown(a); strings.Contains(s, "板はセッションごと") {
		t.Errorf("the hidden line is still drawn:\n%s", s)
	}
	if pv.cur != "a" {
		t.Errorf("after hiding, the cursor goes to the next line: %q", pv.cur)
	}
	// Right after the viewer moved the cursor, x y n are dropped.
	a.handle(key{r: 'x'})
	a.handle(key{r: 'y'})
	if evs := st.Events(); len(evs) != 1 {
		t.Fatalf("keys acted on a line just moved to: %+v", evs)
	}
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	a.draw()
	if s := shown(a); strings.Contains(s, "決まったこと") {
		t.Errorf("a section whose lines are all hidden is still drawn:\n%s", s)
	}
	if pv.hiddenCount() != 2 {
		t.Errorf("hidden count = %d", pv.hiddenCount())
	}

	// H shows them faintly, and x on one brings it back.
	a.handle(key{r: 'H'})
	a.draw()
	if s := shown(a); !strings.Contains(s, "⌀ 板はセッションごと") || !strings.Contains(s, "決まったこと") {
		t.Errorf("H does not show hidden lines:\n%s", s)
	}
	pv.cur = "a"
	a.draw()
	wait(t, settleDelay)
	a.handle(key{r: 'x'})
	a.handle(key{r: 'H'})
	a.draw()
	if s := shown(a); !strings.Contains(s, "題 A") || strings.Contains(s, "板はセッションごと") {
		t.Errorf("after unhiding a:\n%s", s)
	}

	// The AI changes the hidden line's status: it is a new line and shows.
	os.WriteFile(st.reportPath(), []byte(strings.Replace(hideReport, "- 板はセッションごと", "- 板はセッションごと (確定)", 1)), 0o644)
	a.reload(pv)
	a.draw()
	if s := shown(a); !strings.Contains(s, "板はセッションごと (確定)") {
		t.Errorf("a changed line must show again:\n%s", s)
	}

	sendAll(st)
	got, _ := st.Changes()
	want := []string{"hide (2): 板はセッションごと / 題 A", "unhide (1): 題 A"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s", strings.Join(got, "\n"))
	}
}

func TestChangedItemShowsAgain(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	n := byID(a.rep.nodes, "a")
	st.Append(Event{Op: "hide", Key: hideKey(n), Title: n.title()})
	a.loadEvents()
	if s := shown(a); strings.Contains(s, "題 A") {
		t.Fatalf("hidden:\n%s", s)
	}
	os.WriteFile(st.reportPath(), []byte(strings.Replace(hideReport, `status="作業中"`, `state="done"`, 1)), 0o644)
	a.reload(a.top())
	if s := shown(a); !strings.Contains(s, "題 A") {
		t.Errorf("the item's state changed, so it shows again:\n%s", s)
	}
}

func TestViewerKeysForOptions(t *testing.T) {
	_, probs, _ := parseReport("<Ask id=\"q\">q\n<Option key=\"x\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>")
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
		t.Errorf("x as an Option key: %v", probs)
	}
}

// ---- the thoughts pane ----

func TestThoughtsPane(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	if p := a.pane(30); p != nil {
		t.Errorf("no thoughts, no pane: %q", p)
	}
	for i, txt := range []string{"一\n二", "settled one", "three", "four", "five", "six"} {
		e := Event{Op: "thought", Text: txt}
		if i == 0 {
			e.ID, e.Title = "hook", "hook を入れるか決める"
		}
		st.Append(e)
	}
	sendAll(st)
	st.Changes() // thoughts 1-6 received
	st.Settle(2, "todo にした")
	st.Append(Event{Op: "thought", Text: "new"}) // 9, not sent
	a.loadEvents()
	var p []string
	for _, l := range a.pane(30) {
		p = append(p, reEsc.ReplaceAllString(l, ""))
	}
	s := strings.Join(p, "\n")
	for _, w := range []string{"コメント", "new", "six", "five", "既読"} {
		if !strings.Contains(s, w) {
			t.Errorf("pane lacks %q:\n%s", w, s)
		}
	}
	for _, w := range []string{"行き先待ち", "ほか", "✎", "←", "→", "four"} {
		if strings.Contains(s, w) {
			t.Errorf("pane has %q:\n%s", w, s)
		}
	}
	if len(p) != 6 { // blank, heading, 3 thoughts, blank
		t.Errorf("pane has %d lines:\n%s", len(p), s)
	}
	// Screens under 20 rows: the heading only.
	a.t.h = 19
	if p := a.pane(17); len(p) != 1 {
		t.Errorf("low screen pane: %q", p)
	}
	// The settled one comes after those waiting, with its destination.
	a.t.h = 40
	order := a.paneOrder()
	if order[0].N != 9 || order[len(order)-1].N != 2 {
		t.Errorf("order: first %d last %d", order[0].N, order[len(order)-1].N)
	}
	if w, _ := a.state.statusOf(order[len(order)-1]); w != "対応済" {
		t.Errorf("settled status %q", w)
	}
	// The report keeps 6 rows: with 10 rows above the hints, one thought.
	if p := a.pane(10); len(p) != 4 {
		t.Errorf("pane in 10 rows has %d lines", len(p))
	}
}

// ---- cursor off screen ----

func TestJKBringTheCursorBack(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString("<Title>t</Title>\n\n")
	for i := 0; i < 7; i++ {
		b.WriteString("段落。\n\n")
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		b.WriteString("<Item id=\"" + id + "\">" + id + "</Item>\n")
	}
	for i := 0; i < 7; i++ {
		b.WriteString("\n段落。\n")
	}
	os.WriteFile(st.reportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 60, 10
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'}) // the person moves to a
	a.handle(key{r: 'k'})
	a.render()
	for range 12 {
		a.handle(key{r: 'k'})
		a.render()
	}
	if pv.curShown || pv.cur != "a" {
		t.Fatalf("a must be off screen: cur %q shown %v", pv.cur, pv.curShown)
	}
	a.handle(key{r: 'j'})
	a.render()
	if pv.cur != "a" || !pv.curShown {
		t.Errorf("j must bring a back first: cur %q shown %v", pv.cur, pv.curShown)
	}
	a.handle(key{r: 'j'})
	a.render()
	if pv.cur != "b" {
		t.Errorf("then j moves: %q", pv.cur)
	}

	// Lines added above: the screen follows the cursor's line.
	var c strings.Builder
	c.WriteString("<Item id=\"a\">a</Item>\n")
	for i := 0; i < 15; i++ {
		c.WriteString("\n段落。\n")
	}
	c.WriteString("<Item id=\"b\">b</Item>\n<Item id=\"c\">c</Item>\n")
	os.WriteFile(st.reportPath(), []byte(c.String()), 0o644)
	a.reload(pv)
	a.render()
	if pv.cur != "b" || !pv.curShown {
		t.Errorf("after lines were added above: cur %q shown %v", pv.cur, pv.curShown)
	}
}

func TestHelpScrolls(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.t.w, a.t.h = 90, 10
	a.handle(key{r: '?'})
	a.render()
	a.handle(key{r: 'y'}) // not a closing key
	if !a.help {
		t.Fatal("y closed the list of keys")
	}
	for range 80 {
		a.handle(key{r: 'j'})
		a.render()
	}
	rows := a.helpRows()
	if a.htop != len(rows)-(a.t.h-1) {
		t.Errorf("htop %d, rows %d", a.htop, len(rows))
	}
	a.handle(key{name: "esc"})
	if a.help {
		t.Error("esc must close the list")
	}
}

func TestNarrowHintsKeepLineKeys(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="hook">hook を入れるか決める
中身
</Item>`), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 36, 10
	a.render()
	if s := strings.Join(a.bottom(), "\n"); !strings.Contains(reEsc.ReplaceAllString(s, ""), "j k") {
		t.Errorf("placed cursor, narrow: %q", reEsc.ReplaceAllString(s, ""))
	}
	a.handle(key{r: 'j'})
	a.render()
	s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	// One line: x, enter, n, y go first; c, "m 書く" and "? キー" stay.
	if s != "    c 行にコメント  m 書く  ? キー" {
		t.Errorf("narrow hints: %q", s)
	}
	a.t.w = 100
	a.render()
	s = reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	if !strings.HasPrefix(s, "    enter 開く  y よし  n だめ  c 行に書く  x 隠す  space 次へ  m 書く  ? キー") || strings.Contains(s, "tab") || strings.Contains(s, "\n") {
		t.Errorf("wide hints: %q", s)
	}
	a.t.w = 20
	a.handle(key{r: 'j'})
	st.Append(Event{Op: "thought", Text: "x"})
	a.loadEvents()
	a.top().placed = true
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k") {
		t.Errorf("width 20, placed: %q", s)
	}
}

func TestLongAskKeepsItsMark(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte("<Ask id=\"q\">"+strings.Repeat("長い問い", 50)+"\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>"), 0o644)
	a := testApp(t, st)
	a.t.w = 60
	s := shown(a)
	if !strings.Contains(s, "? 長い問い") {
		t.Errorf("a long Ask must keep ?:\n%s", s)
	}
}
