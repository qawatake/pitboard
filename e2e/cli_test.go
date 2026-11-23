package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ev builds one events.jsonl line in the documented format (SPEC.md
// "人の操作 (events.jsonl に 1 行ずつ)").
func ev(n int, op string, fields string) string {
	if fields != "" {
		fields = "," + fields
	}
	return fmt.Sprintf(`{"n":%d,"t":"2026-01-01T00:00:00Z","op":%q%s}`, n, op, fields)
}

func wantLines(t *testing.T, got string, want ...string) {
	t.Helper()
	g := strings.TrimRight(got, "\n")
	w := strings.Join(want, "\n")
	if g != w {
		t.Errorf("got:\n%s\nwant:\n%s", g, w)
	}
}

// SPEC.md "コマンド": -h is the whole usage and names every command.
func TestHelpMentionsEveryCommand(t *testing.T) {
	s := newSandbox(t)
	out := s.mustRun("-h")
	for _, cmd := range []string{"view", "path", "changes", "settle", "open", "check", "wait", "hook", "--skill", "-h",
		"prompt-submit", "post-tool", "stop", "PITBOARD_SEND", "A layout to start from", "Manners"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("-h does not mention %q", cmd)
		}
	}
}

// SPEC.md "組み込み skill": --skill prints skills/pitboard/SKILL.md.
func TestSkillPrintsSKILLmd(t *testing.T) {
	s := newSandbox(t)
	_, self, _, _ := runtime.Caller(0)
	want, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(self)), "skills", "pitboard", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := s.mustRun("--skill")
	if got != string(want) {
		t.Errorf("--skill differs from SKILL.md:\n%s", got)
	}
	if !strings.Contains(got, "name: pitboard") || !strings.Contains(got, "pitboard -h") {
		t.Errorf("SKILL.md must name the skill and point at pitboard -h:\n%s", got)
	}
}

// SPEC-format.md "置き場所": path makes the session directory and prints
// report.md's path; found from the current directory upwards.
func TestPath(t *testing.T) {
	s := newSandbox(t)
	out := strings.TrimSpace(s.mustRun("-s", "abcdef12345", "path"))
	want := filepath.Join(s.dir, ".pitboard", "abcdef12345", "report.md")
	if out != want {
		t.Fatalf("path = %q, want %q", out, want)
	}
	if fi, err := os.Stat(filepath.Dir(want)); err != nil || !fi.IsDir() {
		t.Fatalf("path did not make the session directory: %v", err)
	}
	if _, err := os.Stat(want); err == nil {
		t.Errorf("path must not write report.md itself")
	}

	t.Run("from a subdirectory finds the board above", func(t *testing.T) {
		sub := filepath.Join(s.dir, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		r := s.runIn(sub, "", "-s", "abcdef12345", "path")
		if strings.TrimSpace(r.stdout) != want {
			t.Errorf("path from a subdirectory = %q, want %q", r.stdout, want)
		}
		r = s.runIn(sub, "", "-s", "newsess", "path")
		if got := strings.TrimSpace(r.stdout); got != filepath.Join(s.dir, ".pitboard", "newsess", "report.md") {
			t.Errorf("a new session goes in the nearest .pitboard above: %q", got)
		}
	})

	t.Run("no .pitboard above: made here", func(t *testing.T) {
		s := newSandbox(t)
		other := filepath.Join(s.home, "other")
		if err := os.MkdirAll(other, 0o755); err != nil {
			t.Fatal(err)
		}
		r := s.runIn(other, "", "-s", "x1", "path")
		if got := strings.TrimSpace(r.stdout); got != filepath.Join(other, ".pitboard", "x1", "report.md") {
			t.Errorf("path = %q", got)
		}
	})

	t.Run("never above the home directory", func(t *testing.T) {
		s := newSandbox(t)
		above := filepath.Dir(s.home)
		if err := os.MkdirAll(filepath.Join(above, ".pitboard", "x1"), 0o755); err != nil {
			t.Fatal(err)
		}
		r := s.run("-s", "x1", "path")
		if got := strings.TrimSpace(r.stdout); got != filepath.Join(s.dir, ".pitboard", "x1", "report.md") {
			t.Errorf("path used a board above HOME: %q", got)
		}
	})
}

// SPEC-format.md "置き場所": sessions by -s or $PITBOARD_SESSION,
// [A-Za-z0-9_-]+, unique prefixes; -s right before or after the command.
func TestSessionRules(t *testing.T) {
	s := newSandbox(t)
	full := strings.TrimSpace(s.mustRun("-s", "abcdef12345", "path"))
	s.mustRun("-s", "abxyz", "path")

	tests := []struct {
		name string
		env  string
		args []string
		ok   bool
		out  string
	}{
		{"unique prefix", "", []string{"-s", "abc", "path"}, true, full},
		{"-s after the command", "", []string{"path", "-s", "abcd"}, true, full},
		{"$PITBOARD_SESSION", "abcdef12345", []string{"path"}, true, full},
		{"no session", "", []string{"changes"}, false, "no session"},
		{"bad characters", "", []string{"-s", "a b", "path"}, false, "A-Z a-z 0-9 _ -"},
		{"ambiguous prefix", "", []string{"-s", "ab", "changes"}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.env["PITBOARD_SESSION"] = tt.env
			defer delete(s.env, "PITBOARD_SESSION")
			r := s.run(tt.args...)
			if tt.ok != (r.code == 0) {
				t.Fatalf("exit %d, stdout %q stderr %q", r.code, r.stdout, r.stderr)
			}
			if tt.out != "" && !strings.Contains(r.stdout+r.stderr, tt.out) {
				t.Errorf("output %q %q does not contain %q", r.stdout, r.stderr, tt.out)
			}
		})
	}

	t.Run("-s inside TEXT stays text", func(t *testing.T) {
		ss := &session{s: s, id: "abcdef12345", report: full, dir: filepath.Dir(full)}
		ss.appendEvents(ev(1, "thought", `"text":"hi"`), ev(2, "send", ""))
		ss.mustRun("changes")
		s.mustRun("settle", "-s", "abcdef", "1", "use", "-s", "here")
		if !strings.Contains(ss.events(), `"text":"use -s here"`) {
			t.Errorf("settle text lost -s:\n%s", ss.events())
		}
	})
}

