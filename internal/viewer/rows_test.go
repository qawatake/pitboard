package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

const colorReport = `<Item id="you" wait="you" status="あなた待ち">決めてほしい</Item>
<Item id="run" state="doing" status="作業中">動いている</Item>
<Item id="queue" status="順番待ち">待っている</Item>
<Item id="old" status="あなた待ち">語だけ</Item>
<Item id="plan" ask="y">聞いている</Item>
`

func TestStatusColors(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(colorReport), 0o644)
	a := testApp(t, st)
	pv := a.top()
	for id, color := range map[string]string{"you": yellow, "run": green, "queue": dim, "old": dim} {
		n := report.ByID(pv.nodes, id)
		r := a.right(pv, n)
		if len(r) == 0 || r[0].st != color {
			t.Errorf("%s: status color %q, want %q", id, r, color)
		}
	}
	// wait="you" is not "waiting", but space goes to it after the waiting.
	if w, _ := a.counts(pv); w != 1 {
		t.Errorf("waiting %d (only ask=\"y\")", w)
	}
	a.render()
	a.handle(key{r: ' '})
	if pv.cur != "plan" {
		t.Fatalf("first space: %q", pv.cur)
	}
	st.Append(board.Event{Op: "react", ID: "plan", Value: "y", Line: report.ByID(pv.nodes, "plan").Sig()})
	a.loadEvents()
	a.viewAll(pv)
	a.handle(key{r: ' '})
	if pv.cur != "you" {
		t.Errorf("second space: %q, want the wait=\"you\" Item before anything new", pv.cur)
	}
}

// rightOf is what a line shows at its right end, without styles.
func rightOf(a *app, id string) string {
	pv := a.top()
	return a.right(pv, report.ByID(pv.nodes, id)).plain()
}

// squeeze drops spaces and line ends, to find a text however it wrapped.
func squeeze(s string) string {
	return strings.NewReplacer(" ", "", "\n", "", "…", "").Replace(s)
}

func TestTitlesWrapAndRightPartsStayWhole(t *testing.T) {
	titles := []string{
		"短い題",
		strings.Repeat("とても長い題で", 12),
		strings.Repeat("a long English title ", 8),
		"混ざった title と English と日本語が" + strings.Repeat("続く mixed 行", 6),
	}
	statuses := []string{"", "順番待ち", "あなたの返事を待っています (hook を入れるかどうか)"}
	var src strings.Builder
	src.WriteString("<Title>" + titles[1] + "</Title>\n\n<Section title=\"s\">\n")
	i := 0
	for _, ti := range titles {
		for _, st := range statuses {
			attr := ""
			if st != "" {
				attr = fmt.Sprintf(" status=%q", st)
			}
			fmt.Fprintf(&src, "<Item id=\"i%d\"%s>%s</Item>\n", i, attr, ti)
			fmt.Fprintf(&src, "<Item id=\"d%d\" state=\"done\"%s>%s</Item>\n", i, attr, ti)
			i++
		}
	}
	fmt.Fprintf(&src, "<Link id=\"l\" path=\"x\">%s</Link>\n", titles[3])
	src.WriteString("- " + titles[1] + "\n</Section>\n")
	for w := 20; w <= 200; w += 3 {
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte(src.String()), 0o644)
		st.Append(board.Event{Op: "thought", ID: "i1", Text: titles[3]})
		st.Append(board.Event{Op: "thought", Text: titles[1]})
		a := testApp(t, st)
		a.t.w, a.t.h = w, 400
		var lines []string
		for _, r := range a.rows(a.top()) {
			lines = append(lines, r.line)
		}
		lines = append(lines, a.pane(400)...)
		a.list = true
		for _, r := range a.listRows() {
			lines = append(lines, r.line)
		}
		a.list = false
		for _, l := range lines {
			if vw := visibleWidth(l); vw > w {
				t.Fatalf("w %d: a line of %d columns: %q", w, vw, reEsc.ReplaceAllString(l, ""))
			}
		}
		all := squeeze(reEsc.ReplaceAllString(strings.Join(lines, "\n"), ""))
		for _, s := range append(statuses[1:], "未送信") {
			if !strings.Contains(all, squeeze(s)) {
				t.Errorf("w %d: %q was cut", w, s)
			}
		}
		for _, ti := range titles {
			if !strings.Contains(all, squeeze(ti)) {
				t.Errorf("w %d: title %q was cut", w, ti)
			}
		}
	}
}

