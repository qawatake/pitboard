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
	if !strings.Contains(hints, "esc やめる  a お知らせ  b あなたへ  c 進行中  d 文だけ  e 決まったこと") {
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
	// Not on the comments screen.
	a.handle(key{name: "tab"})
	a.handle(key{r: 'g'})
	if a.jumping {
		t.Error("g works on the comments screen")
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
			// x on a line off the screen: brought back, nothing recorded.
			top := pv.top
			a.handle(key{r: 'x'})
			a.render()
			if len(st.Events()) != 0 || !pv.curShown {
				t.Errorf("x off screen: events %d shown %v", len(st.Events()), pv.curShown)
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

func manyPlaces(n int) string {
	var b strings.Builder
	for i := range n {
		title := fmt.Sprintf("節 %d", i+1)
		if i%3 == 0 {
			title = fmt.Sprintf("とても長い題の節その %d で、見分けは終わりに付く", i+1)
		}
		fmt.Fprintf(&b, "<Section title=\"%s\">\n<Item id=\"i%d\">項目 %d</Item>\n</Section>\n\n", title, i, i)
	}
	return b.String()
}

// g is never a place's letter: with 7 places or more, g g still leaves,
// and h goes to the 7th.
func TestJumpSkipsG(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(manyPlaces(30)), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 80, 24
	pv := a.top()
	a.render()
	if n := len(a.jumpTargets(pv)); n != 25 {
		t.Fatalf("%d places", n)
	}
	before := pv.cur
	a.handle(key{r: 'g'})
	a.handle(key{r: 'g'})
	if a.jumping || pv.cur != before {
		t.Fatalf("g g: jumping %v cur %q", a.jumping, pv.cur)
	}
	a.handle(key{r: 'g'})
	a.handle(key{r: 'h'})
	if pv.cur != "i6" {
		t.Errorf("h: cur %q", pv.cur)
	}
	a.handle(key{r: 'g'})
	a.handle(key{r: 'z'})
	if pv.cur != "i24" {
		t.Errorf("z: cur %q", pv.cur)
	}
}

// The list of places takes at most a third of the screen (three lines),
// esc first, titles cut evenly but not below 6 cells, then "… +N"; the
// letters of the places left out still work.
func TestJumpListFits(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(manyPlaces(25)), 0o644)
	for _, sz := range [][2]int{{80, 24}, {40, 24}, {20, 10}, {20, 5}, {30, 2}} {
		a := testApp(t, st)
		a.t.w, a.t.h = sz[0], sz[1]
		pv := a.top()
		a.render()
		a.handle(key{r: 'g'})
		ls := a.bottomLines(0)
		hints := plainLines(ls)
		most := max(min(3, sz[1]/3), 1)
		if len(a.jumpLines(a.jumpTo)) > most {
			t.Errorf("%v: %d lines:\n%s", sz, len(a.jumpLines(a.jumpTo)), plainLines(a.jumpLines(a.jumpTo)))
		}
		if !strings.HasPrefix(strings.TrimSpace(hints), "esc やめる") && !strings.Contains(strings.Split(hints, "\n")[len(strings.Split(hints, "\n"))-len(a.jumpLines(a.jumpTo))], "esc やめる") {
			t.Errorf("%v: esc is not first:\n%s", sz, hints)
		}
		for _, l := range ls {
			if w := width(reEsc.ReplaceAllString(l, "")); w > sz[0] {
				t.Errorf("%v: %d cells: %q", sz, w, l)
			}
		}
		if sz == [2]int{80, 24} && !strings.Contains(hints, "b 節 2") {
			t.Errorf("short titles cut:\n%s", hints)
		}
		if sz == [2]int{20, 10} && !strings.Contains(hints, "… +") {
			t.Errorf("no … +N:\n%s", hints)
		}
		a.handle(key{r: 'z'})
		if pv.cur != "i24" {
			t.Errorf("%v: z: cur %q", sz, pv.cur)
		}
	}
}

// A rewrite of the report while g waits leaves goto mode: the letters
// were given to the report as it was.
func TestJumpRewriteLeaves(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(manyPlaces(4)), 0o644)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'g'})
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(st.ReportPath(), []byte("<Section title=\"零\">\n<Item id=\"z\">z</Item>\n</Section>\n\n"+manyPlaces(4)), 0o644)
	future := time.Now().Add(time.Second)
	os.Chtimes(st.ReportPath(), future, future)
	if !pv.Stale() {
		t.Skip("the rewrite is not seen as one")
	}
	a.tickReload()
	if a.jumping || a.msg != msgMoved {
		t.Errorf("jumping %v msg %q", a.jumping, a.msg)
	}
}

// The cursor's line off the screen: the arrow outranks every key, even
// with "s 送る" on a narrow line.
func TestOffScreenArrowKept(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString("<Item id=\"a\">長い項目\n")
	for i := range 60 {
		fmt.Fprintf(&b, "  中身の %d 行目\n\n", i)
	}
	b.WriteString("</Item>\n<Item id=\"b\">次の項目</Item>\n")
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	for _, w := range []int{80, 40, 20} {
		a := testApp(t, st)
		a.t.w, a.t.h = w, 24
		pv := a.top()
		a.render()
		a.handle(key{r: 'j'})
		a.handle(key{r: 'k'})
		a.handle(key{name: "enter"})
		a.render()
		a.handle(key{r: 'j'})
		a.render()
		a.handle(key{r: 'j'})
		a.render()
		if pv.curShown {
			t.Fatalf("%d: the cursor is on screen", w)
		}
		a.b.Unsent = 2
		if s := hintLine(a); !strings.Contains(s, "↑ カーソルは上") && (w > 20 || !strings.Contains(s, "↑ カーソル")) {
			t.Errorf("%d: %q", w, s)
		}
	}
}