// SPEC-format.md "check が見ること": every documented error class, with
// line numbers and exit 1.
func TestCheck(t *testing.T) {
	tests := []struct {
		name   string
		report string
		line   int
		msg    string
	}{
		{"unknown part", "<Foo>x</Foo>", 1, "unknown part"},
		{"lower-case part name", `<item id="a">x</item>`, 1, "capital letter"},
		{"unknown attribute", `<Item id="a" lang="diff">x</Item>`, 1, `no attribute "lang"`},
		{"Item without id", `<Item>x</Item>`, 1, "id"},
		{"Section without title", "<Section>\n</Section>", 1, "title"},
		{"Option without key", "<Ask id=\"q\">q\n  <Option>a</Option>\n  <Option key=\"b\">b</Option>\n</Ask>", 2, "key"},
		{"duplicate id", "<Item id=\"a\">x</Item>\n<Item id=\"a\">y</Item>", 2, "duplicate id"},
		{"bad id", `<Item id="Bad_">x</Item>`, 1, `id "Bad_"`},
		{"id too long", `<Item id="abcdefghijklmnopq">x</Item>`, 1, "id"},
		{"unclosed part", "<Section title=\"s\">\n  <Item id=\"a\">x", 2, "not closed"},
		{"Section in Section", "<Section title=\"a\">\n<Section title=\"b\">\n</Section>\n</Section>", 2, "inside"},
		{"Item in Item", "<Item id=\"a\">x\n  <Item id=\"b\">y</Item>\n</Item>", 2, "inside"},
		{"Notice not at the top", "<Section title=\"s\">\n  <Notice id=\"n\">x</Notice>\n</Section>", 2, "Notice"},
		{"four Notices", "<Notice id=\"a\">a</Notice>\n<Notice id=\"b\">b</Notice>\n<Notice id=\"c\">c</Notice>\n<Notice id=\"d\">d</Notice>", 4, "Notice"},
		{"one Option", "<Ask id=\"q\">q\n  <Option key=\"a\">a</Option>\n</Ask>", 1, "2 or more"},
		{"viewer key as Option key", "<Ask id=\"q\">q\n  <Option key=\"j\">a</Option>\n  <Option key=\"b\">b</Option>\n</Ask>", 2, "viewer key"},
		{"duplicate Option key", "<Ask id=\"q\">q\n  <Option key=\"a\">a</Option>\n  <Option key=\"a\">b</Option>\n</Ask>", 3, "key"},
		{"bad state", `<Item id="a" state="weird">x</Item>`, 1, "state"},
		{"bad ask", `<Item id="a" ask="n">x</Item>`, 1, "ask"},
		{"ask on done", `<Item id="a" state="done" ask="y">x</Item>`, 1, "ask"},
		{"bad wait", `<Item id="a" wait="me">x</Item>`, 1, "wait"},
		{"wait on done", `<Item id="a" state="done" wait="you">x</Item>`, 1, "wait"},
		{"bad look", `<Item id="a" look="n">x</Item>`, 1, "look"},
		{"bad re", `<Item id="a" re="x">x</Item>`, 1, "re"},
		{"Link with both", `<Link id="l" path="a" url="https://x">l</Link>`, 1, "exactly one"},
		{"Link with neither", `<Link id="l">l</Link>`, 1, "exactly one"},
		{"Link non-http url", `<Link id="l" url="ftp://x">l</Link>`, 1, "http"},
		{"bad at", `<Notice id="n" at="yesterday">x</Notice>`, 1, "YYYY-MM-DD HH:MM"},
		{"list indent not even", "- a\n   - b", 2, "2 spaces"},
		{"four list levels", "- a\n  - b\n    - c\n      - d", 4, "level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			ss := s.session("abcdef12345")
			ss.write(tt.report + "\n")
			r := ss.run("check")
			if r.code != 1 {
				t.Fatalf("exit %d, want 1; out %q %q", r.code, r.stdout, r.stderr)
			}
			out := r.stdout + r.stderr
			want := fmt.Sprintf("%s:%d: ", ss.report, tt.line)
			found := false
			for _, l := range strings.Split(out, "\n") {
				if strings.HasPrefix(l, want) && strings.Contains(l, tt.msg) {
					found = true
				}
			}
			if !found {
				t.Errorf("no line %q...%q in:\n%s", want, tt.msg, out)
			}
		})
	}

	t.Run("a clean report", func(t *testing.T) {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write(fullReport)
		r := ss.run("check")
		if r.code != 0 || strings.TrimSpace(r.stdout+r.stderr) != "" {
			t.Errorf("clean report: exit %d, out %q %q", r.code, r.stdout, r.stderr)
		}
	})

	t.Run("numbered nesting aligned to the text is fine", func(t *testing.T) {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write("1. a\n   - b\n")
		if r := ss.run("check"); r.code != 0 {
			t.Errorf("exit %d: %s", r.code, r.stdout+r.stderr)
		}
	})
}

