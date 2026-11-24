package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// record is one decoded events.jsonl line.
type record map[string]any

func (r record) str(k string) string {
	s, _ := r[k].(string)
	return s
}

func (r record) num(k string) int {
	f, _ := r[k].(float64)
	return int(f)
}

// records decodes events.jsonl, keeping the ops given (all when none).
func (ss *session) records(ops ...string) []record {
	var out []record
	for _, l := range strings.Split(strings.TrimSpace(ss.events()), "\n") {
		if l == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			continue
		}
		if len(ops) == 0 {
			out = append(out, r)
			continue
		}
		for _, op := range ops {
			if r.str("op") == op {
				out = append(out, r)
			}
		}
	}
	return out
}

// waitRecords waits until there are n records with op.
func (ss *session) waitRecords(t *testing.T, op string, n int) []record {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		rs := ss.records(op)
		if len(rs) >= n {
			if len(rs) > n {
				t.Fatalf("%d %s records, want %d:\n%s", len(rs), op, n, ss.events())
			}
			return rs
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d %s records:\n%s", n, op, ss.events())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// noRecords fails when there is any record with op. Call after Sync.
func (ss *session) noRecords(t *testing.T, op string) {
	t.Helper()
	if rs := ss.records(op); len(rs) > 0 {
		t.Fatalf("unexpected %s records:\n%s", op, ss.events())
	}
}

// pressUntil presses key until the n-th op record appears, for keys the
// viewer drops for 0.4 s after it moved the cursor (SPEC.md "報告の画面の
// カーソル"). Polling, not sleeping: each try waits at most 150 ms.
func (v *viewer) pressUntil(ss *session, key, op string, n int) {
	v.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		v.Keys(key)
		try := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(try) {
			if len(ss.records(op)) >= n {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	v.t.Fatalf("%q never made %d %s records:\n%s\n%s", key, n, op, ss.events(), v.dump())
}

const reactReport = `<Title>題</Title>
<Section title="あなたへ">
  <Item id="first">最初の項目</Item>
  <Ask id="hook">hook を入れるか決める
    <Option key="y">入れる</Option>
    <Option key="n">見てから</Option>
  </Ask>
  <Ask id="base">土台を選ぶ
    <Option key="a">A案</Option>
    <Option key="b">B案</Option>
  </Ask>
  <Link id="lnk" path="docs/x.md">リンクの題</Link>
  <Item id="read" look="y">読んでほしい</Item>
  <Item id="last">最後の項目</Item>
</Section>
`

// SPEC.md "報告の画面のカーソル": a cursor the person never moved takes no
// option (an answer to the AI) and says why; x r c m enter work. y and n
// are no keys on an Item (FEAT25).
func TestPlacedCursorTakesNoReaction(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 24, "最初の項目")
	v.WaitCursorOn("最初の項目")
	v.WaitBottom("c 行に書く  x 隠す")
	if strings.Contains(v.Bottom(), "y n") {
		t.Errorf("y n offered: %q", v.Bottom())
	}
	v.Keys("y")
	v.Keys("n")
	v.Sync()
	ss.noRecords(t, "react")
	// c works on the placed cursor.
	v.Keys("c")
	v.WaitFor("enter 置く")
	v.Keys("esc")
	v.WaitGone("enter 置く")
	// So does x (seen at once, not sent until s, undone with u).
	v.Keys("x")
	rs := ss.waitRecords(t, "hide", 1)
	if rs[0].str("title") != "最初の項目" {
		t.Errorf("hide record: %v", rs[0])
	}

	t.Run("narrow: shrinks to j k but is not dropped", func(t *testing.T) {
		v, _ := open(t, "<Ask id=\"q\">問い\n  <Option key=\"a\">A</Option>\n  <Option key=\"b\">B</Option>\n</Ask>\n", 24, 24, "問い")
		v.Keys("enter")
		v.WaitFor("A")
		v.Until("a narrow guide", func() bool { return strings.Contains(v.Bottom(), "j k") })
	})

	t.Run("an open Ask: j k 動かすと答えられる", func(t *testing.T) {
		v, ss := open(t, "<Ask id=\"q\">問い\n  <Option key=\"a\">A</Option>\n  <Option key=\"b\">B</Option>\n</Ask>\n", 100, 20, "問い")
		v.Keys("enter")
		v.WaitFor("A")
		v.WaitBottom("j k 動かすと答えられる")
		v.Keys("a")
		v.WaitFor("j k で動かすと答えられます")
		v.Sync()
		ss.noRecords(t, "react")
	})

	t.Run("r works on a placed cursor", func(t *testing.T) {
		v, _ := open(t, "<Item id=\"g\" look=\"y\">読む</Item>\n<Item id=\"b\">次</Item>\n", 100, 20, "読む")
		v.WaitFor("未読 1")
		v.Keys("r")
		v.WaitGone("未読 1")
	})
}

// SPEC.md "反応": a yes / no is an Ask; its options may be keyed y and n.
func TestAnswerYesNo(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 24, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	row := rowOf(t, v, "hook を入れるか決める")
	if !strings.HasPrefix(v.Line(row), "> ●") && !strings.Contains(v.Line(row), "●") {
		t.Fatalf("an unanswered Ask without a yellow ●:\n%s", v.dump())
	}
	v.Keys("enter")
	v.WaitFor("入れる", "見てから")
	v.WaitBottom("y n 答える")
	v.Keys("y")
	rs := ss.waitRecords(t, "react", 1)
	if rs[0].str("id") != "hook" || rs[0].str("value") != "y" || rs[0].str("option") != "入れる" || rs[0].str("title") != "hook を入れるか決める" {
		t.Errorf("record: %v", rs[0])
	}
	v.Until("y 未送信 at the right end", func() bool { return strings.HasSuffix(v.Line(row), "y 未送信") })
	if c := v.ColorAt(row, colOf(v.Line(row))-1); c != "yellow" {
		t.Errorf("未送信 is %q, want yellow", c)
	}
	if strings.Contains(v.Line(row), "●") {
		t.Errorf("the yellow ● stays after y:\n%s", v.dump())
	}
	// Pressed again: the newer counts, both are kept.
	v.Keys("n")
	ss.waitRecords(t, "react", 2)
	v.Until("n 未送信", func() bool { return strings.HasSuffix(v.Line(row), "n 未送信") })
	// "報告は書き換えない"
	b, _ := os.ReadFile(ss.report)
	if string(b) != reactReport {
		t.Errorf("the viewer changed report.md")
	}

	t.Run("not on a Link: y is no key there", func(t *testing.T) {
		v.In(t).MoveTo("リンクの題")
		v.Keys("y")
		v.Sync()
		if rs := ss.records("react"); len(rs) != 2 {
			t.Errorf("y on a Link recorded: %v", rs)
		}
	})

	t.Run("the AI rewrites the line: the reaction stops showing", func(t *testing.T) {
		v.In(t)
		ss.write(strings.Replace(reactReport, "hook を入れるか決める", "hook を入れる", 1))
		v.WaitFor("hook を入れる\n")
		v.Until("no reaction on the rewritten line", func() bool {
			return !strings.Contains(v.Line(rowOf(t, v, "hook を入れる")), "未送信")
		})
	})
}

// SPEC-format.md "描き方" Ask: option keys work only when open.
func TestAnswerAsk(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 24, "最初の項目")
	v.MoveTo("土台を選ぶ")
	v.WaitBottom("enter 開く  c 行に書く  x 隠す")
	v.Keys("a")
	v.WaitFor("A案", "B案")
	v.Sync()
	ss.noRecords(t, "react")
	v.WaitBottom("a b 答える  enter 閉じる  c 行に書く")
	v.Keys("b")
	rs := ss.waitRecords(t, "react", 1)
	if rs[0].str("id") != "base" || rs[0].str("value") != "b" || rs[0].str("option") != "B案" || rs[0].str("title") != "土台を選ぶ" {
		t.Errorf("record: %v", rs[0])
	}
	// Stays open after answering; the chosen option is marked.
	v.WaitFor("b 未送信")
	if !strings.Contains(v.Screen(), "A案") {
		t.Errorf("the Ask closed after answering")
	}
	if a, b := v.Line(rowOf(t, v, "A案")), v.Line(rowOf(t, v, "B案")); strings.Count(a, "●")+strings.Count(a, "✓") >= strings.Count(b, "●")+strings.Count(b, "✓") && a[:6] == b[:6] {
		t.Errorf("the chosen option has no mark:\n%s", v.dump())
	}
	// Stays open across a reload (kept by id).
	ss.write(reactReport + "\n")
	v.Sync()
	if !strings.Contains(v.Screen(), "A案") {
		t.Errorf("the Ask closed on reload")
	}
}

// SPEC.md "コメント": c on a line, the editor under it, enter places it.
func TestCommentOnLine(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 30, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	v.Keys("c")
	v.WaitFor("enter 置く  esc やめる")
	row := rowOf(t, v, "hook を入れるか決める")
	// The block sits right under the line: a rule, the input with ❯, a
	// rule, the guide; no heading for c.
	// "上の線の上と、案内の行の下に空行を 1 行ずつ置く"
	if strings.TrimSpace(v.Line(row+1)) != "" || !strings.HasPrefix(strings.TrimSpace(v.Line(row+2)), "─") {
		t.Errorf("no blank line and rule under the line:\n%s", v.dump())
	}
	if !strings.Contains(v.Line(row+3), "❯") {
		t.Errorf("no ❯ on the input row:\n%s", v.dump())
	}
	if strings.Contains(v.Screen(), "にコメント") || strings.Contains(v.Screen(), "宛先なしのコメント") {
		t.Errorf("c shows a heading:\n%s", v.dump())
	}
	// The pty cannot tell shift+enter from enter: the guide says ctrl+j.
	if !strings.Contains(v.Screen(), "ctrl+j 改行") {
		t.Errorf("guide without ctrl+j 改行:\n%s", v.dump())
	}
	// The bottom row is empty while writing; the comments pane is not shown.
	if strings.TrimSpace(v.Bottom()) != "" {
		t.Errorf("bottom row while writing: %q", v.Bottom())
	}
	v.Type("一行目")
	v.Keys("ctrl+j")
	v.Type("二行目")
	v.Keys("shift+enter")
	v.Type("三行目")
	v.Keys("alt+enter")
	v.Type("四行目")
	// ctrl+s inside the editor does nothing.
	v.Keys("ctrl+s")
	v.WaitFor("四行目")
	if len(ss.records("send")) > 0 {
		t.Fatalf("ctrl+s sent")
	}
	v.Keys("enter")
	rs := ss.waitRecords(t, "thought", 1)
	if rs[0].str("id") != "hook" || rs[0].str("text") != "一行目\n二行目\n三行目\n四行目" || rs[0].str("title") != "hook を入れるか決める" {
		t.Errorf("record: %v", rs[0])
	}
	// On the row's right end; in the comments pane (newlines as spaces);
	// counted at the top right; under the line when it is opened, one row
	// per line.
	v.WaitFor("未送信 1", "コメント", "一行目 二行目 三行目 四行目")
	v.Keys("enter")
	v.WaitFor("✎ 一行目")
	if r := rowOf(t, v, "二行目"); r != rowOf(t, v, "✎ 一行目")+1 {
		t.Errorf("comment rows:\n%s", v.dump())
	}
	if !strings.HasSuffix(v.Line(row), "未送信") {
		t.Errorf("row's right end: %q", v.Line(row))
	}
	wantColor(t, v, "未送信 1", "yellow")
	v.WaitBottom("s 送る  m 書く")
}

// SPEC.md "コメント": m writes a thought without a line.
func TestCommentWithoutLine(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 30, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	row := rowOf(t, v, "hook を入れるか決める")
	v.Keys("m")
	v.WaitFor("宛先なしのコメント")
	if r := rowOf(t, v, "宛先なしのコメント"); r != row+1 && r != row+2 {
		t.Errorf("m's block not under the cursor's line:\n%s", v.dump())
	}
	v.Type("宛先なし")
	v.Keys("enter")
	rs := ss.waitRecords(t, "thought", 1)
	if _, ok := rs[0]["id"]; ok || rs[0].str("text") != "宛先なし" {
		t.Errorf("record: %v", rs[0])
	}

	t.Run("esc cancels: nothing recorded", func(t *testing.T) {
		v.In(t).Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Type("やめる")
		v.Keys("esc")
		v.WaitGone("宛先なしのコメント")
		v.Sync()
		if n := len(ss.records("thought")); n != 1 {
			t.Errorf("%d thoughts", n)
		}
		// The draft stays: m again brings it back; emptied and closed, it
		// is gone.
		v.Keys("m")
		v.WaitFor("❯ やめる")
		v.Keys("ctrl+u", "esc")
		v.WaitGone("宛先なしのコメント")
	})

	t.Run("a paste: CR, LF and CRLF are one newline each; it does not submit", func(t *testing.T) {
		v.In(t).Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Paste("a\rb\nc\r\nd")
		v.WaitFor("❯ a")
		v.Until("the whole paste", func() bool { return strings.TrimSpace(v.Line(rowOf(t, v, "❯ a")+3)) == "d" })
		if n := len(ss.records("thought")); n != 1 {
			t.Fatalf("the paste submitted")
		}
		v.Keys("enter")
		rs := ss.waitRecords(t, "thought", 2)
		if got := rs[1].str("text"); got != "a\nb\nc\nd" {
			t.Errorf("pasted text %q", got)
		}
	})
}

// SPEC.md "コメントを直す" and "コメントを取り消す": e and x in the pane.
func TestEditAndRetractInPane(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 30, "最初の項目")
	v.Keys("m")
	v.WaitFor("宛先なしのコメント")
	v.Type("元の文")
	v.Keys("enter")
	ss.waitRecords(t, "thought", 1)
	v.WaitFor("コメント")
	// j past the last stop enters the pane.
	for i := 0; i < 10 && !strings.Contains(v.Bottom(), "e 直す"); i++ {
		v.Keys("j")
		v.Sync()
	}
	for _, part := range []string{"enter 開く", "e 直す", "x 取り消す", "m 書く", "? キー"} {
		v.WaitBottom(part)
	}
	v.Keys("e")
	v.WaitFor("コメント 1 を直す", "❯ 元の文")
	// Unchanged: nothing.
	v.Keys("enter")
	v.WaitGone("コメント 1 を直す")
	v.Sync()
	ss.noRecords(t, "edit")
	v.Keys("e")
	v.WaitFor("コメント 1 を直す")
	v.Keys("ctrl+u")
	v.Type("直した文")
	v.Keys("enter")
	rs := ss.waitRecords(t, "edit", 1)
	if rs[0].num("thought") != 1 || rs[0].str("text") != "直した文" {
		t.Errorf("edit record: %v", rs[0])
	}
	v.WaitFor("直した文")

	v.Keys("x")
	rs = ss.waitRecords(t, "retract", 1)
	if rs[0].num("thought") != 1 {
		t.Errorf("retract record: %v", rs[0])
	}
	v.WaitFor("取り消しました (u で戻す)")
	v.Until("the pane empties", func() bool { return !strings.Contains(v.Screen(), "直した文") })

	v.Keys("u")
	ss.waitRecords(t, "undo", 1)
	v.WaitFor("取り消したコメントを戻しました")
}

// hideReport has no Section (no g) and no marks (no space), so the guide
// has room for 隠した行 N.
const hideReport = `<Item id="first">最初の項目</Item>
<Item id="hook">hook を入れるか決める</Item>
<Item id="base">土台を選ぶ</Item>
<Item id="last">最後の項目</Item>
`

// listReport is plain "- " lines: x is all they take, so the guide is short.
const listReport = `- 最初の項目
- hook を入れるか決める
- 土台を選ぶ
- 最後の項目
`

// SPEC.md "行を隠す".
func TestHide(t *testing.T) {
	t.Parallel()
	v, ss := open(t, listReport, 120, 30, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	v.WaitBottom("x 隠す")
	v.Keys("x")
	rs := ss.waitRecords(t, "hide", 1)
	if rs[0].str("title") != "hook を入れるか決める" || rs[0].str("key") == "" {
		t.Errorf("hide record: %v", rs[0])
	}
	v.WaitFor("隠しました (H で表示、u で戻す)")
	v.WaitGone("hook を入れるか決める")
	// The cursor moves to the next line, which takes no x for 0.4 s: a
	// dropped x says why, and pressing it again takes it.
	v.WaitCursorOn("土台を選ぶ")
	v.Keys("x")
	v.WaitForJoined("いま移った行です。もう一度押すと隠れます")
	if n := len(ss.records("hide")); n != 1 {
		t.Fatalf("x right after hiding was taken (%d hides)", n)
	}
	v.WaitBottom("隠した行 1")
	wantColor(t, v, "隠した行 1", "dim")
	// 0.4 s later x works without moving.
	v.pressUntil(ss, "x", "hide", 2)
	v.WaitBottom("隠した行 2")

	t.Run("H shows hidden lines faint with ⌀; x there unhides", func(t *testing.T) {
		v.In(t).Keys("H")
		v.WaitFor("hook を入れるか決める")
		v.WaitBottom("隠した行 2 (表示中)")
		r := rowOf(t, v, "hook を入れるか決める")
		if !strings.Contains(v.Line(r), "⌀") {
			t.Errorf("no ⌀:\n%s", v.dump())
		}
		wantColor(t, v, "hook を入れるか決める", "dim")
		v.MoveTo("hook を入れるか決める")
		v.WaitBottom("x 表示に戻す")
		v.Keys("x")
		rs := ss.waitRecords(t, "unhide", 1)
		if rs[0].str("key") != ss.records("hide")[0].str("key") {
			t.Errorf("unhide key differs from hide key")
		}
		v.WaitForJoined("表示に戻しました")
		v.Keys("H")
		v.WaitBottom("隠した行 1")
		if !strings.Contains(v.Screen(), "hook を入れるか決める") {
			t.Errorf("unhidden line not shown")
		}
	})

	t.Run("a changed title shows again", func(t *testing.T) {
		v.In(t)
		hidden := ss.records("hide")[1].str("title")
		ss.write(strings.Replace(listReport, hidden, hidden+" (改)", 1))
		v.WaitFor(hidden + " (改)")
	})
}

// FIX22, FIX23: hiding three lines in a row is x, then x x on each line
// the viewer moved the cursor to (the first x there is dropped); u brings
// back the last only.
func TestHideRowsInARow(t *testing.T) {
	t.Parallel()
	v, ss := open(t, listReport, 120, 30, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	v.MoveTo("最初の項目")
	v.WaitBottom("x 隠す")
	v.Keys("x")
	ss.waitRecords(t, "hide", 1)
	for n, line := range []string{"hook を入れるか決める", "土台を選ぶ"} {
		v.WaitCursorOn(line)
		v.Keys("x", "x")
		ss.waitRecords(t, "hide", n+2)
	}
	rs := ss.records("hide")
	for i, want := range []string{"最初の項目", "hook を入れるか決める", "土台を選ぶ"} {
		if i >= len(rs) || rs[i].str("title") != want {
			t.Errorf("hide %d: %v, want %q", i+1, rs, want)
		}
	}
	v.WaitBottom("隠した行 3")
	v.WaitCursorOn("最後の項目")
	v.Keys("u")
	v.WaitFor("土台を選ぶ")
	v.WaitBottom("隠した行 2")
	if s := v.Screen(); strings.Contains(s, "最初の項目") || strings.Contains(s, "hook を入れるか決める") {
		t.Errorf("u brought back more than the last:\n%s", v.dump())
	}
}

// SPEC.md "行を隠す": every section's lines hidden: no heading.
func TestHideWholeSection(t *testing.T) {
	t.Parallel()
	report := `<Section title="一つ目">
  <Item id="a">項目A</Item>
</Section>
<Section title="二つ目">
  <Item id="b">項目B</Item>
</Section>
`
	v, ss := open(t, report, 60, 20, "項目A")
	v.MoveTo("項目B")
	v.MoveTo("項目A")
	v.Keys("x")
	ss.waitRecords(t, "hide", 1)
	v.WaitGone("項目A", "一つ目")
}

// SPEC.md "反応待ちと未読": r, A and the viewed file.
func TestReadMarks(t *testing.T) {
	t.Parallel()
	report := `<Section title="s">
  <Item id="a">普通</Item>
  <Item id="g1" look="y">読んでほしい一</Item>
  <Item id="g2" look="y">読んでほしい二</Item>
</Section>
`
	v, ss := open(t, report, 60, 20, "読んでほしい一")
	viewed := filepath.Join(ss.dir, "viewed")
	for _, title := range []string{"読んでほしい一", "読んでほしい二"} {
		r := rowOf(t, v, title)
		if !strings.Contains(v.Line(r), "●") || v.ColorAt(r, 2) != "green" {
			t.Errorf("%s: no green ●:\n%s", title, v.dump())
		}
	}
	v.WaitFor("未読 2")
	wantColor(t, v, "未読 2", "green")
	// The cursor alone is not reading.
	v.MoveTo("読んでほしい一")
	v.MoveTo("普通")
	v.MoveTo("読んでほしい一")
	if !strings.Contains(v.Line(rowOf(t, v, "読んでほしい一")), "●") {
		t.Fatalf("the cursor cleared the mark")
	}
	v.WaitBottom("r 読んだ")
	v.Keys("r")
	v.Until("the mark cleared", func() bool { return !strings.Contains(v.Line(rowOf(t, v, "読んでほしい一")), "●") })
	v.WaitFor("未読 1")
	got := pollFile(t, viewed, "a hash", func(s string) bool { return len(strings.TrimSpace(s)) == 32 })
	for _, c := range strings.TrimSpace(got) {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("viewed is not 32 hex: %q", got)
		}
	}
	// u takes r back with a "-" line.
	v.Keys("u")
	v.WaitFor("r (読んだ) を取り消しました")
	pollFile(t, viewed, "a - line", func(s string) bool { return strings.Contains(s, "-"+strings.TrimSpace(got)) })
	v.WaitFor("未読 2")
	// A reads everything.
	v.Keys("A")
	v.WaitGone("未読")
	ss.noRecords(t, "react") // never reaches the AI
	// Rewriting a look line marks it unread again.
	ss.write(strings.Replace(report, "読んでほしい二", "読んでほしい二 (改)", 1))
	v.WaitFor("未読 1")
}

// SPEC.md "u で取り消す".
func TestUndo(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 30, "最初の項目")
	v.Keys("u")
	v.WaitFor("取り消せる操作がありません")
	v.MoveTo("hook を入れるか決める")
	v.Keys("enter")
	v.WaitFor("見てから")
	v.Keys("n")
	ss.waitRecords(t, "react", 1)
	v.Keys("y")
	ss.waitRecords(t, "react", 2)
	row := rowOf(t, v, "hook を入れるか決める")

	// Not sent: as if never; y goes back to n.
	v.Keys("u")
	v.WaitFor("答え y を取り消しました: hook を入れるか決める")
	v.Until("back to n", func() bool { return strings.HasSuffix(v.Line(row), "n 未送信") })
	// Sent: the undo is new input, shown as 取り消し 未送信 until sent.
	v.Keys("s")
	v.WaitFor("送りました")
	v.Until("n 送信済", func() bool { return strings.HasSuffix(v.Line(row), "n 送信済") })
	v.Keys("u")
	v.WaitFor("答え n を取り消しました")
	v.Until("取り消し 未送信", func() bool { return strings.HasSuffix(v.Line(row), "取り消し 未送信") })
	v.Keys("s")
	v.WaitFor("送りました")
	ss.waitRecords(t, "send", 2)
	// Answered and taken back before the AI read either: neither is told.
	if out := ss.mustRun("changes"); strings.Contains(out, "answer hook = n") && !strings.Contains(out, "undo") {
		t.Errorf("changes: %q", out)
	}
	// s cannot be undone.
	v.Keys("u")
	v.WaitFor("取り消せる操作がありません")
}

// SPEC.md "u で取り消す": the guide shows u when there is something to
// take back (here with room for it).
func TestUndoGuide(t *testing.T) {
	t.Parallel()
	v, ss := open(t, hideReport, 120, 20, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	if strings.Contains(v.Bottom(), "u 元に戻す") {
		t.Errorf("u in the guide with nothing to take back")
	}
	v.Keys("x")
	ss.waitRecords(t, "hide", 1)
	v.WaitFor("隠しました")
	v.Keys("k") // the message gives the guide back on the next move
	v.WaitBottom("u 元に戻す")
}

// SPEC.md "送る": every way of waking the AI.
func TestSend(t *testing.T) {
	t.Run("nothing unsent", func(t *testing.T) {
		t.Parallel()
		v, ss := open(t, reactReport, 120, 24, "最初の項目")
		v.WaitFor("あなたの番")           // never sent
		wantColor(t, v, "あなたの番", "") // the normal color: yellow is for the counts
		v.Keys("s")
		v.WaitFor("未送信のものはありません")
		ss.noRecords(t, "send")
	})

	t.Run("nobody to wake", func(t *testing.T) {
		t.Parallel()
		v, ss := open(t, reactReport, 100, 24, "最初の項目")
		v.Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Type("hi")
		v.Keys("enter")
		ss.waitRecords(t, "thought", 1)
		v.Keys("s")
		v.WaitFor("送りました。ただ AI が待っていないので、チャットで一言伝えてください。")
		v.WaitFor("PITBOARD_SEND")
		ss.waitRecords(t, "send", 1)
		v.WaitGone("未送信")
		v.WaitFor("AI の番")
		wantColor(t, v, "AI の番", "dim")
	})

	t.Run("to a waiting wait", func(t *testing.T) {
		t.Parallel()
		v, ss := open(t, reactReport, 120, 24, "最初の項目")
		w := ss.start("wait")
		ss.waitWaiting(t, w.cmd.Process.Pid)
		v.WaitFor("AI 待機中")
		wantColor(t, v, "AI 待機中", "dim")
		v.Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Type("hi")
		v.Keys("enter")
		ss.waitRecords(t, "thought", 1)
		v.Keys("s")
		v.WaitFor("送りました")
		w.waitExit(t)
		wantLines(t, w.stdout(), "thought 1: hi")
		v.WaitGone("AI 待機中")
		// The AI read it: 既読.
		v.WaitFor("既読")
	})

	t.Run("through PITBOARD_SEND", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		out := filepath.Join(s.home, "sent.txt")
		s.env["PITBOARD_SEND"] = `printf '%s|%s' "$PITBOARD_SESSION" "$PITBOARD_MESSAGE" > ` + out
		ss := s.session("abcdef12345")
		ss.write(reactReport)
		v := ss.view(80, 24)
		v.WaitFor("最初の項目")
		v.Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Type("hi")
		v.Keys("enter")
		ss.waitRecords(t, "thought", 1)
		v.Keys("s")
		v.WaitFor("送りました")
		got := pollFile(t, out, "the typed line", func(s string) bool { return s != "" })
		if got != "abcdef12345|pitboard: 1 sent. Run: pitboard -s abcdef12 changes" {
			t.Errorf("PITBOARD_SEND got %q", got)
		}
	})

	t.Run("PITBOARD_SEND fails", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		s.env["PITBOARD_SEND"] = `echo "boom happened" >&2; exit 3`
		ss := s.session("abcdef12345")
		ss.write(reactReport)
		v := ss.view(120, 24)
		v.WaitFor("最初の項目")
		v.Keys("m")
		v.WaitFor("宛先なしのコメント")
		v.Type("hi")
		v.Keys("enter")
		ss.waitRecords(t, "thought", 1)
		v.Keys("s")
		v.WaitFor("送りました。ただ AI を起こせなかったので、チャットで一言伝えてください。", "送れませんでした: ", "boom happened")
		// The send is recorded anyway.
		ss.waitRecords(t, "send", 1)
	})
}