func TestWrappedTitleRows(t *testing.T) {
	st := testStore(t)
	long := strings.Repeat("長い題", 30)
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"a\" status=\"順番待ち\">"+long+"</Item>\n<Item id=\"b\">次</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w = 60
	pv := a.top()
	rows := a.rows(pv)
	var first int
	for i, r := range rows {
		if r.key == "a" {
			first = i
		}
	}
	head := reEsc.ReplaceAllString(rows[first].line, "")
	if !strings.HasSuffix(head, "順番待ち") {
		t.Errorf("the status is not at the end of the first row: %q", head)
	}
	col := visibleWidth(head[:strings.Index(head, "長")]) // the title's first character
	if !rows[first+1].cont {
		t.Fatal("the title did not wrap")
	}
	next := reEsc.ReplaceAllString(rows[first+1].line, "")
	if lead := len(next) - len(strings.TrimLeft(next, " ")); lead != col {
		t.Errorf("the second row is not under the title:\n%q\n%q", head, next)
	}
	if strings.Contains(next, "順番待ち") || strings.HasPrefix(next, ">") {
		t.Errorf("the second row carries the right part or the cursor: %q", next)
	}
	// A long title alone does not open.
	if a.canOpen(pv, report.ByID(pv.nodes, "a")) {
		t.Error("a long title opens without contents")
	}
}