// fullReport uses every part and attribute; check must accept it.
var fullReport = dedent(`
	<Notice id="restart" at="2020-01-02 03:04">pitboard を開き直してください
	  新しい版を入れました。
	</Notice>
	<Title>pitboard を作る</Title>

	inbox を広げた pitboard を作る。

	<Section title="あなたへ">
	  <Ask id="unit">ページの単位
	    <Option key="a">セッションごと</Option>
	    <Option key="b">プロジェクトごと</Option>
	  </Ask>
	  <Item id="hook" ask="y">hook を入れるか決める
	    入れると、書いたことが AI の作業中にも届く。
	  </Item>
	</Section>

	<Section title="進行中">
	  <Item id="impl" state="doing">最小版を実装する</Item>
	  <Item id="parts" status="あなた待ち" wait="you" note="上の問いの答え">部品の一覧を決める</Item>
	  <Item id="read" look="y">読んでほしい結果</Item>
	</Section>

	<Section title="見せたいもの">
	  <Link id="spec" path="docs/spec/SPEC.md">最小版の仕様</Link>
	  <Link id="pr" url="https://example.com/pull/1">プルリクエスト</Link>
	  <Code lang="diff">
	  -old
	  +new
	  </Code>
	</Section>

	<Fold title="済んだこと (2)">
	  <Item id="tui" state="done">viewer を 1 列にする</Item>
	  - id の要らない、ただの行
	</Fold>
`)

const changesReport = `<Section title="s">
  <Item id="hook" ask="y">hook を入れるか決める</Item>
  <Ask id="base">土台
    <Option key="a">A案</Option>
    <Option key="d">B案</Option>
  </Ask>
  <Item id="impl">新しい題</Item>
</Section>
`

