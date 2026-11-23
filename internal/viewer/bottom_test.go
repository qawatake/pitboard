package viewer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Every word on screen follows the word list.
func TestNoOldWords(t *testing.T) {
	st := testStore(t)
	st.Append(board.Event{Op: "thought", ID: "hook", Text: "x"})
	a := testApp(t, st)
	a.render()
	var all []string
	for _, r := range a.helpRows() {
		all = append(all, r.line)
	}
	a.in = newInput("", "", "コメントを書く")
	all = append(all, a.editorLines()...)
	a.in = nil
	a.handle(key{name: "tab"})
	a.lopen[1] = true
	for _, r := range a.listRows() {
		all = append(all, r.line)
	}
	// The screens without a src, and the editor's file.
	all = append(all, msgChanged, msgMoved, msgNoReport, msgNoBoard, msgNoSession)
	for _, setup := range []func(b *app){
		func(b *app) { b.setStore(nil) },
		func(b *app) { os.Mkdir(filepath.Join(b.base, ".pitboard"), 0o755); b.setStore(nil) },
		func(b *app) { b.setStore(board.Open(b.base, "nosuch")) },
	} {
		b := &app{t: &term{w: 120, h: 30, out: io.Discard}, base: t.TempDir(), lopen: map[int]bool{}}
		setup(b)
		all = append(all, shown(b))
	}
	s := reEsc.ReplaceAllString(strings.Join(all, "\n"), "")
	if !strings.Contains(s, "セッション nosuch") {
		t.Errorf("the missing session's screen was not drawn:\n%s", s)
	}
	for _, old := range []string{"板", "思いつき", "送信済み", "新着", "受け取った", "行き先", "ページ", "j k ↑↓  送る", "半画面ずつ送る", "ターミナル"} {
		if strings.Contains(s, old) {
			t.Errorf("old word %q on screen:\n%s", old, s)
		}
	}
}

func TestUnsentMark(t *testing.T) {
	noSendTarget(t)
	st := testStore(t)
	title := strings.Repeat("とても長い題", 12)
	src := "<Title>" + title + "</Title>\n\n<Section title=\"s\">\n"
	for i := range 40 {
		src += fmt.Sprintf("<Item id=\"i%d\">項目 %d</Item>\n", i, i)
	}
	os.WriteFile(st.ReportPath(), []byte(src+"</Section>\n"), 0o644)
	a := testApp(t, st)
	if ls, _ := screen(t, a); strings.Contains(strings.Join(ls, "\n"), "未送信") {
		t.Error("nothing unsent, no mark")
	}
	st.Append(board.Event{Op: "thought", Text: "a"})
	st.Append(board.Event{Op: "react", ID: "i1", Value: "y"})
	st.Append(board.Event{Op: "hide", Key: "k", Title: "x"})
	a.loadEvents()
	// The top line is the blank padding: the mark goes on its right.
	ls, _ := screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") || visibleWidth(ls[0]) != lgutter+a.textWidth() {
		t.Errorf("top line %q", ls[0])
	}
	// Scrolled so the title is on top: the mark keeps a line of its own,
	// and the title is not cut.
	a.top().top = 1
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") || strings.TrimSpace(strings.TrimSuffix(ls[0], "● 未送信 3")) != "" || !strings.Contains(ls[1], "とても長い題") || strings.Contains(ls[1], "…") || visibleWidth(ls[0]) > lgutter+a.textWidth() {
		t.Errorf("title line %q / %q", ls[0], ls[1])
	}
	// Scrolled further: still on the top line.
	a.top().top = 10
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") {
		t.Errorf("scrolled: %q", ls[0])
	}
	// The thoughts list shows it too; the editor's text is not counted.
	a.list = true
	a.in = &input{label: "x", ed: newEdit("書きかけ")}
	ls, _ = screen(t, a)
	if !strings.HasSuffix(ls[0], "● 未送信 3") {
		t.Errorf("list: %q", ls[0])
	}
	a.in, a.list = nil, false
	// s clears it: what was sent waits, faint, until the AI reads it.
	t.Setenv("PITBOARD_SEND", "true")
	a.handle(key{r: 's'})
	ls, _ = screen(t, a)
	if strings.Contains(ls[0], "未送信") || !strings.HasSuffix(ls[0], "送信済 3") {
		t.Errorf("after s: %q", ls[0])
	}
	// Something new after the send: both.
	st.Append(board.Event{Op: "thought", Text: "b"})
	a.loadEvents()
	if ls, _ := screen(t, a); !strings.HasSuffix(ls[0], "送信済 3  ● 未送信 1") {
		t.Errorf("sent and unsent: %q", ls[0])
	}
	st.Changes()
	a.loadEvents()
	if ls, _ := screen(t, a); strings.Contains(ls[0], "送信済") || !strings.HasSuffix(ls[0], "● 未送信 1") {
		t.Errorf("after changes: %q", ls[0])
	}
}

