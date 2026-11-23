package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func jumpReport() string {
	var b strings.Builder
	b.WriteString("<Notice id=\"n\">お知らせ</Notice>\n<Title>題</Title>\n\n<Section title=\"あなたへ\">\n<Item id=\"a\">項目 a</Item>\n</Section>\n\n<Section title=\"進行中\">\n")
	for i := range 30 {
		fmt.Fprintf(&b, "<Item id=\"p%d\">進行 %d</Item>\n", i, i)
	}
	b.WriteString("</Section>\n\n<Section title=\"文だけ\">\nただの文。\n</Section>\n\n<Fold title=\"決まったこと\">\n<Item id=\"d\">決めた</Item>\n<Fold title=\"内側\">\n- x\n</Fold>\n</Fold>\n")
	return b.String()
}

func TestGoTo(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(jumpReport()), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 24
	pv := a.top()
	a.render()
	ts := a.jumpTargets(pv)
	var names []string
	for _, n := range ts {
		names = append(names, jumpTitle(n))
	}
	if got := strings.Join(names, "|"); got != "お知らせ|あなたへ|進行中|文だけ|決まったこと" {
		t.Fatalf("targets %q", got)
	}
	a.handle(key{r: 'g'})
	ls, _ := screen(t, a)
	hints := plainLines(a.bottomLines(0))
	if !strings.Contains(hints, "a お知らせ  b あなたへ  c 進行中  d 文だけ  e 決まったこと  esc やめる") {
		t.Errorf("hints %q", hints)
	}
	if i := rowOf(ls, "あなたへ"); i < 0 || !strings.HasPrefix(ls[i], "b ") {
		t.Errorf("no letter at the heading:\n%s", strings.Join(ls, "\n"))
	}
	if !strings.HasPrefix(ls[0], "a") {
		t.Errorf("no letter on the notice: %q", ls[0])
	}
	// c: the first stop of 進行中, its heading at the top of the page.
	a.handle(key{r: 'c'})
	ls, _ = screen(t, a)
	if a.jumping || pv.cur != "p0" {
		t.Fatalf("after c: jumping %v cur %q", a.jumping, pv.cur)
	}
	if i := rowOf(ls, "進行中"); i < 0 || i > 3 {
		t.Errorf("the heading is not at the top (%d):\n%s", i, strings.Join(ls, "\n"))
	}
	// Moved by the viewer: y waits settleDelay.
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	if !pv.settling() {
		t.Error("not settling after g")
	}
	// e: the Fold itself, not opened.
	a.handle(key{r: 'g'})
	a.handle(key{r: 'e'})
	a.render()
	if pv.cur != "fold:1" || pv.open["fold:1"] {
		t.Errorf("fold: cur %q open %v", pv.cur, pv.open["fold:1"])
	}
	// d: no stop, only scrolls; the cursor stays.
	a.handle(key{r: 'g'})
	a.handle(key{r: 'd'})
	ls, _ = screen(t, a)
	if pv.cur != "fold:1" || rowOf(ls, "文だけ") < 0 {
		t.Errorf("text-only section: cur %q\n%s", pv.cur, strings.Join(ls, "\n"))
	}
	// esc, g, an unused letter: leave without doing anything.
	for _, k := range []key{{name: "esc"}, {r: 'g'}, {r: 'z'}} {
		before := pv.cur
		a.handle(key{r: 'g'})
		a.handle(k)
		if a.jumping || pv.cur != before {
			t.Errorf("%+v: jumping %v cur %q", k, a.jumping, pv.cur)
		}
	}
	// Not in the list.
	a.handle(key{name: "tab"})
	a.handle(key{r: 'g'})
	if a.jumping {
		t.Error("g works in the list")
	}
}

// j / k read what is taller than the screen half a screen at a time
// before moving on; the cursor stays, and reacting off screen only brings
// the line back.
func TestReadTallContents(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString("<Item id=\"a\">長い項目\n")
	for i := range 60 {
		fmt.Fprintf(&b, "  中身の %d 行目\n\n", i)
	}
	b.WriteString("</Item>\n<Item id=\"b\">次の項目</Item>\n")
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 24
	wait(t, time.Hour)
	pv := a.top()
	a.render()
	a.handle(key{r: 'j'})
	a.handle(key{r: 'k'})
	a.handle(key{name: "enter"})
	a.render()
	seen := map[int]bool{}
	look := func() {
		ls, _ := screen(t, a)
		for i := range 60 {
			if rowOf(ls, fmt.Sprintf("中身の %d 行目", i)) >= 0 {
				seen[i] = true
			}
		}
	}
	look()
	presses := 0
	for pv.cur == "a" && presses < 50 {
		a.handle(key{r: 'j'})
		a.render()
		look()
		presses++
		if pv.cur == "a" && !pv.curShown && presses == 2 {
			if s := hintLine(a); !strings.Contains(s, "↑ カーソルは上") {
				t.Errorf("off screen hints %q", s)
			}
			// y on a line off the screen: brought back, nothing recorded.
			top := pv.top
			a.handle(key{r: 'y'})
			a.render()
			if len(st.Events()) != 0 || !pv.curShown {
				t.Errorf("y off screen: events %d shown %v", len(st.Events()), pv.curShown)
			}
			pv.top, pv.follow = top, false
			a.render()
		}
	}
	if pv.cur != "b" || presses < 4 {
		t.Errorf("cur %q after %d presses", pv.cur, presses)
	}
	for i := range 60 {
		if !seen[i] {
			t.Errorf("line %d never shown", i)
			break
		}
	}
	// k reads back the same way before going to a.
	presses = 0
	for pv.cur == "b" && presses < 50 {
		a.handle(key{r: 'k'})
		a.render()
		presses++
	}
	if pv.cur != "a" || presses < 4 {
		t.Errorf("k: cur %q after %d presses", pv.cur, presses)
	}
}