func TestScrollKeepsWrappedLineWhole(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	for i := range 30 {
		fmt.Fprintf(&b, "<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	b.WriteString("<Item id=\"long\">" + strings.Repeat("とても長い題", 20) + "</Item>\n")
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 40, 20
	pv := a.top()
	a.draw()
	for range 30 {
		a.handle(key{r: 'j'})
		a.draw()
	}
	if pv.cur != "long" {
		t.Fatalf("cursor %q", pv.cur)
	}
	lines, _ := screen(t, a)
	got := squeeze(strings.Join(lines, "\n"))
	if !strings.Contains(got, squeeze(strings.Repeat("とても長い題", 20))) {
		t.Errorf("the cursor's wrapped line is not wholly on screen:\n%s", strings.Join(lines, "\n"))
	}
}

func TestNoticeFixedAtTop(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString(noticeReport)
	for i := range 60 {
		fmt.Fprintf(&b, "<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.ReportPath(), []byte(b.String()), 0o644)
	a := testApp(t, st)
	pv := a.top()
	lines, raw := screen(t, a)
	if !strings.Contains(lines[0], "pitboard を開き直してください") || !strings.HasPrefix(lines[0], ">") {
		t.Fatalf("top line %q", lines[0])
	}
	if !strings.Contains(strings.SplitN(raw, "\r\n", 2)[0], yellow) {
		t.Error("not yellow")
	}
	if pv.cur != "restart" || !pv.placed {
		t.Fatalf("cursor %q placed %v", pv.cur, pv.placed)
	}
	// The placed cursor takes no x.
	a.handle(key{r: 'x'})
	if len(st.Events()) != 0 {
		t.Fatal("x on a placed cursor")
	}
	for range 50 {
		a.handle(key{r: 'j'})
		a.draw()
	}
	lines, _ = screen(t, a)
	if !strings.Contains(lines[0], "pitboard を開き直してください") || strings.HasPrefix(lines[0], ">") {
		t.Fatalf("scrolled: top line %q", lines[0])
	}
	// k from the first line goes back up into it.
	for range 60 {
		a.handle(key{r: 'k'})
		a.draw()
	}
	if pv.cur != "restart" || pv.placed || !pv.curShown {
		t.Fatalf("k to the notice: %q placed %v shown %v", pv.cur, pv.placed, pv.curShown)
	}
	// Not counted, not jumped to.
	if w, f := a.counts(pv); w != 1 || f != 0 {
		t.Errorf("counts %d %d", w, f)
	}
	a.handle(key{r: ' '})
	if pv.cur != "a" {
		t.Errorf("space went to %q", pv.cur)
	}
	a.handle(key{r: 'k'})
	a.draw()
	wait(t, settleDelay)
	// y works on it, and x hides it.
	a.handle(key{r: 'y'})
	if evs := st.Events(); len(evs) != 1 || evs[0].Value != "y" || evs[0].ID != "restart" {
		t.Fatalf("y: %+v", evs)
	}
	a.handle(key{r: 'x'})
	a.draw()
	lines, _ = screen(t, a)
	if strings.Contains(strings.Join(lines, "\n"), "開き直して") {
		t.Fatal("hidden, still shown")
	}
	evs := st.Events()
	if e := evs[len(evs)-1]; e.Op != "hide" || !strings.HasPrefix(e.Key, "Notice|restart|") {
		t.Fatalf("hide: %+v", e)
	}
	sendAll(st)
	if ls, _ := st.Changes(); !strings.Contains(strings.Join(ls, "\n"), "hide (1): pitboard を開き直して") {
		t.Errorf("changes: %q", ls)
	}
	// Rewritten by the AI: it shows again.
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(b.String(), "新しい版を入れました", "もう一度入れ直しました", 1)), 0o644)
	os.Chtimes(st.ReportPath(), time.Now().Add(time.Second), time.Now().Add(time.Second))
	a.reload(pv)
	lines, _ = screen(t, a)
	if !strings.Contains(lines[0], "もう一度入れ直しました") {
		t.Errorf("rewritten notice not back: %q", lines[0])
	}
}

// A notice's title is never cut; the notices take a third of the screen
// at most, the rest summed up on the last row, and the report and the
// cursor stay in view.
func TestNoticesTakeAThirdAtMost(t *testing.T) {
	st := testStore(t)
	long := strings.Repeat("長いお知らせ", 40)
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n1\">一つ目のお知らせ</Notice>\n<Notice id=\"n2\">"+long+"</Notice>\n<Notice id=\"n3\">三つ目</Notice>\n<Title>題</Title>\n<Item id=\"a\">項目 a</Item>\n<Item id=\"b\">項目 b</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w = 40
	// Tall enough: every title in full.
	a.t.h = 60
	lines, _ := screen(t, a)
	all := strings.Join(lines, "")
	if !strings.Contains(strings.ReplaceAll(all, " ", ""), long) || !strings.Contains(all, "三つ目") || strings.Contains(all, "…") {
		t.Errorf("titles cut:\n%s", strings.Join(lines, "\n"))
	}
	for _, h := range []int{5, 8, 24} {
		a.t.h = h
		a.render()
		pv := a.top()
		// From the notices down to the report.
		for range 4 {
			a.handle(key{r: 'j'})
		}
		lines, _ := screen(t, a)
		pins := a.pin(pv, h, "")
		if len(pins.rows) > max(h/3, 1) {
			t.Errorf("h %d: notices take %d rows", h, len(pins.rows))
		}
		if !strings.Contains(strings.Join(lines, "\n"), "ほか") {
			t.Errorf("h %d: no sum of the notices left out:\n%s", h, strings.Join(lines, "\n"))
		}
		cur := false
		for _, l := range lines {
			cur = cur || strings.HasPrefix(l, ">")
		}
		if !cur || pv.nodeAt(pv.cur).Kind == report.KindNotice {
			t.Errorf("h %d: cursor %q not on screen:\n%s", h, pv.cur, strings.Join(lines, "\n"))
		}
		// k back up to the long notice: it is shown, with the cursor on it.
		for range 4 {
			a.handle(key{r: 'k'})
		}
		lines, _ = screen(t, a)
		cur = false
		for _, l := range lines {
			cur = cur || strings.HasPrefix(l, ">")
		}
		if !cur || pv.nodeAt(pv.cur).Kind != report.KindNotice {
			t.Errorf("h %d: cursor %q on a notice not shown:\n%s", h, pv.cur, strings.Join(lines, "\n"))
		}
	}
}

// An empty report says so, faintly.
func TestEmptyReport(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), nil, 0o644)
	a := testApp(t, st)
	lines, _ := screen(t, a)
	if !strings.Contains(strings.Join(lines, "\n"), "報告はまだ空です。AI が書くと、ここに出ます。") {
		t.Errorf("empty report:\n%s", strings.Join(lines, "\n"))
	}
}

