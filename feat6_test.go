package main

import (
	"os"
	"strings"
	"testing"
)

const marksReport = `<Title>t</Title>
<Section title="あなたへ">
<Item id="plan" ask="y">この方針で進めてよいか</Item>
<Item id="other">ただの項目</Item>
</Section>
<Fold title="済んだこと">
<Ask id="q">どちら
<Option key="a">A</Option>
<Option key="b">B</Option>
</Ask>
</Fold>
`

func TestAskAttributeCheck(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<Item id="a" ask="y">x</Item>`, ""},
		{`<Link id="a" ask="y" path="p">x</Link>`, ""},
		{`<Item id="a" ask="n">x</Item>`, `ask "n"`},
		{`<Item id="a" state="done" ask="y">x</Item>`, "done"},
		{"<Ask id=\"a\" ask=\"y\">x\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>", "no attribute"},
	} {
		_, probs, _ := parseReport(c.src)
		got := ""
		for _, p := range probs {
			got += p.Msg
		}
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q (want %q)", c.src, got, c.want)
		}
	}
}

func marksApp(t *testing.T) (*Store, *app) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(marksReport), 0o644)
	return st, testApp(t, st)
}

func TestWaitingAndNewMarks(t *testing.T) {
	st, a := marksApp(t)
	pv := a.top()
	a.render()
	if w, f := a.counts(pv); w != 2 || f != 1 {
		t.Fatalf("counts %d %d, want 2 waiting (plan, q) and 1 new (other)", w, f)
	}
	var frame strings.Builder
	a.t.out = &frame
	a.render()
	s := reEsc.ReplaceAllString(frame.String(), "")
	if !strings.Contains(s, "反応待ち 2  新着 1") {
		t.Errorf("top line:\n%s", s)
	}
	// The cursor's > wins; the others get ● in the gutter, the Fold too.
	rows := shown(a)
	for _, want := range []string{"> ○ この方針", "● ○ ただの項目", "● ▸ 済んだこと"} {
		if !strings.Contains(rows, want) {
			t.Errorf("want %q in\n%s", want, rows)
		}
	}
	// Placed cursor: resting there does not count as looking.
	wait(t, settleDelay)
	a.render()
	if f := len(st.Viewed()); f != 0 {
		t.Errorf("placed cursor marked %d viewed", f)
	}
	// y on plan: no longer waiting, and looked at.
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.handle(key{r: 'y'})
	if w, _ := a.counts(pv); w != 1 {
		t.Errorf("waiting after y: %d", w)
	}
	// Resting on "other" for settleDelay marks it as looked at.
	a.handle(key{r: 'j'})
	a.render()
	a.render()
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("new before resting: %d", f)
	}
	wait(t, settleDelay)
	a.render()
	if _, f := a.counts(pv); f != 0 {
		t.Errorf("new after resting: %d", f)
	}
	// It is remembered in the file, not in events.jsonl.
	a2 := testApp(t, st)
	if _, f := a2.counts(a2.top()); f != 0 {
		t.Errorf("a new viewer sees %d new", f)
	}
	for _, e := range st.Events() {
		if e.Op != "react" {
			t.Errorf("unexpected event %+v", e)
		}
	}
	// What the line says changes: new again.
	os.WriteFile(st.reportPath(), []byte(strings.Replace(marksReport, "ただの項目", "ただの項目 (改)", 1)), 0o644)
	a.reload(pv)
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("changed line not new: %d", f)
	}
	// A: everything now in the report is looked at.
	a.handle(key{r: 'A'})
	if w, f := a.counts(pv); w != 1 || f != 0 {
		t.Errorf("after A: %d %d", w, f)
	}
}

func TestSpaceJumps(t *testing.T) {
	_, a := marksApp(t)
	pv := a.top()
	a.render()
	// The cursor is on plan (waiting): space goes on to the next waiting
	// line, inside the closed Fold, and opens it.
	a.handle(key{r: ' '})
	a.render()
	if pv.cur != "q" || !pv.open["fold:1"] || pv.placed {
		t.Fatalf("space: cur %q open %v placed %v", pv.cur, pv.open, pv.placed)
	}
	// The line just jumped to waits settleDelay.
	a.handle(key{r: 'o'})
	a.handle(key{r: ' '}) // wraps to plan
	a.render()
	if pv.cur != "plan" {
		t.Fatalf("second space: %q", pv.cur)
	}
	a.handle(key{r: 'y'})
	if len(a.st.Events()) != 0 {
		t.Error("y right after a jump was recorded")
	}
	wait(t, settleDelay)
	a.render()
	a.handle(key{r: 'y'})
	a.handle(key{r: ' '})
	a.render()
	if pv.cur != "q" {
		t.Fatalf("third space: %q", pv.cur)
	}
	wait(t, 2*settleDelay)
	a.render()
	a.handle(key{r: 'a'}) // opens the Ask
	a.handle(key{r: 'a'})
	// No waiting left: space goes to what is new (other).
	a.handle(key{r: ' '})
	if pv.cur != "other" {
		t.Errorf("space to new: %q", pv.cur)
	}
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "space 次へ  s 送る  m 書く") {
		t.Errorf("hints: %q", s)
	}
	a.handle(key{r: 'A'})
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); strings.Contains(s, "space") {
		t.Errorf("hints with nothing to go to: %q", s)
	}
	a.handle(key{r: ' '})
	if a.msg == "" {
		t.Error("space with nothing to go to says nothing")
	}
}

func TestBrokenViewedFile(t *testing.T) {
	st, _ := marksApp(t)
	os.WriteFile(st.viewedPath(), []byte("garbage\n\x00\x01\n"), 0o644)
	a := testApp(t, st)
	if _, f := a.counts(a.top()); f != 1 {
		t.Errorf("broken file: %d new", f)
	}
	os.Remove(st.viewedPath())
	os.Mkdir(st.viewedPath(), 0o755) // cannot be read as a file
	a = testApp(t, st)
	a.render()
	a.handle(key{r: 'A'}) // cannot be written either: no crash
}
