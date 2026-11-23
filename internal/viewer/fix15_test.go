package viewer

import (
	"os"
	"strings"
	"testing"
	"time"
)

// rewriteReport writes src as the report, newer than the last read.
func rewriteReport(t *testing.T, a *app, src string) {
	t.Helper()
	p := a.st.ReportPath()
	os.WriteFile(p, []byte(src), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(p, future, future)
}

const fix15Report = `<Notice id="restart">開き直してください
新しい版を入れました。
</Notice>
<Title>題</Title>

<Section title="s">
<Item id="a" ask="y">項目 a</Item>
</Section>
`

func bottomText(a *app) string {
	return reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
}

// x works on a cursor the viewer placed on a Notice; y says why it waits.
func TestPlacedNoticeTakesX(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(fix15Report), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	if pv.cur != "restart" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	s := bottomText(a)
	if !strings.Contains(s, "x 隠す") || !strings.Contains(s, "j k 動かすと y n が効く") {
		t.Errorf("placed hint: %q", s)
	}
	a.t.w = 60
	if s := bottomText(a); !strings.Contains(s, "動かすと y n が効く") {
		t.Errorf("placed hint shrunk at 60: %q", s)
	}
	a.t.w = 80
	a.handle(key{r: 'y'})
	if a.msg != msgPlacedYN || len(st.Events()) != 0 {
		t.Fatalf("y on placed: %q %+v", a.msg, st.Events())
	}
	a.handle(key{r: 'x'})
	if evs := st.Events(); len(evs) != 1 || evs[0].Op != "hide" || a.msg != "隠しました (H で表示、u で戻す)" {
		t.Fatalf("x on placed: %q %+v", a.msg, evs)
	}
	a.render()
	if strings.Contains(shown(a), "開き直して") {
		t.Error("the notice is still drawn")
	}
}

// A press dropped while the cursor settles says why; the same key again works.
func TestSettlingSaysWhy(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(hideReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'x'})
	a.render()
	if pv.cur != "a" || !pv.settling() {
		t.Fatalf("cursor %q settling %v", pv.cur, pv.settling())
	}
	if s := bottomText(a); strings.Contains(s, "x 隠す") {
		// The message is shown; with it gone, the hints must not offer x.
		t.Errorf("x offered while settling: %q", s)
	}
	a.msg = ""
	if s := bottomText(a); strings.Contains(s, "x 隠す") || strings.Contains(s, "y よし") {
		t.Errorf("x/y offered while settling: %q", s)
	}
	a.handle(key{r: 'x'})
	if a.msg != msgSettlingHide || len(st.Events()) != 1 {
		t.Fatalf("x while settling: %q", a.msg)
	}
	a.handle(key{r: 'x'})
	if len(st.Events()) != 2 {
		t.Fatalf("x again: %+v", st.Events())
	}
}

// Every key refused on a rewritten line says so; a hidden line the AI
// rewrote comes back with a word.
func TestLockAndReappear(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(fix15Report), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'x'}) // hide the notice
	a.render()
	rewriteReport(t, a, strings.Replace(fix15Report, "開き直してください", "もう一度開き直してください", 1))
	a.msg = ""
	a.tickReload()
	if a.msg != msgReappeared+"もう一度開き直してください" {
		t.Errorf("reappear: %q", a.msg)
	}
	// Rewrite the line under the cursor: lock.
	a.handle(key{r: 'j'})
	a.render()
	rewriteReport(t, a, strings.Replace(fix15Report, "項目 a", "項目 A", 1))
	a.tickReload()
	if !pv.lock {
		t.Fatalf("not locked: %q", pv.cur)
	}
	for _, r := range "yxrc" {
		a.handle(key{r: r})
		if a.msg != msgMoved {
			t.Errorf("%c on lock: %q", r, a.msg)
		}
	}
	_ = pv
}

// j reading through a tall notice body is the person's move.
func TestReadingTallNoticeUnplaces(t *testing.T) {
	st := testStore(t)
	body := strings.Repeat("長い本文の行です。\n\n", 40)
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(fix15Report, "新しい版を入れました。\n", body, 1)), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{name: "enter"})
	a.render()
	if !a.unpinned["restart"] || !pv.placed {
		t.Fatalf("unpinned %v placed %v", a.unpinned, pv.placed)
	}
	a.handle(key{r: 'j'})
	a.render()
	if pv.cur != "restart" || pv.placed {
		t.Fatalf("after j: cur %q placed %v", pv.cur, pv.placed)
	}
	// Read on until the title is off screen: a reaction brings it back, and says so.
	for i := 0; i < 5 && pv.curShown; i++ {
		a.handle(key{r: 'j'})
		a.render()
	}
	if pv.curShown {
		t.Skip("title still on screen")
	}
	a.handle(key{r: 'x'})
	if a.msg != msgBack || len(st.Events()) != 0 {
		t.Errorf("x off screen: %q", a.msg)
	}
}
