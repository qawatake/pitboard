package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC.md "u で取り消す": each kind of action, taken back before it is sent,
// says what it took back and leaves nothing for the AI.
func TestUndoEachAction(t *testing.T) {
	report := `<Item id="a">項目A</Item>
<Item id="look" look="y">読む項目</Item>
<Ask id="q">土台を選ぶ
  <Option key="b">B案</Option>
  <Option key="d">D案</Option>
</Ask>
- ただの行
`
	tests := []struct {
		name string
		do   func(v *viewer, ss *session)
		msg  string
	}{
		{"answer", func(v *viewer, ss *session) {
			v.MoveTo("土台を選ぶ")
			v.Keys("enter")
			v.WaitFor("B案")
			v.Keys("b")
			ss.waitRecords(v.t, "react", 1)
		}, "答え b を取り消しました: 土台を選ぶ"},
		{"place a comment", func(v *viewer, ss *session) {
			v.MoveTo("読む項目")
			v.Keys("c")
			v.WaitFor("enter 置く")
			v.Type("置いた文")
			v.Keys("enter")
			ss.waitRecords(v.t, "thought", 1)
		}, "コメントを取り消しました: 置いた文"},
		{"hide", func(v *viewer, ss *session) {
			v.MoveTo("ただの行")
			v.Keys("x")
			ss.waitRecords(v.t, "hide", 1)
		}, "隠したのを取り消しました: ただの行"},
		{"A", func(v *viewer, ss *session) {
			v.Keys("A")
			v.WaitGone("未読")
		}, "A (すべて読んだことにする) を取り消しました"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, ss := open(t, report, 100, 30, "項目A")
			v.MoveTo("読む項目")
			v.MoveTo("項目A")
			tt.do(v, ss)
			v.Keys("u")
			v.WaitForJoined(tt.msg)
			ss.appendEvents(ev(99, "send", ""))
			if out := ss.mustRun("changes"); out != "" {
				t.Errorf("an action taken back before sending reached the AI: %q", out)
			}
		})
	}

	t.Run("edit and unhide", func(t *testing.T) {
		t.Parallel()
		v, ss := open(t, report, 100, 30, "項目A",
			ev(1, "thought", `"text":"元の文"`), ev(2, "hide", `"key":"-|ただの行","title":"ただの行"`))
		// Into the pane, then e.
		for i := 0; i < 10 && !strings.Contains(v.Bottom(), "e 直す"); i++ {
			v.Keys("j")
			v.Sync()
		}
		v.Keys("e")
		v.WaitFor("コメント 1 を直す")
		v.Keys("ctrl+u")
		v.Type("直した")
		v.Keys("enter")
		ss.waitRecords(t, "edit", 1)
		v.Keys("u")
		v.WaitForJoined("コメントの直しを取り消しました")
		// Unhide, then u.
		v.Keys("k")
		v.Keys("H")
		v.WaitFor("ただの行")
		v.MoveTo("ただの行")
		v.Keys("x")
		ss.waitRecords(t, "unhide", 1)
		v.Keys("u")
		v.WaitForJoined("表示に戻したのを取り消しました: ただの行")
	})
}

// SPEC.md "画像を貼る".
func TestImagePaste(t *testing.T) {
	t.Parallel()
	v, ss := open(t, "<Item id=\"a\">項目A</Item>\n", 100, 30, "項目A")
	src := filepath.Join(ss.s.home, "shot one.PNG")
	writeFile(t, src, "\x89PNG fake")
	v.Keys("m")
	v.WaitFor("宛先なしのコメント")
	// No clipboard tool on PATH.
	v.Keys("ctrl+v")
	v.WaitFor("クリップボードに画像がありません")
	// A dragged file: quoted, with the extension in capitals.
	v.Paste(" '" + src + "' ")
	v.WaitFor("[Image #1]")
	v.Keys("enter")
	rs := ss.waitRecords(t, "thought", 1)
	if rs[0].str("text") != "[Image #1]" {
		t.Errorf("thought %q", rs[0].str("text"))
	}
	img := filepath.Join(ss.dir, "images", "1.png")
	fi, err := os.Stat(img)
	if err != nil {
		matches, _ := filepath.Glob(filepath.Join(ss.dir, "images", "*"))
		t.Fatalf("image not copied: %v (images: %v)", err, matches)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("image mode %v, want 0600", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(img)); di.Mode().Perm() != 0o700 {
		t.Errorf("images/ mode %v, want 0700", di.Mode().Perm())
	}
	wantColor(t, v, "[Image #1]", "dim")
	v.Keys("s")
	v.WaitFor("送りました")
	wantLines(t, ss.mustRun("changes"), "thought 1: [Image #1]", "  image 1: "+img)
}