// A notice opens like an Item: its contents under its title, pinned while
// they fit in a third of the screen, else at the head of the page.
func TestNoticeOpens(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n\">開き直してください\n  手順その一\n  <Code>\n  a\n  b\n  c\n  d\n  e\n  f\n  </Code>\n</Notice>\n<Title>題</Title>\n<Item id=\"a\">項目</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.h = 60
	a.render()
	pv := a.top()
	if pv.cur != "n" {
		t.Fatalf("cursor %q", pv.cur)
	}
	lines, _ := screen(t, a)
	if !strings.HasPrefix(lines[0], ">   ▸ 開き直してください") || strings.Contains(lines[0], "!") || strings.Contains(strings.Join(lines, "\n"), "手順その一") {
		t.Errorf("closed:\n%s", strings.Join(lines, "\n"))
	}
	a.handle(key{name: "enter"})
	lines, _ = screen(t, a)
	if !strings.HasPrefix(lines[0], ">   ▾ 開き直してください") || !strings.Contains(lines[1], "手順その一") || len(a.pin(pv, a.t.h, "").rows) == 0 {
		t.Errorf("open:\n%s", strings.Join(lines, "\n"))
	}
	// Too tall for a third of 15 rows: it heads the page instead.
	a.t.h = 15
	lines, _ = screen(t, a)
	if !a.unpinned["n"] || len(a.pin(pv, a.t.h, "").rows) != 0 || !strings.Contains(strings.Join(lines, "\n"), "手順その一") {
		t.Errorf("unpinned %v:\n%s", a.unpinned, strings.Join(lines, "\n"))
	}
	a.handle(key{name: "enter"})
	lines, _ = screen(t, a)
	if a.unpinned["n"] || !strings.HasPrefix(lines[0], ">   ▸ ") {
		t.Errorf("closed again:\n%s", strings.Join(lines, "\n"))
	}
	// One that does not open keeps ! at the head, in the column of an
	// Item's mark.
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n\">開き直してください</Notice>\n<Item id=\"a\">項目</Item>\n"), 0o644)
	a.reload(pv)
	lines, _ = screen(t, a)
	if !strings.HasPrefix(lines[0], ">   ! 開き直してください") || strings.Contains(lines[0], "▸") || !strings.Contains(strings.Join(lines, "\n"), "    · 項目") {
		t.Errorf("not opening:\n%s", strings.Join(lines, "\n"))
	}
}

// A notice's at: faint at the right end of its title, the time alone when
// it is today; the state of a comment goes left of it. A change of at alone
// keeps a hidden notice hidden.
func TestNoticeAt(t *testing.T) {
	prev := clock
	clock = func() time.Time { return time.Date(2026, 11, 23, 23, 0, 0, 0, time.Local) }
	t.Cleanup(func() { clock = prev })
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n\" at=\"2026-11-23 21:46\">開き直してください</Notice>\n<Notice id=\"m\" at=\"2026-11-22 09:05\">"+strings.Repeat("とても長いお知らせ", 8)+"</Notice>\n<Item id=\"a\">項目</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w = 60
	ls, _ := screen(t, a)
	if !strings.HasSuffix(strings.TrimRight(ls[0], " "), "21:46") || strings.Contains(ls[0], "11/23") {
		t.Errorf("today:\n%s", strings.Join(ls, "\n"))
	}
	if i := rowOf(ls, "11/22 09:05"); i < 0 || !strings.Contains(ls[i], "とても長い") || !strings.Contains(ls[i+1], "とても長い") {
		t.Errorf("another day, the title wrapped beside it:\n%s", strings.Join(ls, "\n"))
	}
	st.Append(board.Event{Op: "thought", ID: "n", Title: "開き直してください", Text: "了解"})
	a.loadEvents()
	ls, _ = screen(t, a)
	if l := ls[0]; !strings.Contains(l, "未送信  21:46") {
		t.Errorf("with a comment: %q", l)
	}
	// Hidden, then put up again with only a new time: still hidden.
	n := report.ByID(a.top().nodes, "n")
	st.Append(board.Event{Op: "hide", Key: report.HideKey(n), Title: n.Title()})
	a.loadEvents()
	os.WriteFile(st.ReportPath(), []byte("<Notice id=\"n\" at=\"2026-11-23 22:30\">開き直してください</Notice>\n<Item id=\"a\">項目</Item>\n"), 0o644)
	a.reload(a.top())
	if ls, _ = screen(t, a); rowOf(ls, "開き直して") >= 0 {
		t.Errorf("shown again by a new time:\n%s", strings.Join(ls, "\n"))
	}
}