// SPEC.md "コメント": the states as seen on screen.
func TestCommentStates(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 30, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	v.Keys("c")
	v.WaitFor("enter 置く")
	v.Type("状態を見る")
	v.Keys("enter")
	ss.waitRecords(t, "thought", 1)
	v.WaitGone("enter 置く")
	v.Keys("enter") // open the line: its comments show under it
	v.WaitFor("✎ 状態を見る")
	row := rowOf(t, v, "hook を入れるか決める")
	state := func(word, color string) {
		t.Helper()
		v.Until("row ends with "+word, func() bool { return strings.HasSuffix(v.Line(row), word) })
		if got := v.ColorAt(row, colOf(v.Line(row))-1); got != color {
			t.Errorf("%s is %q, want %q", word, got, color)
		}
		v.Until("comment line ends with "+word, func() bool { return strings.HasSuffix(v.Line(rowOf(t, v, "✎ 状態を見る")), word) })
	}
	state("未送信", "yellow")
	v.Keys("s")
	state("送信済", "dim")
	ss.mustRun("changes")
	state("既読", "dim")
	ss.mustRun("settle", "1", "--doing", "見ています")
	state("対応中", "green") // the AI works on it: move
	ss.mustRun("settle", "1", "--hold", "あとで")
	state("保留", "dim")
	ss.mustRun("settle", "1", "--done", "やりました")
	v.Until("row ends with 対応済", func() bool { return strings.HasSuffix(v.Line(row), "対応済") })
	// Done, it folds into one faint line under the row; enter shows it.
	v.WaitFor("✎ 対応済 1 件")
	v.WaitGone("✎ 状態を見る")
	wantColor(t, v, "対応済 1 件", "dim")
	v.Keys("enter")
	v.WaitGone("対応済 1 件")
	state("対応済", "dim")
	// SPEC.md 「画面」: "行の下のコメントに AI の文があれば `対応: …` とし、
	// 頭の語だけを薄く". (「コメント」 says only the state word shows under
	// the line; the two statements conflict: see docs/dev/E2E1-result.md.)
	v.WaitFor("対応: やりました")
	wantColor(t, v, "対応:", "dim")
	wantColor(t, v, "やりました", "")
	// Not in the pane: only the state word there.
	if strings.Count(v.Screen(), "やりました") != 1 {
		t.Errorf("the AI's text shows in the pane:\n%s", v.dump())
	}
	v.Keys("tab")
	v.WaitFor("状態を見る")
	v.MoveTo("状態を見る")
	v.Keys("enter")
	v.WaitFor("やりました")
}

