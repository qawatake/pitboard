package viewer

import (
	"os"
	"strings"
	"testing"
	"time"
)

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
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != "      enter 開く  c 行に書く  x 隠す  space 次へ  g 飛ぶ  m 書く  ? キー" {
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
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != "      a b 答える  enter 閉じる  c 行に書く  g 飛ぶ  m 書く  ? キー" {
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
	os.Chtimes(st.ReportPath(), time.Now().Add(time.Second), time.Now().Add(time.Second))
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
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 60, 10
	pv := a.top()
	a.render()
	for pv.cur != "a" {
		a.handle(key{r: 'j'}) // the person moves to a
		a.render()
	}
	for range 4 {
		a.handle(key{name: "ctrl+u"}) // the view goes up; the cursor stays
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
	os.WriteFile(st.ReportPath(), []byte(c.String()), 0o644)
	a.reload(pv)
	a.render()
	if pv.cur != "b" || !pv.curShown {
		t.Errorf("after lines were added above: cur %q shown %v", pv.cur, pv.curShown)
	}
}

func TestLongAskKeepsItsMark(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Ask id=\"q\">"+strings.Repeat("長い問い", 50)+"\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>"), 0o644)
	a := testApp(t, st)
	a.t.w = 60
	s := shown(a)
	if !strings.Contains(s, "? 長い問い") {
		t.Errorf("a long Ask must keep ?:\n%s", s)
	}
}

func TestPlacedCursorTakesNoReaction(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	pv.settleCursor()
	pv.curShown = true
	if pv.cur != "unit" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	a.react(pv, 'a')
	if len(st.Events()) != 0 {
		t.Fatal("an option was recorded on a line the person never pointed at")
	}
	// c still writes a thought about the line.
	a.react(pv, 'c')
	if a.in == nil || a.in.id != "unit" {
		t.Errorf("c on a placed cursor: %+v", a.in)
	}
	a.in = nil
	a.render()
	a.handle(key{r: 'j'})
	a.render()
	a.handle(key{r: 'k'})
	a.render()
	pv.curShown = true
	a.react(pv, 'a')
	if evs := st.Events(); len(evs) != 1 || evs[0].Value != "a" {
		t.Errorf("after j k the option is recorded: %+v", evs)
	}
}

func TestPlacedWhenStopsAppear(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(""), 0o644) // no line yet
	a := testApp(t, st)
	pv := a.top()
	pv.settleCursor()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	os.WriteFile(st.ReportPath(), []byte("<Title>t</Title>\n<Ask id=\"new\">AI が足した問い\n<Option key=\"a\">A</Option>\n<Option key=\"b\">B</Option>\n</Ask>\n"), 0o644)
	a.reload(pv)
	pv.settleCursor()
	pv.curShown = true
	if pv.cur != "new" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	pv.open["new"] = true
	a.react(pv, 'a')
	if len(st.Events()) != 0 {
		t.Error("an answer was recorded on a line that appeared under a cursor the person did not move")
	}
}
