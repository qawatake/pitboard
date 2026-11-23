package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWakeMessageIsShort(t *testing.T) {
	base := t.TempDir()
	for _, s := range []string{"38d12f95-0b22-4d64-88fb-17ed69d428cb", "38d12f95-9999", "abc"} {
		os.MkdirAll(newStore(base, s).dir, 0o755)
	}
	st := newStore(base, "38d12f95-0b22-4d64-88fb-17ed69d428cb")
	if got := wakeMessage(st, 4); got != "pitboard: 4 sent. Run: pitboard -s 38d12f95-0 changes" {
		t.Errorf("%q", got)
	}
	if got := shortSession(newStore(base, "abc")); got != "abc" {
		t.Errorf("short id %q", got)
	}
	os.RemoveAll(newStore(base, "38d12f95-9999").dir)
	if got := shortSession(st); got != "38d12f95" {
		t.Errorf("alone: %q", got)
	}
	// The short id resolves to the session.
	if s, err := resolveSession(base, shortSession(st)); err != nil || s != st.session {
		t.Errorf("resolves to %q %v", s, err)
	}
}

func TestChangesAreCompact(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="hook">`+strings.Repeat("長い題", 20)+`</Item>
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

func TestSettleMany(t *testing.T) {
	st := testStore(t)
	for range 9 {
		st.Append(Event{Op: "thought", Text: "t"})
	}
	t.Chdir(st.base)
	var out bytes.Buffer
	if err := run([]string{"-s", "s1", "settle", "--done", "1", "todo にした", "--hold", "3", "答えた", "あとで"}, &out); err != nil {
		t.Fatal(err)
	}
	// Without a state first: one thought, and every word is its text.
	if err := run([]string{"-s", "s1", "settle", "9", "fixed", "5", "bugs"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-s", "s1", "settle", "2", "--doing", "made", "it", "a", "todo"}, &out); err != nil {
		t.Fatal(err)
	}
	s := replayEvents(st.Events(), 0)
	got := []string{}
	for _, n := range []int{1, 2, 3, 5, 9} {
		for _, th := range s.thoughts {
			if th.N == n {
				got = append(got, th.sstate+":"+th.settled)
			}
		}
	}
	if strings.Join(got, "|") != "done:todo にした|doing:made it a todo|hold:答えた あとで|:|done:fixed 5 bugs" {
		t.Errorf("settled %q", got)
	}
	// Errors, and nothing recorded.
	n := len(st.Events())
	for _, args := range [][]string{
		{"--done", "1", "x", "--done", "99", "y"}, // no thought 99
		{"5", "--doing", "--hold", "x"},           // two states
		{"--doing", "5", "--hold", "x"},           // two states
		{"1", "x", "--done", "2", "y"},            // several without a state first
		{"x"},
	} {
		err := run(append([]string{"-s", "s1", "settle"}, args...), &out)
		if err == nil || len(st.Events()) != n {
			t.Errorf("%q: %v, %d events", args, err, len(st.Events())-n)
		}
	}
	err := run([]string{"-s", "s1", "settle", "5", "--doing", "--hold", "x"}, &out)
	if err == nil || !strings.Contains(err.Error(), "two states") {
		t.Errorf("two states: %v", err)
	}
}

// ---- images ----

func pngFile(t *testing.T, dir, name string, size int) string {
	p := filepath.Join(dir, name)
	os.WriteFile(p, append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, size)...), 0o644)
	return p
}