// SPEC.md "コマンド": what changes prints.
func TestChanges(t *testing.T) {
	s := newSandbox(t)
	ss := s.session("abcdef12345")
	ss.write(changesReport)
	if err := os.MkdirAll(filepath.Join(ss.dir, "images"), 0o700); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(ss.dir, "images", "3.png")
	writeFile(t, img, "png")
	ss.appendEvents(
		ev(1, "react", `"id":"hook","value":"y","title":"hook を入れるか決める"`),
		ev(2, "react", `"id":"hook","value":"n","title":"hook を入れるか決める"`),
		ev(3, "react", `"id":"base","value":"a","title":"土台","option":"A案"`),
		ev(4, "react", `"id":"base","value":"d","title":"土台","option":"B案"`),
		ev(5, "thought", `"id":"impl","text":"もっと速く","title":"古い題"`),
		ev(6, "thought", `"text":"一行目\n二行目"`),
		ev(7, "react", `"id":"gone","value":"y","title":"消えた行"`),
		ev(8, "hide", `"key":"k1","title":"とても長い題のとても長い行をここで隠しました"`),
		ev(9, "hide", `"key":"k2","title":"短い"`),
		ev(10, "thought", `"text":"これを見て [Image #3] [Image #3]"`),
		ev(11, "unhide", `"key":"k2","title":"短い"`),
		ev(12, "send", ""),
		ev(13, "thought", `"text":"まだ送っていない"`),
	)
	wantLines(t, ss.mustRun("changes"),
		"n hook (hook を入れるか決める)",
		"answer base = d (B案)",
		"thought 5 on impl (古い題 → now: 新しい題): もっと速く",
		"thought 6: 一行目",
		"  二行目",
		"y gone (消えた行 → now gone)",
		"thought 10: これを見て [Image #3] [Image #3]",
		"  image 3: "+img,
		"hide (2): とても長い題のとても長い行をここで隠し… / 短い",
		"unhide (1): 短い",
	)
	// Only what came before the latest send; nothing twice.
	if out := ss.mustRun("changes"); out != "" {
		t.Errorf("second changes printed %q", out)
	}
	if b, _ := os.ReadFile(filepath.Join(ss.dir, "seen")); strings.TrimSpace(string(b)) != "12" {
		t.Errorf("seen = %q, want 12 (the latest send)", b)
	}
	ss.appendEvents(ev(14, "send", ""))
	wantLines(t, ss.mustRun("changes"), "thought 13: まだ送っていない")
}

// SPEC.md "コメントを直す" "u で取り消す" "コメントを取り消す": how edits,
// retracts and undos reach the AI.
func TestChangesEditsRetractsUndos(t *testing.T) {
	tests := []struct {
		name   string
		before []string // delivered with a first changes
		after  []string // then sent
		want   []string // what the second changes prints
	}{
		{
			name:  "edit before delivery arrives as the thought",
			after: []string{ev(1, "thought", `"text":"old"`), ev(2, "edit", `"thought":1,"text":"new"`)},
			want:  []string{"thought 1: new"},
		},
		{
			name:   "edit after delivery",
			before: []string{ev(1, "thought", `"text":"old"`)},
			after:  []string{ev(3, "edit", `"thought":1,"text":"new"`), ev(4, "edit", `"thought":1,"text":"newer"`)},
			want:   []string{"edit thought 1: newer"},
		},
		{
			name:  "retract before delivery is never told",
			after: []string{ev(1, "thought", `"text":"old"`), ev(2, "retract", `"thought":1`)},
			want:  nil,
		},
		{
			name:   "retract after delivery",
			before: []string{ev(1, "thought", `"text":"old words"`)},
			after:  []string{ev(3, "retract", `"thought":1`)},
			want:   []string{"retract thought 1: old words"},
		},
		{
			name:  "undo of an unsent reaction: as if never",
			after: []string{ev(1, "react", `"id":"hook","value":"n","title":"hook を入れるか決める"`), ev(2, "react", `"id":"hook","value":"y","title":"hook を入れるか決める"`), ev(3, "undo", `"undo":2`)},
			want:  []string{"n hook (hook を入れるか決める)"},
		},
		{
			name:   "undo of a sent reaction",
			before: []string{ev(1, "react", `"id":"hook","value":"y","title":"hook を入れるか決める"`)},
			after:  []string{ev(3, "undo", `"undo":1`)},
			want:   []string{"retract y hook (hook を入れるか決める)"},
		},
		{
			name:   "undo of a sent hide",
			before: []string{ev(1, "hide", `"key":"k","title":"隠した行"`)},
			after:  []string{ev(3, "undo", `"undo":1`)},
			want:   []string{"retract hide: 隠した行"},
		},
		{
			// "送ったが AI がまだ読んでいないものを取り消して送ったら、どちらも届けない"
			name:   "a sent but unread retract taken back: neither told",
			before: []string{ev(1, "thought", `"text":"words"`)},
			after:  []string{ev(3, "retract", `"thought":1`), ev(4, "send", ""), ev(5, "undo", `"undo":3`)},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			ss := s.session("abcdef12345")
			ss.write(changesReport)
			if tt.before != nil {
				ss.appendEvents(tt.before...)
				ss.appendEvents(ev(len(tt.before)+1, "send", ""))
				ss.mustRun("changes")
			}
			ss.appendEvents(tt.after...)
			ss.appendEvents(ev(99, "send", ""))
			out := ss.mustRun("changes")
			wantLines(t, out, tt.want...)
		})
	}

	t.Run("unretract after a delivered retract", func(t *testing.T) {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write(changesReport)
		ss.appendEvents(ev(1, "thought", `"text":"words"`), ev(2, "send", ""))
		ss.mustRun("changes")
		ss.appendEvents(ev(3, "retract", `"thought":1`), ev(4, "send", ""))
		wantLines(t, ss.mustRun("changes"), "retract thought 1: words")
		ss.appendEvents(ev(5, "undo", `"undo":3`), ev(6, "send", ""))
		wantLines(t, ss.mustRun("changes"), "unretract thought 1: words")
	})
}

