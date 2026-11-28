package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SPEC.md "報告の画面のカーソル": after a reload the cursor follows its id.
func TestReloadFollowsID(t *testing.T) {
	t.Parallel()
	report := "<Item id=\"a\">項目A</Item>\n<Item id=\"b\">項目B</Item>\n<Item id=\"c\">項目C</Item>\n"
	v, ss := open(t, report, 60, 20, "項目A")
	v.MoveTo("項目B")
	// Lines added above: the cursor stays on b.
	ss.write("<Item id=\"new\">新しい項目</Item>\n" + report)
	v.WaitFor("新しい項目")
	v.WaitCursorOn("項目B")
	if strings.Contains(v.Screen(), "報告が書き換わりました") {
		t.Errorf("an unchanged cursor line says the report changed")
	}
	// The cursor's line removed: it moves near and says so.
	ss.write("<Item id=\"new\">新しい項目</Item>\n<Item id=\"a\">項目A</Item>\n<Item id=\"c\">項目C</Item>\n")
	v.WaitFor("報告が書き換わりました")
	v.WaitGone("項目B")
	if r := v.CursorRow(); r < 0 {
		t.Errorf("no cursor after its line went:\n%s", v.dump())
	}
}

// SPEC.md "報告の画面のカーソル": the screen follows the cursor's line when
// a reload pushes it off screen.
func TestReloadKeepsCursorOnScreen(t *testing.T) {
	t.Parallel()
	report := "<Item id=\"a\">項目A</Item>\n<Item id=\"b\">項目B</Item>\n"
	v, ss := open(t, report, 60, 10, "項目A")
	v.MoveTo("項目B")
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("<Item id=\"x" + itoa(i) + "\">上の項目" + itoa(i) + "</Item>\n")
	}
	ss.write(b.String() + report)
	v.WaitFor("上の項目")
	v.WaitCursorOn("項目B")
}

// SPEC-format.md "check が見ること" / "解析が壊れないこと": a report that
// was never good is drawn as far as it reads, with the problems at the top.
// SPEC.md: once a good version was read, a broken one is not shown; the good
// one stays with a quiet line until a good one comes.
func TestBrokenReport(t *testing.T) {
	t.Parallel()
	v, ss := open(t, "<Item id=\"a\">項目A</Item>\n<Item id=\"b\">項目B\n", 80, 20, "項目A")
	v.WaitFor("報告に書き方の誤りがあります", "(読めた所までを出しています)", "2 行目")
	// A problem that loses nothing: no "(読めた所まで…)".
	ss.write("<Item id=\"a\" look=\"n\">項目A</Item>\n")
	v.WaitGone("(読めた所までを出しています)")
	v.WaitFor("報告に書き方の誤りがあります", "1 行目")
	// Garbage: no crash.
	ss.write("<<<\x00\x1b[31m</Item><Fold title=\"\n<Code>\n")
	v.WaitFor("報告に書き方の誤りがあります")
	ss.write("<Item id=\"a\">直った</Item>\n")
	v.WaitFor("直った")
	v.WaitGone("報告に書き方の誤りがあります")
	// Broken again: the good version stays, with the quiet line.
	ss.write("<Item id=\"a\">壊れた版</Item>\n<Item id=\"b\">項目B\n")
	v.WaitFor("AI が報告を直しています")
	wantColor(t, v, "AI が報告を直しています", "dim")
	if s := v.Screen(); !strings.Contains(s, "直った") || strings.Contains(s, "壊れた版") || strings.Contains(s, "報告に書き方の誤りがあります") {
		t.Errorf("broken again: not the good version:\n%s", v.dump())
	}
	ss.write("<Item id=\"a\">また直った</Item>\n")
	v.WaitFor("また直った")
	v.WaitGone("AI が報告を直しています")
	// Control characters from the report never reach the terminal.
	if strings.Contains(v.Raw(), "\x1b[31m</Item>") {
		t.Errorf("an escape sequence from the report reached the terminal")
	}
}