// ---- the m hint ----

func TestHintsShowM(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.render()
	a.handle(key{r: 'j'}) // the Item
	a.render()
	s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	if s != "      y よし  n だめ  c 行に書く  x 隠す  space 次へ  m 書く  ? キー" {
		t.Errorf("hints: %q", s)
	}
	// Dropped in order x, enter, n, y, c; m and ? stay.
	for _, c := range []struct {
		w    int
		want string
	}{
		{54, "      y よし  n だめ  c 行に書く  m 書く  ? キー"},
		{48, "      y よし  c 行に書く  m 書く  ? キー"},
		{40, "      c 行に書く  m 書く  ? キー"},
		{26, "      m 書く  ? キー"},
	} {
		a.t.w = c.w
		if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); s != c.want {
			t.Errorf("width %d: %q, want %q", c.w, s, c.want)
		}
	}
	a.t.w = 80
	a.list = true
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "m 書く  ? キー") {
		t.Errorf("list hints: %q", s)
	}
}

func hintLine(a *app) string { return plainLines(a.bottom()) }

// ---- one-line messages and hints ----

func TestMessagesWrap(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.t.w = 50
	a.msg = "送れませんでした: PITBOARD_SEND: exit status 2: " + strings.Repeat("とても長いエラー ", 10)
	b := a.bottom()
	if len(b) < 2 || strings.Contains(plainLines(b), "…") || !strings.HasSuffix(plainLines(b), "エラー") {
		t.Errorf("message: %q", plainLines(b))
	}
	for _, l := range b {
		if visibleWidth(l) > lgutter+a.textWidth() {
			t.Errorf("too wide: %q", l)
		}
	}
	a.msg = "短い"
	if b := a.bottom(); len(b) != 1 {
		t.Errorf("short message: %q", plainLines(b))
	}
}

func TestEditorHintsKeepEsc(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.in = &input{label: "x"}
	for _, c := range []struct {
		w    int
		want string
	}{
		{82, "      enter 置く  esc やめる  ctrl+j 改行  ctrl+g エディタ"},
		{46, "      enter 置く  esc やめる  ctrl+j 改行"},
		{32, "      enter 置く  esc やめる"},
	} {
		a.t.w = c.w
		if s := plainLines(a.editorHints()); s != c.want {
			t.Errorf("width %d: %q, want %q", c.w, s, c.want)
		}
	}
}

// ---- no board ----

func TestNoBoardMessages(t *testing.T) {
	dir := t.TempDir()
	a := &app{t: &term{w: 120, h: 30}, base: dir, lopen: map[int]bool{}}
	a.setStore(nil)
	if s := shown(a); !strings.Contains(s, "このディレクトリと親に pitboard の置き場 (.pitboard/) がありません") {
		t.Errorf("no .pitboard:\n%s", s)
	}
	if s := hintLine(a); !strings.Contains(s, filepath.Base(dir)) {
		t.Errorf("the place is not shown: %q", s)
	}
	os.Mkdir(filepath.Join(dir, ".pitboard"), 0o755)
	if s := shown(a); !strings.Contains(s, "pitboard にはまだ報告がありません。AI が報告を書くと、ここに出ます") {
		t.Errorf("empty .pitboard:\n%s", s)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if got := homeTilde(filepath.Join(home, "x", "y")); got != filepath.Join("~", "x", "y") {
			t.Errorf("homeTilde = %q", got)
		}
		if got := homeTilde(home + "z"); got != home+"z" {
			t.Errorf("homeTilde on a sibling = %q", got)
		}
	}
}