// SPEC.md "コマンド": the open: line and the report-problem line.
func TestChangesTrailingLines(t *testing.T) {
	s := newSandbox(t)
	ss := s.session("abcdef12345")
	ss.write(changesReport)
	ss.appendEvents(ev(1, "thought", `"text":"a"`), ev(2, "thought", `"text":"b"`), ev(3, "send", ""))
	ss.mustRun("changes")
	// settle prints no open: line.
	if out := ss.mustRun("settle", "1", "--doing", "looking"); strings.Contains(out, "open:") {
		t.Errorf("settle printed an open: line: %q", out)
	}
	ss.mustRun("settle", "2", "--hold")
	wantLines(t, ss.mustRun("changes"), "open: 1 2 (oldest 0s). Details: pitboard -s abcdef12 open")

	// A broken report adds the problem line, after open:.
	ss.write(changesReport + "<Item id=\"z\">\n")
	n := len(strings.Split(strings.TrimSpace(ss.run("check").stdout), "\n"))
	problem := fmt.Sprintf("report.md has %d problem(s). Run: pitboard -s abcdef12 check", n)
	r := ss.run("changes")
	wantLines(t, r.stdout, "open: 1 2 (oldest 0s). Details: pitboard -s abcdef12 open", problem)
	if r.code != 0 {
		t.Errorf("the problem line does not change the exit code: %d", r.code)
	}
	// settle too, even when it fails.
	r = ss.run("settle", "99", "x")
	if r.code == 0 || !strings.Contains(r.stdout, problem) {
		t.Errorf("failed settle: exit %d, stdout %q", r.code, r.stdout)
	}
}

// SPEC.md "コマンド": the shortest unique head of the id, 8 or more.
func TestShortIDInOutput(t *testing.T) {
	s := newSandbox(t)
	ss := s.session("abcdef1234567")
	s.session("abcdef1299999")
	ss.write(changesReport)
	ss.appendEvents(ev(1, "thought", `"text":"a"`), ev(2, "send", ""))
	ss.mustRun("changes")
	ss.mustRun("settle", "1", "--doing")
	if out := ss.mustRun("changes"); !strings.Contains(out, "pitboard -s abcdef123 open") {
		t.Errorf("want the shortest unique head (9 chars): %q", out)
	}
	short := s.session("zz")
	short.write(changesReport)
	short.appendEvents(ev(1, "thought", `"text":"a"`), ev(2, "send", ""))
	short.mustRun("changes")
	short.mustRun("settle", "1", "--doing")
	if out := short.mustRun("changes"); !strings.Contains(out, "pitboard -s zz open") {
		t.Errorf("a short id is printed whole: %q", out)
	}
}