func TestImageNumbersAndDelivery(t *testing.T) {
	st := testStore(t)
	n1, err := st.saveImage([]byte("a"), ".png")
	n2, _ := st.saveImage([]byte("b"), ".png")
	if err != nil || n1 != 1 || n2 != 2 {
		t.Fatalf("numbers %d %d %v", n1, n2, err)
	}
	fi, _ := os.Stat(filepath.Join(st.imagesDir(), "1.png"))
	di, _ := os.Stat(st.imagesDir())
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Errorf("modes %v %v", fi.Mode(), di.Mode())
	}
	// Removed files are not reused.
	os.Remove(filepath.Join(st.imagesDir(), "2.png"))
	if n, _ := st.saveImage([]byte("c"), ".png"); n != 3 {
		t.Errorf("after a removal: %d", n)
	}
	// Only images still in the text, and existing, are told.
	st.Append(Event{Op: "thought", Text: "見て [Image #1] と [Image #2] と [Image #3] [Image #1]"})
	sendAll(st)
	lines, _ := st.Changes()
	abs := func(n string) string { p, _ := filepath.Abs(filepath.Join(st.imagesDir(), n)); return p }
	want := []string{
		"thought 1: 見て [Image #1] と [Image #2] と [Image #3] [Image #1]",
		"  image 1: " + abs("1.png"),
		"  image 3: " + abs("3.png"),
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s", strings.Join(lines, "\n"))
	}
	if _, err := st.saveImage(make([]byte, imageMax+1), ".png"); err != errTooLarge {
		t.Errorf("too large: %v", err)
	}
}

func TestPasteImage(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	prev := clipboardImage
	t.Cleanup(func() { clipboardImage = prev })
	clipboardImage = func(context.Context) ([]byte, error) { return nil, errNoImage }
	a.handle(key{r: 'm'})
	a.handle(key{r: 'x'})
	a.handle(key{name: "ctrl+v"})
	if a.in.ed.String() != "x" || a.msg != "クリップボードに画像がありません" {
		t.Errorf("no image: %q %q", a.in.ed.String(), a.msg)
	}
	clipboardImage = func(context.Context) ([]byte, error) { return []byte("png"), nil }
	a.handle(key{name: "left"})
	a.handle(key{name: "ctrl+v"})
	if a.in.ed.String() != "[Image #1]x" {
		t.Errorf("pasted: %q", a.in.ed.String())
	}
	clipboardImage = func(context.Context) ([]byte, error) { return make([]byte, imageMax+1), nil }
	a.handle(key{name: "ctrl+v"})
	if !strings.Contains(a.msg, "大きすぎる") {
		t.Errorf("too large: %q", a.msg)
	}

	// A pasted path of an image file (as dropping a file pastes it).
	dir := filepath.Join(t.TempDir(), "my shots")
	os.MkdirAll(dir, 0o755)
	p := pngFile(t, dir, "shot 1.PNG", 10)
	a.in = newInput("", "", "思いつきを書く")
	paste := func(s string) {
		a.handle(key{name: "paste-start"})
		for _, r := range s {
			a.handle(key{r: r})
		}
		a.handle(key{name: "paste-end"})
	}
	paste(strings.ReplaceAll(p, " ", `\ `))
	if got := a.in.ed.String(); got != "[Image #2]" {
		t.Errorf("pasted path: %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(st.imagesDir(), "2.png")); len(b) != 18 {
		t.Errorf("copied %d bytes", len(b))
	}
	// Other text, a missing file, or two paths: kept as text.
	for _, s := range []string{" see " + p, "/no/such.png", p + "\n" + p, "'" + filepath.Join(dir, "x.txt") + "'"} {
		a.in = newInput("", "", "")
		paste(s)
		if a.in.ed.String() != s {
			t.Errorf("%q became %q", s, a.in.ed.String())
		}
	}
	a.in = newInput("", "", "")
	paste("'" + p + "'")
	if a.in.ed.String() != "[Image #3]" {
		t.Errorf("quoted path: %q", a.in.ed.String())
	}
	big := pngFile(t, dir, "big.png", imageMax)
	a.in = newInput("", "", "")
	paste(big)
	if a.in.ed.String() != big || !strings.Contains(a.msg, "大きすぎる") {
		t.Errorf("big file: %q %q", a.in.ed.String(), a.msg)
	}
	// Drawn faint.
	if s := withImages("a [Image #3] b", "").String(); !strings.Contains(s, dim+"[") {
		t.Errorf("not faint: %q", s)
	}
}

func TestPasteKeys(t *testing.T) {
	ks := parseAll("\x1b[200~/a b.png\x1b[201~")
	if len(ks) < 2 || ks[0].name != "paste-start" || ks[len(ks)-1].name != "paste-end" {
		t.Errorf("keys %+v", ks)
	}
}