// SPEC.md "報告の画面のカーソル": a changed cursor line takes no reaction
// until j / k.
func TestChangedLineGuard(t *testing.T) {
	t.Parallel()
	v, ss := open(t, reactReport, 120, 24, "最初の項目")
	v.MoveTo("hook を入れるか決める")
	v.Keys("enter")
	v.WaitFor("見てから")
	ss.write(strings.Replace(reactReport, "hook を入れるか決める", "hook の題が変わった", 1))
	v.WaitFor("報告が書き換わりました")
	v.Keys("y")
	v.Sync()
	ss.noRecords(t, "react")
	v.Keys("j", "k")
	v.WaitCursorOn("hook の題が変わった")
	v.Keys("y")
	ss.waitRecords(t, "react", 1)

	t.Run("an option-only change does not guard", func(t *testing.T) {
		v.In(t)
		ss.write(strings.Replace(strings.Replace(reactReport, "hook を入れるか決める", "hook の題が変わった", 1), "見てから", "見てから決める", 1))
		v.WaitFor("見てから決める")
		v.Keys("n")
		ss.waitRecords(t, "react", 2)
	})

	t.Run("pressed on a report changed on disk: nothing recorded", func(t *testing.T) {
		v.In(t)
		ss.write(strings.Replace(reactReport, "hook を入れるか決める", "またも変わった", 1))
		v.Keys("y")
		v.Until("either guard message", func() bool {
			s := v.Screen()
			return strings.Contains(s, "報告が書き換わりました")
		})
		v.Sync()
		ss.waitRecords(t, "react", 2)
	})
}
