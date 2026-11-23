package viewer

import (
	"os"
	"strings"
	"testing"
	"time"
)

// c on a Fold writes a comment on it: the editor right under its title
// (open or not), the record by its id or else its title, the state at the
// right end, and the comments under the title when it is open.
func TestCommentOnAFold(t *testing.T) {
	for _, attr := range []string{``, ` id="done"`} {
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Fold"+attr+" title=\"済んだこと (2)\">\n<Item id=\"a\" look=\"y\">項目 a</Item>\n<Item id=\"b\">項目 b</Item>\n</Fold>\n<Item id=\"c\">項目 c</Item>\n"), 0o644)
		a := testApp(t, st)
		wait(t, time.Hour)
		a.render()
		pv := a.top()
		a.handle(key{r: 'k'}) // pointed at by the person
		a.render()
		if n := pv.nodeAt(pv.cur); n == nil || n.Title() != "済んだこと (2)" {
			t.Fatalf("%q: cursor %q", attr, pv.cur)
		}
		if s := hintLine(a); !strings.Contains(s, "c 行に書く") || strings.Contains(s, "y よし") {
			t.Errorf("%q: hints %q", attr, s)
		}
		a.handle(key{name: "enter"}) // open: the editor still goes under the title
		a.render()
		a.handle(key{r: 'c'})
		if a.in == nil {
			t.Fatalf("%q: c did not open the editor", attr)
		}
		a.in.ed = newEdit("これは要る？")
		ls, _ := screen(t, a)
		title := rowOf(ls, "済んだこと (2)")
		if title < 0 || strings.Trim(ls[title+2], " ─") != "" || strings.TrimSpace(ls[title+3]) != "❯ これは要る？" || rowOf(ls, "項目 a") < title+3 {
			t.Errorf("%q: the editor is not under the title:\n%s", attr, strings.Join(ls, "\n"))
		}
		a.handle(key{name: "enter"})
		evs := st.Events()
		want := "Fold|title=済んだこと (2)"
		if attr != "" {
			want = "done"
		}
		if len(evs) != 1 || evs[0].Op != "thought" || evs[0].ID != want || evs[0].Title != "済んだこと (2)" {
			t.Fatalf("%q: events %+v", attr, evs)
		}
		// The green mark of what it holds goes, as with r.
		if m := a.markOfLine(pv, pv.nodeAt(pv.cur)); m != markNone {
			t.Errorf("%q: mark %d after the comment", attr, m)
		}
		ls, _ = screen(t, a)
		title = rowOf(ls, "済んだこと (2)")
		if !strings.Contains(ls[title], "未送信") || !strings.Contains(ls[title+1], "✎ これは要る？") || rowOf(ls, "項目 a") != title+2 {
			t.Errorf("%q: open fold:\n%s", attr, strings.Join(ls, "\n"))
		}
		// Closed: only the state at the right end.
		a.handle(key{name: "enter"})
		ls, _ = screen(t, a)
		if rowOf(ls, "✎") >= 0 || !strings.Contains(ls[rowOf(ls, "済んだこと (2)")], "未送信") {
			t.Errorf("%q: closed fold:\n%s", attr, strings.Join(ls, "\n"))
		}
		// The pane says which line it is on; u takes it back.
		if d := plainLines(func() []string {
			var out []string
			for _, r := range a.thoughtDetails(a.state.Thoughts[0], 0) {
				out = append(out, r.line)
			}
			return out
		}()); !strings.Contains(d, "行: 済んだこと (2)") {
			t.Errorf("%q: details %q", attr, d)
		}
		a.handle(key{r: 'u'})
		if len(a.state.Thoughts) != 0 {
			t.Errorf("%q: u left %d thoughts", attr, len(a.state.Thoughts))
		}
	}
}