// SPEC.md "コマンド" settle.
func TestSettle(t *testing.T) {
	setup := func(t *testing.T) *session {
		s := newSandbox(t)
		ss := s.session("abcdef12345")
		ss.write(changesReport)
		ss.appendEvents(ev(1, "thought", `"text":"a"`), ev(2, "thought", `"text":"b"`), ev(3, "send", ""))
		ss.mustRun("changes")
		return ss
	}
	last := func(ss *session, n int) []string {
		lines := strings.Split(strings.TrimSpace(ss.events()), "\n")
		return lines[len(lines)-n:]
	}

	t.Run("one, default done, rest is text", func(t *testing.T) {
		ss := setup(t)
		ss.mustRun("settle", "1", "fixed", "5", "bugs")
		l := last(ss, 1)[0]
		if !strings.Contains(l, `"op":"settle"`) || !strings.Contains(l, `"thought":1`) || !strings.Contains(l, `"text":"fixed 5 bugs"`) || strings.Contains(l, `"state"`) {
			t.Errorf("settle record: %s", l)
		}
	})
	t.Run("states", func(t *testing.T) {
		ss := setup(t)
		ss.mustRun("settle", "1", "--doing", "on it")
		ss.mustRun("settle", "2", "--hold")
		l := last(ss, 2)
		if !strings.Contains(l[0], `"state":"doing"`) || !strings.Contains(l[1], `"state":"hold"`) {
			t.Errorf("records: %v", l)
		}
		wantLines(t, ss.mustRun("open"), "doing 1 (0s): a", "hold 2 (0s): b")
		ss.mustRun("settle", "1", "--done", "ok")
		wantLines(t, ss.mustRun("open"), "hold 2 (0s): b")
	})
	t.Run("several", func(t *testing.T) {
		ss := setup(t)
		ss.mustRun("settle", "--done", "1", "ok", "--hold", "2", "later", "on")
		l := last(ss, 2)
		if !strings.Contains(l[0], `"text":"ok"`) || !strings.Contains(l[1], `"text":"later on"`) || !strings.Contains(l[1], `"state":"hold"`) {
			t.Errorf("records: %v", l)
		}
	})
	errs := []struct {
		name string
		args []string
		msg  string
	}{
		{"no such thought", []string{"99", "x"}, "no thought 99"},
		{"done without text", []string{"1", "--done"}, "needs a text"},
		{"two states", []string{"1", "--done", "--hold", "x"}, "two states"},
		{"one missing writes none", []string{"--done", "1", "ok", "--done", "99", "x"}, "99"},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			ss := setup(t)
			before := ss.events()
			r := ss.run(append([]string{"settle"}, tt.args...)...)
			if r.code == 0 || !strings.Contains(r.stderr, tt.msg) {
				t.Errorf("exit %d, stderr %q, want %q", r.code, r.stderr, tt.msg)
			}
			if ss.events() != before {
				t.Errorf("a failed settle wrote something:\n%s", ss.events())
			}
		})
	}
	t.Run("a retracted thought settles without error", func(t *testing.T) {
		ss := setup(t)
		ss.appendEvents(ev(4, "retract", `"thought":1`))
		if r := ss.run("settle", "1", "ok"); r.code != 0 {
			t.Errorf("exit %d: %s", r.code, r.stderr)
		}
	})
}

// SPEC.md "コマンド" open.
func TestOpen(t *testing.T) {
	s := newSandbox(t)
	ss := s.session("abcdef12345")
	ss.write(changesReport)
	r := ss.run("open")
	if r.code != 0 || r.stdout != "" {
		t.Errorf("nothing open: exit %d, out %q", r.code, r.stdout)
	}
	long := strings.Repeat("あ", 40)
	ss.appendEvents(ev(1, "thought", `"text":"`+long+`"`), ev(2, "thought", `"text":"x"`), ev(3, "send", ""))
	ss.mustRun("changes")
	ss.mustRun("settle", "2", "--hold", "later")
	ss.mustRun("settle", "1", "--doing")
	// Oldest first: 2 got its state first. The head is 30 characters,
	// the ellipsis included.
	wantLines(t, ss.mustRun("open"), "hold 2 (0s): x", "doing 1 (0s): "+strings.Repeat("あ", 29)+"…")
	// A retracted one is not open.
	ss.appendEvents(ev(6, "retract", `"thought":2`))
	wantLines(t, ss.mustRun("open"), "doing 1 (0s): "+strings.Repeat("あ", 29)+"…")
}
