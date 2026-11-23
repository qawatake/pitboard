package e2e_test

import (
	"strings"
	"testing"
)

// open starts a viewer on report (w x h) and waits for text to show.
func open(t *testing.T, report string, w, h int, text string, events ...string) (*viewer, *session) {
	t.Helper()
	s := newSandbox(t)
	ss := s.session("abcdef12345")
	if len(events) > 0 {
		ss.appendEvents(events...)
	}
	ss.write(report)
	v := ss.view(w, h)
	v.WaitFor(text)
	return v, ss
}

// rowOf returns the row containing sub, failing when absent.
func rowOf(t *testing.T, v *viewer, sub string) int {
	t.Helper()
	r, _ := v.Find(sub)
	if r < 0 {
		t.Fatalf("%q not on screen:\n%s", sub, v.dump())
	}
	return r
}

// colOfText returns the column where sub starts.
func colOfText(t *testing.T, v *viewer, sub string) int {
	t.Helper()
	_, c := v.Find(sub)
	if c < 0 {
		t.Fatalf("%q not on screen:\n%s", sub, v.dump())
	}
	return c
}

func wantColor(t *testing.T, v *viewer, sub, want string) {
	t.Helper()
	if got := v.ColorOf(sub); got != want {
		t.Errorf("%q is %q, want %q; screen:\n%s", sub, got, want, v.dump())
	}
}

// joinedWithout is joined with the words removed from the ends of lines
// (right-end words), so a wrapped title reads whole.
func joinedWithout(v *viewer, words ...string) string {
	var b strings.Builder
	for _, l := range v.Lines() {
		l = strings.TrimSpace(l)
		for _, w := range words {
			l = strings.TrimSpace(strings.TrimSuffix(l, w))
		}
		b.WriteString(l)
	}
	return b.String()
}

// joined is the screen with every line trimmed and joined, to find text
// that wraps.
func joined(v *viewer) string {
	var b strings.Builder
	for _, l := range v.Lines() {
		b.WriteString(strings.TrimSpace(l))
	}
	return b.String()
}

const textReport = `<Title>とても長い題名がここにあって折り返されるかどうかを見るためのものです</Title>

段落の一行目
続く行は同じ段落。

二つ目の段落

**語**: 太字の語

**小見出し**

- 箇条一
  - 入れ子二
    - 入れ子三
1. 番号一
2. 番号二
行末で改行{{2SP}}
次の行
バックスラッシュ\
の次
`