// The title of the report is never the cursor's line, whatever happens to
// the notices above it (hidden with x, opened too tall to pin, gone from
// the report), and a title that wraps keeps its second row under its text.
func TestTitleNeverTheCursor(t *testing.T) {
	notice := "<Notice id=\"n\">開き直してください\n" + strings.Repeat("  本文の行。\n\n", 8) + "</Notice>\n"
	page := "<Title>AI セッションの横に置く pitboard を作る</Title>\n\n<Item id=\"a\">項目 a</Item>\n"
	check := func(step string, a *app) {
		t.Helper()
		ls, _ := screen(t, a)
		for i, l := range ls {
			if strings.HasPrefix(l, ">") && strings.Contains(l, "AI セッション") {
				t.Errorf("%s: the title has the cursor:\n%s", step, strings.Join(ls, "\n"))
			}
			if strings.Contains(l, "AI セッションの横に置く") && i+1 < len(ls) && strings.Contains(ls[i+1], "pitboard を作る") {
				if strings.Index(ls[i+1], "pitboard") != strings.Index(l, "AI") {
					t.Errorf("%s: the second row is not under the first:\n%s", step, strings.Join(ls, "\n"))
				}
			}
		}
	}
	for _, w := range []int{40, 80} {
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte(notice+page), 0o644)
		a := testApp(t, st)
		a.t.w, a.t.h = w, 20
		wait(t, time.Hour)
		pv := a.top()
		a.render()
		a.handle(key{r: 'j'})
		a.handle(key{r: 'k'})
		check("pinned", a)
		a.handle(key{name: "enter"}) // too tall: heads the page
		check("unpinned", a)
		a.handle(key{r: 'x'})
		check("hidden", a)
		a.handle(key{r: 'u'})
		a.handle(key{r: 'k'})
		check("back", a)
		os.WriteFile(st.ReportPath(), []byte(page), 0o644)
		a.reload(pv)
		check("gone", a)
	}
}

// What a Fold holds goes 2 past the first character of its title (its
// marks under the title's text, 2 further), a Fold in a Fold as much
// again; text lines up with the titles of the Items in it. Narrow screens
// keep it.
func TestFoldIndent(t *testing.T) {
	src := "<Fold title=\"見本\">\n<Item id=\"a\">項目 a\n  中身の文\n</Item>\n書き出す CSV の行\n- 箇条\n<Ask id=\"q\">問い\n<Option key=\"a\">選ぶ</Option>\n<Option key=\"b\">選ばない</Option>\n</Ask>\n<Fold title=\"内側\">\n<Item id=\"b\">項目 b</Item>\n</Fold>\n</Fold>\n"
	for _, w := range []int{80, 40} {
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte(src), 0o644)
		a := testApp(t, st)
		a.t.w = w
		pv := a.top()
		for _, k := range []string{"fold:1", "a", "q", "fold:2"} {
			pv.open[k] = true
		}
		ls, _ := screen(t, a)
		col := func(s string) int {
			i := rowOf(ls, s)
			if i < 0 {
				t.Fatalf("w %d: no %q:\n%s", w, s, strings.Join(ls, "\n"))
			}
			return width(ls[i][:strings.Index(ls[i], s)])
		}
		title := col("見本")
		checks := []struct {
			s    string
			want int
		}{
			{"▾ 項目 a", title + 2},
			{"中身の文", title + 4 + 2},
			{"書き出す CSV の行", title + 4},
			{"箇条", title + 4},
			{"? 問い", title + 2},
			{"a  選ぶ", title + 4 + 2},
			{"▾ 内側", title + 2},
			{"項目 b", title + 4 + 4},
		}
		for _, c := range checks {
			if got := col(c.s); got != c.want {
				t.Errorf("w %d: %q at %d, want %d:\n%s", w, c.s, got, c.want, strings.Join(ls, "\n"))
			}
		}
		for _, l := range ls {
			if visibleWidth(l) > w {
				t.Errorf("w %d: too wide %q", w, l)
			}
		}
	}
}