func TestHelpScrolls(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	a.t.w, a.t.h = 90, 10
	a.handle(key{r: '?'})
	a.render()
	a.handle(key{r: 'y'}) // not a closing key
	if !a.help {
		t.Fatal("y closed the list of keys")
	}
	for range 80 {
		a.handle(key{r: 'j'})
		a.render()
	}
	rows := a.helpRows()
	if a.htop != len(rows)-(a.t.h-1) {
		t.Errorf("htop %d, rows %d", a.htop, len(rows))
	}
	a.handle(key{name: "esc"})
	if a.help {
		t.Error("esc must close the list")
	}
}

func TestNarrowHintsKeepLineKeys(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(`<Item id="hook">hook を入れるか決める
中身
</Item>`), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = 38, 10
	a.render()
	if s := strings.Join(a.bottom(), "\n"); !strings.Contains(reEsc.ReplaceAllString(s, ""), "j k") {
		t.Errorf("placed cursor, narrow: %q", reEsc.ReplaceAllString(s, ""))
	}
	a.handle(key{r: 'j'})
	a.render()
	s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	// One line: x, enter, n, y go first; c, "m 書く" and "? キー" stay.
	if s != "      c 行に書く  m 書く  ? キー" {
		t.Errorf("narrow hints: %q", s)
	}
	a.t.w = 100
	a.render()
	s = reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), "")
	if !strings.HasPrefix(s, "      enter 開く  y よし  n だめ  c 行に書く  x 隠す  m 書く  ? キー") || strings.Contains(s, "tab") || strings.Contains(s, "\n") {
		t.Errorf("wide hints: %q", s)
	}
	a.t.w = 20
	a.handle(key{r: 'j'})
	st.Append(board.Event{Op: "thought", Text: "x"})
	a.loadEvents()
	a.top().placed = true
	a.render()
	if s := reEsc.ReplaceAllString(strings.Join(a.bottom(), "\n"), ""); !strings.Contains(s, "j k") {
		t.Errorf("width 20, placed: %q", s)
	}
}

// Every letter the viewer does not keep for itself must answer an Ask, and
// every letter it keeps must be refused by check: when the viewer takes a
// new key, viewerKeys has to follow, or this fails.
func TestViewerKeysMatchTheViewer(t *testing.T) {
	for r := 'a'; r <= 'z'; r++ {
		k := string(r)
		other := "a"
		if k == "a" {
			other = "b"
		}
		src := "<Ask id=\"q\">q\n<Option key=\"" + k + "\">x</Option>\n<Option key=\"" + other + "\">y</Option>\n</Ask>\n"
		_, probs, _ := report.Parse(src)
		if strings.Contains(report.ViewerKeys, k) {
			if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
				t.Errorf("%s: check must refuse a viewer key: %v", k, probs)
			}
			continue
		}
		if len(probs) != 0 {
			t.Errorf("%s: %v", k, probs)
			continue
		}
		st := testStore(t)
		os.WriteFile(st.ReportPath(), []byte(src), 0o644)
		a := testApp(t, st)
		pv := a.top()
		a.render()
		a.handle(key{r: 'j'})
		a.handle(key{r: 'k'})
		a.render()
		a.handle(key{name: "enter"})
		a.render()
		if !pv.open["q"] {
			t.Fatalf("%s: the Ask did not open", k)
		}
		a.handle(key{r: r})
		if evs := st.Events(); len(evs) != 1 || evs[0].Value != k {
			t.Errorf("%s is not a viewer key, but pressing it did not answer: %+v (add it to viewerKeys?)", k, evs)
		}
	}
}

// The keys list says how to reach the notices, and that y / n work on
// them.
func TestHelpTellsNotices(t *testing.T) {
	a := testApp(t, testStore(t))
	var all []string
	for _, r := range a.helpRows() {
		all = append(all, reEsc.ReplaceAllString(r.line, ""))
	}
	s := strings.Join(all, "\n")
	if !strings.Contains(s, "固定されたお知らせへ") || !strings.Contains(s, "よし (項目・リンク・お知らせ)") {
		t.Errorf("help:\n%s", s)
	}
}
