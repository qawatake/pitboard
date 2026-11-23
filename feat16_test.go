package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

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
	var report strings.Builder
	report.WriteString("<Title>" + titles[1] + "</Title>\n\n<Section title=\"s\">\n")
	i := 0
	for _, ti := range titles {
		for _, st := range statuses {
			attr := ""
			if st != "" {
				attr = fmt.Sprintf(" status=%q", st)
			}
			fmt.Fprintf(&report, "<Item id=\"i%d\"%s>%s</Item>\n", i, attr, ti)
			fmt.Fprintf(&report, "<Item id=\"d%d\" state=\"done\"%s>%s</Item>\n", i, attr, ti)
			i++
		}
	}
	fmt.Fprintf(&report, "<Link id=\"l\" path=\"x\">%s</Link>\n", titles[3])
	report.WriteString("- " + titles[1] + "\n</Section>\n")
	for w := 20; w <= 200; w += 3 {
		st := testStore(t)
		os.WriteFile(st.reportPath(), []byte(report.String()), 0o644)
		st.Append(Event{Op: "thought", ID: "i1", Text: titles[3]})
		st.Append(Event{Op: "thought", Text: titles[1]})
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
	os.WriteFile(st.reportPath(), []byte("<Item id=\"a\" status=\"順番待ち\">"+long+"</Item>\n<Item id=\"b\">次</Item>\n"), 0o644)
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
	if a.canOpen(pv, byID(pv.nodes, "a")) {
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
	os.WriteFile(st.reportPath(), []byte(b.String()), 0o644)
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

const noticeReport = `<Notice id="restart">pitboard を開き直してください。新しい版を入れました</Notice>
<Title>題</Title>

<Section title="s">
<Item id="a" ask="y">項目 a</Item>
</Section>
`

func TestNoticeParse(t *testing.T) {
	nodes, probs, _ := parseReport(noticeReport)
	if len(probs) != 0 || nodes[0].kind != kNotice || nodes[0].text != "pitboard を開き直してください。新しい版を入れました" {
		t.Fatalf("%v %+v", probs, nodes[0])
	}
	cases := []struct{ src, want string }{
		{"<Notice>x</Notice>", "needs id"},
		{"<Section title=\"s\">\n<Notice id=\"n\">x</Notice>\n</Section>", "cannot be inside <Section>"},
		{"<Fold title=\"f\">\n<Notice id=\"n\">x</Notice>\n</Fold>", "cannot be inside <Fold>"},
		{"<Notice id=\"a\">a</Notice>\n<Notice id=\"b\">b</Notice>\n<Notice id=\"c\">c</Notice>\n<Notice id=\"d\">d</Notice>", "at most 3"},
	}
	for _, c := range cases {
		_, probs, _ := parseReport(c.src)
		if len(probs) != 1 || !strings.Contains(probs[0].Msg, c.want) {
			t.Errorf("%q: %v", c.src, probs)
		}
	}
	// A paragraph on several lines is joined.
	nodes, probs, _ = parseReport("<Notice id=\"n\">\n  開き直して\n  ください\n</Notice>")
	if len(probs) != 0 || nodes[0].text != "開き直してください" {
		t.Errorf("%v %q", probs, nodes[0].text)
	}
}

func TestNoticeFixedAtTop(t *testing.T) {
	st := testStore(t)
	var b strings.Builder
	b.WriteString(noticeReport)
	for i := range 60 {
		fmt.Fprintf(&b, "<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.reportPath(), []byte(b.String()), 0o644)
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
	os.WriteFile(st.reportPath(), []byte(strings.Replace(b.String(), "新しい版を入れました", "もう一度入れ直しました", 1)), 0o644)
	os.Chtimes(st.reportPath(), time.Now().Add(time.Second), time.Now().Add(time.Second))
	a.reload(pv)
	lines, _ = screen(t, a)
	if !strings.Contains(lines[0], "もう一度入れ直しました") {
		t.Errorf("rewritten notice not back: %q", lines[0])
	}
}

func TestNoticeWrapsToThreeRows(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte("<Notice id=\"n\">"+strings.Repeat("長いお知らせ", 40)+"</Notice>\n<Item id=\"a\">a</Item>\n"), 0o644)
	a := testApp(t, st)
	a.t.w = 40
	rows := a.noticeRows(a.top())
	if len(rows) != noticeMax+1 || !strings.HasSuffix(reEsc.ReplaceAllString(rows[2].line, ""), "…") {
		t.Errorf("%d rows: %q", len(rows), reEsc.ReplaceAllString(rows[len(rows)-1].line, ""))
	}
}