// SPEC.md "画面": what the viewer shows before there is a report.
func TestStartStates(t *testing.T) {
	t.Parallel()
	t.Run("no .pitboard above", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		v := s.view(100, 10)
		v.WaitForJoined("このディレクトリと親に pitboard の置き場 (.pitboard/) がありません。AI を動かしているディレクトリで開いてください")
		v.WaitFor("~/proj") // where it looks, home as ~
		wantColor(t, v, "~/proj", "dim")
	})
	t.Run(".pitboard without sessions, then one appears", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		if err := os.MkdirAll(filepath.Join(s.dir, ".pitboard"), 0o755); err != nil {
			t.Fatal(err)
		}
		v := s.view(100, 10)
		v.WaitForJoined("pitboard にはまだ報告がありません。AI が報告を書くと、ここに出ます")
		ss := s.session("abcdef12345")
		ss.write("<Item id=\"a\">現れた</Item>\n")
		v.WaitFor("現れた")
	})
	t.Run("a session without report.md, then it is written", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		v := ss.view(100, 10)
		v.WaitFor("AI はまだ報告を書いていません")
		ss.write("<Item id=\"a\">書かれた</Item>\n")
		v.WaitFor("書かれた")
	})
	t.Run("-s of a session that does not exist", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		s.session("other")
		v := s.view(100, 10, "-s", "abx")
		v.WaitFor("セッション abx の報告はまだありません")
		v.Quit()
		if _, err := os.Stat(filepath.Join(s.dir, ".pitboard", "abx")); !os.IsNotExist(err) {
			t.Errorf("the viewer made the session directory")
		}
	})
	t.Run("without -s: the most recently updated session", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		old := s.session("oldsess")
		old.write("<Item id=\"a\">古いセッション</Item>\n")
		past := mustParseTime(t, "2020-01-01T00:00:00Z")
		if err := os.Chtimes(old.report, past, past); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(old.dir, past, past); err != nil {
			t.Fatal(err)
		}
		nw := s.session("newsess")
		nw.write("<Item id=\"a\">新しいセッション</Item>\n")
		sub := filepath.Join(s.dir, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		v := s.viewIn(sub, 80, 10)
		v.WaitFor("新しいセッション")
	})
}

// SPEC.md "コマンド": a corrupt events.jsonl line breaks nothing.
func TestCorruptEventsLine(t *testing.T) {
	t.Parallel()
	v, ss := open(t, "<Item id=\"a\">項目A</Item>\n<Item id=\"b\">項目B</Item>\n", 80, 30, "項目A",
		ev(1, "thought", `"text":"前"`), `{"n":2,"op":`, "not json at all", ev(3, "thought", `"text":"後"`))
	v.Keys("l")
	v.WaitFor("前", "後")
	v.Keys("l")
	v.WaitFor("項目A")
	v.MoveTo("項目B")
	v.Keys("x")
	ss.waitRecords(t, "hide", 1)
	v.Keys("s")
	v.WaitFor("送りました")
	out := ss.mustRun("changes")
	for _, want := range []string{"thought 1: 前", "thought 3: 後", "hide (1): 項目B"} {
		if !strings.Contains(out, want) {
			t.Errorf("changes lacks %q:\n%s", want, out)
		}
	}
}