// SPEC-format.md "文の書き方".
func TestRenderText(t *testing.T) {
	t.Parallel()
	// Two trailing spaces are written explicitly: editors strip them.
	v, _ := open(t, strings.ReplaceAll(textReport, "{{2SP}}", "  "), 40, 40, "次の行")

	t.Run("Title wraps and is never cut", func(t *testing.T) {
		v.In(t)
		if !strings.Contains(joined(v), "とても長い題名がここにあって折り返されるかどうかを見るためのものです") {
			t.Errorf("title cut:\n%s", v.dump())
		}
		wantColor(t, v, "とても長い", "bold")
	})
	t.Run("paragraph lines join; a blank line stays between paragraphs", func(t *testing.T) {
		v.In(t)
		r := rowOf(t, v, "段落の一行目続く行は同じ段落。")
		if strings.TrimSpace(v.Line(r+1)) != "" || !strings.Contains(v.Line(r+2), "二つ目の段落") {
			t.Errorf("paragraphs:\n%s", v.dump())
		}
	})
	t.Run("**word**: head word in bold, colon kept", func(t *testing.T) {
		v.In(t)
		rowOf(t, v, "語: 太字の語")
		wantColor(t, v, "語: 太字の語", "bold")
		if c := v.ColorOf(": 太字の語"); c == "bold" {
			t.Errorf("only the head word is bold")
		}
	})
	t.Run("small heading in the section color with a blank line above", func(t *testing.T) {
		v.In(t)
		r := rowOf(t, v, "小見出し")
		wantColor(t, v, "小見出し", "blue")
		if !v.AttrAt(r, colOfText(t, v, "小見出し")).bold {
			t.Errorf("small heading is not bold")
		}
		if strings.TrimSpace(v.Line(r-1)) != "" {
			t.Errorf("no blank line above the small heading:\n%s", v.dump())
		}
	})
	t.Run("nested lists: · - · two columns deeper each", func(t *testing.T) {
		v.In(t)
		r1, r2, r3 := rowOf(t, v, "箇条一"), rowOf(t, v, "入れ子二"), rowOf(t, v, "入れ子三")
		c1, c2, c3 := colOfText(t, v, "箇条一"), colOfText(t, v, "入れ子二"), colOfText(t, v, "入れ子三")
		if c2-c1 != 2 || c3-c2 != 2 {
			t.Errorf("indents %d %d %d:\n%s", c1, c2, c3, v.dump())
		}
		for _, x := range []struct {
			row, col int
			mark     string
		}{{r1, c1, "·"}, {r2, c2, "-"}, {r3, c3, "·"}} {
			if got := string([]rune(v.Line(x.row))[x.col-2]); got != x.mark {
				t.Errorf("mark %q, want %q:\n%s", got, x.mark, v.dump())
			}
		}
	})
	t.Run("numbers shown as written", func(t *testing.T) {
		v.In(t)
		rowOf(t, v, "1. 番号一")
		rowOf(t, v, "2. 番号二")
	})
	t.Run("two spaces or a backslash break the line", func(t *testing.T) {
		v.In(t)
		if rowOf(t, v, "次の行") != rowOf(t, v, "行末で改行")+1 {
			t.Errorf("no break:\n%s", v.dump())
		}
		if rowOf(t, v, "の次") != rowOf(t, v, "バックスラッシュ")+1 || strings.Contains(v.Screen(), `\`) {
			t.Errorf("backslash break:\n%s", v.dump())
		}
	})
}

const partsReport = `<Title>題</Title>
<Section title="あなたへ">
  <Item id="cont">中身あり
    中身の一行目
    - 中身の箇条
    <Code lang="diff">
    -old
    +new
    </Code>
  </Item>
  <Item id="plain">中身なし</Item>
  <Item id="dn" state="done">済んだ</Item>
  <Item id="nt" note="理由の文">note あり</Item>
  <Item id="w" wait="you" status="あなた待ち">待ち</Item>
  <Item id="d" state="doing">動いている</Item>
  <Item id="d2" state="doing" status="半分">動いている2</Item>
  <Item id="q" status="queued">並んでいる</Item>
  <Link id="p" path="docs/x.md">パスのリンク</Link>
  <Link id="u" url="https://example.com/a/very/long/url/that/should/wrap/and/never/be/cut/at/all">URLのリンク</Link>
</Section>
<Fold title="外">
  <Item id="in1">中の項目</Item>
  <Fold title="内">
    <Item id="in2">さらに中</Item>
  </Fold>
  - ただの行
</Fold>
`

// SPEC-format.md "部品" and "描き方".
func TestRenderParts(t *testing.T) {
	t.Parallel()
	v, _ := open(t, partsReport, 40, 50, "パスのリンク")

	t.Run("Section heading in bold blue", func(t *testing.T) {
		v.In(t)
		wantColor(t, v, "あなたへ", "blue")
	})
	t.Run("Item heads: ▸ opens, faint · does not, ✓ done", func(t *testing.T) {
		v.In(t)
		for _, x := range []struct{ title, head, color string }{
			{"中身あり", "▸", ""}, {"note あり", "▸", ""}, {"中身なし", "·", "dim"}, {"済んだ", "✓", "dim"},
		} {
			r, c := v.Find(x.title)
			if r < 0 {
				t.Fatalf("%q missing", x.title)
			}
			line := []rune(v.Line(r))
			if got := string(line[c-2]); got != x.head {
				t.Errorf("%s: head %q, want %q", x.title, got, x.head)
			}
			if got := v.ColorAt(r, c-2); got != x.color {
				t.Errorf("%s: head color %q, want %q", x.title, got, x.color)
			}
		}
		wantColor(t, v, "済んだ", "dim")
	})
	t.Run("right-end words and their colors", func(t *testing.T) {
		v.In(t)
		for _, x := range []struct{ title, word, color string }{
			{"待ち", "あなた待ち", "yellow"},    // wait="you"
			{"動いている2", "半分", "green"},     // doing with a status
			{"動いている", "作業中", "green"},     // doing without one: the tool says 作業中
			{"並んでいる", "queued", "dim"},     // any other status
		} {
			r := rowOf(t, v, x.title)
			line := v.Line(r)
			if !strings.HasSuffix(line, x.word) {
				t.Errorf("%s: %q does not end with %q", x.title, line, x.word)
			}
			if end := colOf(line); end != 40-2 {
				t.Errorf("%s: the word ends at column %d, want %d", x.title, end, 38)
			}
			if got := v.ColorAt(r, colOf(line)-1); got != x.color {
				t.Errorf("%s: %q is %q, want %q", x.title, x.word, got, x.color)
			}
		}
	})
	t.Run("Link shows its path faint and its URL whole", func(t *testing.T) {
		v.In(t)
		r := rowOf(t, v, "パスのリンク")
		if !strings.Contains(v.Line(r+1), "docs/x.md") {
			t.Errorf("no path under the Link:\n%s", v.dump())
		}
		wantColor(t, v, "docs/x.md", "dim")
		if !strings.Contains(joined(v), "https://example.com/a/very/long/url/that/should/wrap/and/never/be/cut/at/all") {
			t.Errorf("URL cut:\n%s", v.dump())
		}
		r, c := v.Find("https://")
		if !v.AttrAt(r, c).ul {
			t.Errorf("URL not underlined")
		}
		if !strings.Contains(v.Raw(), "\x1b]8;;https://example.com/a/very/long/url") {
			t.Errorf("URL is not an OSC 8 hyperlink")
		}
	})
	t.Run("top-level Fold in bold blue, starts closed", func(t *testing.T) {
		v.In(t)
		wantColor(t, v, "▸ 外", "blue")
		if strings.Contains(v.Screen(), "中の項目") {
			t.Errorf("Fold starts open")
		}
	})
	t.Run("opened Item: contents 2 deeper than the title, a blank after, diff colored", func(t *testing.T) {
		v.In(t)
		v.MoveTo("中身あり")
		v.Keys("enter")
		v.WaitFor("中身の一行目")
		tc, cc := colOfText(t, v, "中身あり"), colOfText(t, v, "中身の一行目")
		if cc-tc != 2 {
			t.Errorf("contents at %d, title at %d", cc, tc)
		}
		wantColor(t, v, "中身の一行目", "")
		wantColor(t, v, "-old", "red")
		wantColor(t, v, "+new", "green")
		if strings.TrimSpace(v.Line(rowOf(t, v, "+new")+1)) != "" {
			t.Errorf("no blank line after opened contents:\n%s", v.dump())
		}
		if r, c := v.Find("中身あり"); !v.AttrAt(r, c).bold {
			t.Errorf("an opened Item's title is bold")
		}
		v.Keys("enter")
		v.WaitGone("中身の一行目")
	})
	t.Run("note shows as 理由: with a faint head word", func(t *testing.T) {
		v.In(t)
		v.MoveTo("note あり")
		v.Keys("enter")
		v.WaitFor("理由: 理由の文")
		wantColor(t, v, "理由:", "dim")
		wantColor(t, v, "理由の文", "")
	})
	t.Run("nested Fold indented 2 past its Fold's title; inner Fold in the plain color", func(t *testing.T) {
		v.In(t)
		v.MoveTo("▸ 外")
		v.Keys("enter")
		v.WaitFor("中の項目")
		outer := colOfText(t, v, "外")
		// The inner mark 2 right of the Fold title's first character, so
		// the inner title 4 right; inner text in the same column.
		if c := colOfText(t, v, "中の項目"); c != outer+4 {
			t.Errorf("inner item at %d, Fold title at %d:\n%s", c, outer, v.dump())
		}
		if c := colOfText(t, v, "ただの行"); c != outer+4 {
			t.Errorf("inner text at %d, Fold title at %d", c, outer)
		}
		wantColor(t, v, "▸ 内", "")
		v.MoveTo("内")
		v.Keys("enter")
		v.WaitFor("さらに中")
		if c := colOfText(t, v, "さらに中"); c != outer+8 {
			t.Errorf("Fold in Fold at %d, want %d:\n%s", c, outer+8, v.dump())
		}
		// The cursor column stays at the left edge.
		if r := v.CursorRow(); r < 0 || !strings.Contains(v.Line(r), "内") {
			t.Errorf("cursor:\n%s", v.dump())
		}
	})
}

// SPEC-format.md "描き方": titles are never cut; they wrap at the width
// less the right-end word and 2, aligned under the first character.
func TestRenderWrapping(t *testing.T) {
	t.Parallel()
	const title = "とても長い項目の題で右端の語と重ならないように折り返されるべき文章です"
	report := `<Section title="s">
  <Item id="a" status="状態">` + title + `</Item>
  <Item id="b" status="とても長い状態の語でとても長い">短い題</Item>
  <Item id="c">` + title + `</Item>
</Section>
`
	for _, w := range []int{40, 80, 120} {
		t.Run("w="+itoa(w), func(t *testing.T) {
			v, _ := open(t, report, w, 30, "短い題")
			if strings.Count(joinedWithout(v, "状態"), title) != 2 {
				t.Fatalf("a title is cut at width %d:\n%s", w, v.dump())
			}
			// Item a: the right word on the first row; continuation rows
			// aligned with the title and clear of the word's column.
			r := rowOf(t, v, "とても長い項目")
			c := colOfText(t, v, "とても長い項目")
			first := v.Line(r)
			if !strings.HasSuffix(first, "状態") {
				t.Errorf("right word not on the first row: %q", first)
			}
			limit := w - 2 - colOf("状態") - 2
			for i := r; i < r+5 && i < 30; i++ {
				l := v.Line(i)
				if i > r && (strings.TrimSpace(l) == "" || strings.Contains(l, "短い題")) {
					break
				}
				text := strings.TrimSuffix(strings.TrimRight(l, " "), "状態")
				if end := colOf(strings.TrimRight(text, " ")); end > limit {
					t.Errorf("w=%d row %d runs into the right-word column (%d > %d): %q", w, i, end, limit, l)
				}
				if i > r && colOf(l)-colOf(strings.TrimLeft(l, " ")) != c {
					t.Errorf("w=%d continuation not aligned with the title: %q", w, l)
				}
			}
			// Item b: a right word over half the width goes under the
			// title, right-aligned, whole.
			if !strings.Contains(joined(v), "とても長い状態の語でとても長い") {
				t.Errorf("right word cut:\n%s", v.dump())
			}
			rb := rowOf(t, v, "短い題")
			if colOf("とても長い状態の語でとても長い") > w/2 {
				if strings.Contains(v.Line(rb), "とても長い状態") {
					t.Errorf("w=%d: a wide right word stays on the title row:\n%s", w, v.dump())
				}
				// Right-aligned on the rows under the title (wrapped when it
				// does not fit even there).
				under := strings.TrimSpace(v.Line(rb+1)) + strings.TrimSpace(v.Line(rb+2))
				if colOf(v.Line(rb+1)) != w-2 || !strings.HasPrefix(under, "とても長い状態の語でとても長い") {
					t.Errorf("w=%d: not right-aligned under the title:\n%s", w, v.dump())
				}
			} else if !strings.HasSuffix(v.Line(rb), "とても長い状態の語でとても長い") {
				t.Errorf("w=%d: right word not on the title row:\n%s", w, v.dump())
			}
			// No row runs past the width; wide characters take 2 columns.
			for i, l := range v.Lines() {
				if colOf(l) > w {
					t.Errorf("row %d is %d wide", i, colOf(l))
				}
			}
		})
	}
}

const askNoticeReport = `<Notice id="n1" at="2020-11-22 21:46">開き直してください</Notice>
<Notice id="n2">中身のあるお知らせ
  手順の一行目
</Notice>
<Title>題</Title>
<Section title="あなたへ">
  <Ask id="unit">ページの単位
    <Option key="a">セッションごと</Option>
    <Option key="b">プロジェクトごと</Option>
  </Ask>
  <Item id="a1" re="1">結論の一文
    詳しい答え
  </Item>
  <Item id="a2" re="99">番号違いの返事</Item>
</Section>
`

// SPEC-format.md "描き方": Ask, Notice and re.
func TestRenderAskNoticeRe(t *testing.T) {
	t.Parallel()
	v, _ := open(t, askNoticeReport, 80, 30, "番号違いの返事",
		ev(1, "thought", `"text":"質問の文\nです"`))

	t.Run("Notice: pinned on top, yellow title, ! or ▸, at faint at the right", func(t *testing.T) {
		v.In(t)
		if !strings.Contains(v.Line(0), "開き直してください") {
			t.Errorf("Notice not on the first row:\n%s", v.dump())
		}
		wantColor(t, v, "開き直してください", "yellow")
		wantColor(t, v, "! 開き直して", "yellow")
		wantColor(t, v, "▸ 中身のあるお知らせ", "yellow")
		if !strings.HasSuffix(v.Line(0), "11/22 21:46") {
			t.Errorf("at not at the right end: %q", v.Line(0))
		}
		wantColor(t, v, "11/22 21:46", "dim")
	})
	t.Run("Ask: closed shows the question only", func(t *testing.T) {
		v.In(t)
		if strings.Contains(v.Screen(), "セッションごと") {
			t.Errorf("Ask starts open:\n%s", v.dump())
		}
		r := rowOf(t, v, "ページの単位")
		if !strings.Contains(v.Line(r), "? ページの単位") {
			t.Errorf("Ask head: %q", v.Line(r))
		}
	})
	t.Run("re: the thought quoted as the first row, the title under it", func(t *testing.T) {
		v.In(t)
		r := rowOf(t, v, "> 質問の文 です")
		if !strings.Contains(v.Line(r+1), "結論の一文") {
			t.Errorf("title not under the quote:\n%s", v.dump())
		}
		if colOfText(t, v, "結論の一文") != colOfText(t, v, "質問の文") {
			t.Errorf("title not aligned with the quote's first character")
		}
		wantColor(t, v, "質問の文", "")
		// No such thought: an ordinary Item.
		if strings.Contains(v.Line(rowOf(t, v, "番号違いの返事")), ">") {
			t.Errorf("re to a missing thought quotes something")
		}
	})
	t.Run("Ask opened: options indented; an answer marks one and shows at the right when closed", func(t *testing.T) {
		v.In(t)
		v.MoveTo("ページの単位")
		v.Keys("enter")
		v.WaitFor("セッションごと", "プロジェクトごと")
		if colOfText(t, v, "セッションごと") <= colOfText(t, v, "ページの単位") {
			t.Errorf("options not indented:\n%s", v.dump())
		}
		v.Keys("b")
		v.Until("an answer mark", func() bool {
			l := v.Line(rowOf(t, v, "プロジェクトごと"))
			return l != v.Line(rowOf(t, v, "セッションごと")) && strings.Contains(v.Screen(), "b 未送信")
		})
		v.Keys("enter")
		v.WaitGone("セッションごと")
		if l := v.Line(rowOf(t, v, "ページの単位")); !strings.Contains(l, "b") || !strings.Contains(l, "プロジェク") {
			t.Errorf("closed answered Ask shows no answer: %q", l)
		}
	})
	t.Run("Notice opens with enter", func(t *testing.T) {
		v.In(t)
		v.MoveTo("中身のあるお知らせ")
		v.Keys("enter")
		v.WaitFor("手順の一行目")
		wantColor(t, v, "▾ 中身のあるお知らせ", "yellow")
	})
}

// SPEC.md "画面": an empty report.
func TestRenderEmptyReport(t *testing.T) {
	t.Parallel()
	v, _ := open(t, "\n", 60, 10, "報告はまだ空です。AI が書くと、ここに出ます。")
	wantColor(t, v, "報告はまだ空です", "dim")
}

// SPEC.md "お知らせ": pinned however far the person scrolls.
func TestRenderNoticePinnedWhileScrolling(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("<Notice id=\"n\" at=\"2020-11-22 21:46\">お知らせの題</Notice>\n<Section title=\"s\">\n")
	for i := 0; i < 60; i++ {
		b.WriteString("  <Item id=\"i" + itoa(i) + "\">項目 " + itoa(i) + "</Item>\n")
	}
	b.WriteString("</Section>\n")
	v, _ := open(t, b.String(), 50, 15, "お知らせの題")
	for i := 0; i < 40; i++ {
		v.Keys("j")
	}
	v.WaitCursorOn("項目 39")
	if !strings.Contains(v.Line(0), "お知らせの題") {
		t.Errorf("Notice scrolled away:\n%s", v.dump())
	}
	if strings.Contains(v.Screen(), "項目 0\n") {
		t.Errorf("did not scroll")
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
