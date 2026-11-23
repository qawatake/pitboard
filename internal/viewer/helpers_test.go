package viewer

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// screen draws a and returns the screen lines without styles, and the raw
// output.
func screen(t *testing.T, a *app) ([]string, string) {
	t.Helper()
	var buf bytes.Buffer
	a.t.out = &buf
	a.t.prev = nil // the whole frame
	a.draw()
	raw := buf.String()
	body := strings.TrimPrefix(raw, "\x1b[H")
	var lines []string
	for _, l := range strings.Split(body, "\r\n") {
		l = strings.ReplaceAll(l, "\x1b[2K", "")
		lines = append(lines, reEsc.ReplaceAllString(l, ""))
	}
	return lines, raw
}

func plainLines(ls []string) string {
	var out []string
	for _, l := range ls {
		out = append(out, reEsc.ReplaceAllString(l, ""))
	}
	return strings.Join(out, "\n")
}

// ---- hiding lines ----

// wait moves the viewer's clock on by d.
func wait(t *testing.T, d time.Duration) {
	t.Helper()
	base := clock()
	prev := clock
	clock = func() time.Time { return base.Add(d) }
	t.Cleanup(func() { clock = prev })
}

// testApp is a viewer without a terminal, on a store's report.
func testApp(t *testing.T, st *board.Store) *app {
	t.Helper()
	a := &app{t: &term{w: 80, h: 30, out: io.Discard}, base: st.Base, lopen: map[int]bool{}}
	a.setStore(st)
	return a
}

var reEsc = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)

const noticeReport = `<Notice id="restart">pitboard を開き直してください。新しい版を入れました</Notice>
<Title>題</Title>

<Section title="s">
<Item id="a" ask="y">項目 a</Item>
</Section>
`

var richItem = strings.Replace(`<Item id="where">データをどこに置くか
  おすすめは折衷。

  - **cwd に置く**
    - 良い点: 書ける
      続きの行
    - 悪い点: 見つからない
  - **折衷**: データは cwd

  **手順**
  1. まず cwd
  2. 次に XDG
    - 入れ子
  行末で改行する<BR>
  次の行\
  その次の行
  **語**: 見出し語の段落
</Item>`, "<BR>", "  ", 1)

func changes(t *testing.T, st *board.Store) string {
	t.Helper()
	sendAll(st)
	lines, err := st.Changes()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

// sendAll marks everything so far as sent (what s does on the board).
func sendAll(st *board.Store) {
	st.Append(board.Event{Op: "send"})
}

func testStore(t *testing.T) *board.Store {
	t.Helper()
	st := board.Open(t.TempDir(), "s1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `<Section title="あなたへ">
<Ask id="unit">ページの単位
<Option key="a">セッションごと</Option>
<Option key="b">プロジェクトごと</Option>
</Ask>
<Item id="hook">hook を入れるか決める</Item>
</Section>`
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return st
}