// SPEC.md "画面": every row fits the width, at any size; resize redraws.
func TestTinyAndResize(t *testing.T) {
	t.Parallel()
	// Without a Notice: see TestTinyNotice for what happens with one.
	noNotice := fullReport[strings.Index(fullReport, "<Title>"):]
	v, _ := open(t, noNotice, 20, 5, "")
	v.Until("something drawn", func() bool { return strings.TrimSpace(v.Screen()) != "" })
	v.Keys("j", "j")
	v.Sync()
	v.Keys("c")
	// "画面の高さが足りないときは、空行から先に削る (入力の行と案内を残す)"
	v.WaitFor("❯")
	v.Keys("esc")
	v.Sync()
	for i, l := range v.Lines() {
		if colOf(l) > 20 {
			t.Errorf("row %d is %d wide: %q", i, colOf(l), l)
		}
	}
	// The cursor's line stays on screen however low it is.
	if v.CursorRow() < 0 {
		t.Errorf("no cursor at 20x5:\n%s", v.dump())
	}
	v.Resize(80, 30)
	v.WaitFor("pitboard を作る")
	if v.CursorRow() < 0 {
		t.Errorf("no cursor after resize:\n%s", v.dump())
	}
	v.Resize(30, 8)
	v.Until("redrawn narrow", func() bool {
		for _, l := range v.Lines() {
			if colOf(l) > 30 {
				return false
			}
		}
		return v.CursorRow() >= 0
	})
}

// A huge report renders and moves without trouble.
func TestHugeReport(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("<Section title=\"大きい\">\n")
	for i := 0; i < 3000; i++ {
		b.WriteString("  <Item id=\"i" + itoa(i) + "\" status=\"状態\">項目 " + itoa(i) + " の題をすこし長めに書いておく</Item>\n")
	}
	b.WriteString("</Section>\n")
	v, ss := open(t, b.String(), 80, 24, "項目 0 の題")
	for i := 0; i < 30; i++ {
		v.Keys("pgdn")
	}
	v.Until("scrolled far", func() bool { return !strings.Contains(v.Screen(), "項目 0 の題") })
	v.Keys("g", "a")
	v.WaitFor("項目 0 の題")
	ss.write(strings.Replace(b.String(), "項目 0 の題", "項目 0 の新しい題", 1))
	v.WaitFor("項目 0 の新しい題")
}

// SPEC.md "作り": SIGTERM gives the terminal back and exits 128+15.
func TestSignalRestoresTerminal(t *testing.T) {
	t.Parallel()
	v, _ := open(t, "<Item id=\"a\">項目A</Item>\n", 60, 10, "項目A")
	v.Signal(syscall.SIGTERM)
	if code := v.WaitExit(); code != 128+int(syscall.SIGTERM) {
		t.Errorf("exit %d, want %d", code, 128+int(syscall.SIGTERM))
	}
	raw := v.Raw()
	for _, seq := range []string{"\x1b[?1049l", "\x1b[?25h", "\x1b[?2004l", "\x1b[<u", "\x1b[>4;0m"} {
		if !strings.Contains(raw, seq) {
			t.Errorf("terminal not restored: %q missing", seq)
		}
	}
	// And q quits with 0.
	v2, _ := open(t, "<Item id=\"a\">項目A</Item>\n", 60, 10, "項目A")
	v2.Keys("q")
	if code := v2.WaitExit(); code != 0 {
		t.Errorf("q: exit %d", code)
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

// SPEC-format.md "描き方": a Notice title wraps whole while it fits the
// pinned area (a third of the height); a Notice taller than that is
// shortened with …, its right-end word dropped when it has no row. SPEC.md
// "エディタの置き場所": the input row and the guide stay on a low screen.
func TestTinyNotice(t *testing.T) {
	t.Parallel()
	report := "<Notice id=\"restart\" at=\"2020-01-02 03:04\">pitboard を開き直してください</Notice>\n<Item id=\"a\">項目A</Item>\n"
	v, _ := open(t, report, 20, 30, "pitboard")
	v.WaitForJoined("pitboard を開き直してください")
	v.Resize(20, 5)
	v.Until("the title shortened to one row", func() bool {
		l := v.Line(0)
		return strings.Contains(l, "pitboard") && strings.HasSuffix(strings.TrimSpace(l), "…") && !strings.Contains(v.Line(1), "03:0")
	})
	v.Keys("c")
	v.WaitFor("❯")
	v.WaitForJoined("enter 置く")
}
