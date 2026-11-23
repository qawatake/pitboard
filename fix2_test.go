package main

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestTruncLongLineIsFast(t *testing.T) {
	long := plainText(strings.Repeat("あ", 300000), "")
	start := time.Now()
	got := long.trunc(70)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("trunc of 300k cells took %v", d)
	}
	if got.width() > 70 || got[len(got)-1].r != '…' {
		t.Errorf("trunc = %q (width %d)", got.plain(), got.width())
	}
	if s := plainText("abc", "").trunc(3).plain(); s != "abc" {
		t.Errorf("a fitting text is kept: %q", s)
	}
	if s := plainText("abcd", "").trunc(3).plain(); s != "ab…" {
		t.Errorf("trunc(3) = %q", s)
	}
	if s := plainText("あいう", "").trunc(5).plain(); s != "あい…" && s != "あ…" {
		t.Errorf("wide trunc = %q", s)
	}
	if s := plainText("あいう", "").trunc(5); s.width() > 5 {
		t.Errorf("wide trunc too wide: %q", s.plain())
	}
}

func TestLowercaseClosingTagOnTheSameLine(t *testing.T) {
	nodes, probs, _ := parseReport(`<Section title="あなたへ">
<Item id="hook">hook を入れるか決める</item>
<Item id="impl">最小版を実装する</Item>
</Section>
<fold title="f"></fold>`)
	var got []string
	for _, p := range probs {
		got = append(got, p.String())
	}
	want := []string{
		"2: part names start with a capital letter: </Item>, not </item>",
		"5: part names start with a capital letter: <Fold>, not <fold>",
		"5: part names start with a capital letter: </Fold>, not </fold>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	sec := nodes[0]
	if len(sec.kids) != 2 || sec.kids[1].id != "impl" || len(sec.kids[0].body) != 0 {
		t.Errorf("hook must close on its line; section holds %d parts", len(sec.kids))
	}
}

// testApp is a viewer without a terminal, on a store's report.
func testApp(t *testing.T, st *Store) *app {
	t.Helper()
	a := &app{t: &term{w: 80, h: 30, out: io.Discard}, base: st.base, lopen: map[int]bool{}}
	a.setStore(st)
	return a
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
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	pv.curShown = true
	a.react(pv, 'a')
	if evs := st.Events(); len(evs) != 1 || evs[0].Value != "a" {
		t.Errorf("after j k the option is recorded: %+v", evs)
	}
}

func TestPlacedWhenStopsAppear(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte("<Title>t</Title>\n\nまだ項目は無い。\n"), 0o644)
	a := testApp(t, st)
	pv := a.top()
	pv.settleCursor()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	os.WriteFile(st.reportPath(), []byte("<Title>t</Title>\n<Item id=\"new\">AI が足した項目</Item>\n"), 0o644)
	a.reload(pv)
	pv.settleCursor()
	pv.curShown = true
	if pv.cur != "new" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	a.react(pv, 'y')
	if len(st.Events()) != 0 {
		t.Error("y was recorded on a line that appeared under a cursor the person did not move")
	}
}

var reEsc = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)

func TestThoughtShowsTheTitleItWasWrittenOn(t *testing.T) {
	st := testStore(t)
	st.Append(Event{Op: "thought", ID: "hook", Text: "BBB について", Title: "hook を入れるか決める"})
	os.WriteFile(st.reportPath(), []byte(`<Item id="hook">まったく別の物</Item>`), 0o644)
	a := testApp(t, st)
	pv := a.top()
	pv.open["hook"] = true
	var screen []string
	for _, r := range a.rows(pv) {
		screen = append(screen, reEsc.ReplaceAllString(r.line, ""))
	}
	if s := strings.Join(screen, "\n"); !strings.Contains(s, "(書いたときは: hook を入れるか決める)") {
		t.Errorf("under the line:\n%s", s)
	}
	a.lopen[1] = true
	screen = nil
	for _, r := range a.listRows() {
		screen = append(screen, reEsc.ReplaceAllString(r.line, ""))
	}
	if s := strings.Join(screen, "\n"); !strings.Contains(s, "行: まったく別の物 (書いたときは: hook を入れるか決める)") {
		t.Errorf("in the list:\n%s", s)
	}
}

func TestPasteCRLF(t *testing.T) {
	pasting := true
	var got []rune
	b := []byte("three\r\nfour\rfive\nsix")
	for len(b) > 0 {
		k, n := parseKey(b, &pasting)
		if n == 0 {
			break
		}
		b = b[n:]
		got = append(got, k.r)
	}
	if string(got) != "three\nfour\nfive\nsix" {
		t.Errorf("pasted %q", string(got))
	}
}

func TestSessionFlagOnlyNextToTheCommand(t *testing.T) {
	st := testStore(t)
	t.Chdir(st.base)
	th, _ := st.Append(Event{Op: "thought", Text: "x"})
	var out strings.Builder
	if err := run([]string{"settle", "-s", "s1", "1", "see", "-s", "option"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-s", "s1", "settle", "1", "-s", "x"}, &out); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, e := range st.Events() {
		if e.Op == "settle" && e.Thought == th.N {
			texts = append(texts, e.Text)
		}
	}
	if strings.Join(texts, "|") != "see -s option|-s x" {
		t.Errorf("settle texts = %q", texts)
	}
}
