package main

import (
	"os"
	"strings"
	"testing"
)

const marksReport = `<Title>t</Title>
<Section title="あなたへ">
<Item id="plan" ask="y">この方針で進めてよいか</Item>
<Item id="other" look="y">ただの項目</Item>
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
		{`<Item id="a" look="y">x</Item>`, ""},
		{`<Item id="a" ask="y" look="y">x</Item>`, ""},
		{`<Fold title="f" look="y">x</Fold>`, ""},
		{"<Ask id=\"a\" look=\"y\">x\n<Option key=\"a\">a</Option>\n<Option key=\"b\">b</Option>\n</Ask>", ""},
		{`<Item id="a" look="n">x</Item>`, `look "n"`},
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
	a.t.prev = nil
	a.render()
	s := reEsc.ReplaceAllString(frame.String(), "")
	if !strings.Contains(s, "反応待ち 2  未読 1") {
		t.Errorf("top line:\n%s", s)
	}
	// The marks have a column of their own, beside the cursor's, the
	// Fold's too.
	rows := shown(a)
	for _, want := range []string{"> ● ○ この方針", "  ● ○ ただの項目", "  ● ▸ 済んだこと"} {
		if !strings.Contains(rows, want) {
			t.Errorf("want %q in\n%s", want, rows)
		}
	}
	// y on plan: no longer waiting.
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.render()
	wait(t, settleDelay)
	a.handle(key{r: 'y'})
	if w, _ := a.counts(pv); w != 1 {
		t.Errorf("waiting after y: %d", w)
	}
	// On "other": still unread while the cursor is on it, however long.
	a.handle(key{r: 'j'})
	a.render()
	wait(t, settleDelay)
	a.render()
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("unread while on it: %d", f)
	}
	if rows := shown(a); !strings.Contains(rows, "> ● ○ ただの項目") {
		t.Errorf("the mark goes under the cursor:\n%s", rows)
	}
	// Leaving it: read.
	a.handle(key{r: 'k'})
	a.render()
	if _, f := a.counts(pv); f != 0 {
		t.Errorf("unread after leaving: %d", f)
	}
	// It is remembered in the file, not in events.jsonl.
	a2 := testApp(t, st)
	if _, f := a2.counts(a2.top()); f != 0 {
		t.Errorf("a new viewer sees %d unread", f)
	}
	for _, e := range st.Events() {
		if e.Op != "react" {
			t.Errorf("unexpected event %+v", e)
		}
	}
	// What the line says changes: unread again.
	os.WriteFile(st.reportPath(), []byte(strings.Replace(marksReport, "ただの項目", "ただの項目 (改)", 1)), 0o644)
	a.reload(pv)
	if _, f := a.counts(pv); f != 1 {
		t.Errorf("changed line not unread: %d", f)
	}
	// A: everything now in the report is read; u takes that back.
	a.handle(key{r: 'A'})
	if w, f := a.counts(pv); w != 1 || f != 0 {
		t.Errorf("after A: %d %d", w, f)
	}
	a.handle(key{r: 'u'})
	if _, f := a.counts(pv); f != 1 || !strings.Contains(a.msg, "A") {
		t.Errorf("after u: %d %q", f, a.msg)
	}
	// Lines without look="y" are never unread.
	os.WriteFile(st.reportPath(), []byte(strings.Replace(marksReport, ` look="y"`, "", 1)), 0o644)
	a.reload(pv)
	if _, f := a.counts(pv); f != 0 {
		t.Errorf("unread without look: %d", f)
	}
}

func TestSpaceJumps(t *testing.T) {
	_, a := marksApp(t)
	pv := a.top()
	a.render()
	// From plan (waiting), the nearest marked line below: other (unread),
	// then q inside the closed Fold (opened), then round to plan.
	for _, want := range []string{"other", "q", "plan"} {
		a.handle(key{r: ' '})
		a.render()
		if pv.cur != want || pv.placed {
			t.Fatalf("space: cur %q, want %q", pv.cur, want)
		}
	}
	if !pv.open["fold:1"] {
		t.Error("the Fold was not opened")
	}
	// The line just jumped to waits settleDelay.
	a.handle(key{r: 'y'})
	if len(a.st.Events()) != 0 {
		t.Error("y right after a jump was recorded")
	}
	wait(t, settleDelay)
	a.render()
	a.handle(key{r: 'y'})
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "space 次へ") {
		t.Errorf("hints: %q", s)
	}
	// An Item waiting for the person (wait="you") does not hold space:
	// with two of them, space still goes on to what is below.
	os.WriteFile(a.st.reportPath(), []byte(`<Item id="w1" wait="you" status="あなた待ち">一</Item>
<Item id="w2" wait="you" status="あなた待ち">二</Item>
<Item id="n1" look="y">三</Item>
`), 0o644)
	a.reload(pv)
	pv.cur = "w1"
	for _, want := range []string{"w2", "n1", "w1"} {
		a.handle(key{r: ' '})
		if pv.cur != want {
			t.Fatalf("space with wait=you: %q, want %q", pv.cur, want)
		}
	}
	a.handle(key{r: 'A'})
	os.WriteFile(a.st.reportPath(), []byte(`<Item id="n1">三</Item>`), 0o644)
	a.reload(pv)
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); strings.Contains(s, "space") {
		t.Errorf("hints with nothing to go to: %q", s)
	}
	a.handle(key{r: ' '})
	if a.msg != "印のある行はありません" {
		t.Errorf("msg %q", a.msg)
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
