package viewer

import (
	"os"
	"strings"
	"testing"
)

const listsReport = `<Item id="tight">短い箇条
  - いち
  - に
  - さん
</Item>
<Item id="loose">長い箇条
  - いち
  - 四十の幅では折り返すが八十なら一行に収まる長さ
  - さん
</Item>
<Item id="def">頭の語
  - **理由**: これは頭の語のある行で、とても長いので、狭い幅では頭の語だけを一行に置き、中身をその下に一段下げて続ける
  - **短い**: 一行
</Item>
`

func openAll(t *testing.T, w int) []string {
	t.Helper()
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(listsReport), 0o644)
	a := testApp(t, st)
	a.t.w, a.t.h = w, 80
	a.render()
	for _, id := range []string{"tight", "loose", "def"} {
		a.top().open[id] = true
	}
	_ = strings.TrimSpace
	ls, _ := screen(t, a)
	return ls
}

// rowsBetween is the screen's rows from the one holding from up to the
// one holding to.
func rowsBetween(ls []string, from, to string) []string {
	start := -1
	for i, l := range ls {
		if start < 0 && strings.Contains(l, from) {
			start = i
		}
		if start >= 0 && strings.Contains(l, to) {
			return ls[start : i+1]
		}
	}
	return nil
}

// FIX19: a list is loose (a blank row between all its lines) when any
// of its lines wraps at the width drawn; otherwise tight.
func TestLooseLists(t *testing.T) {
	for _, c := range []struct {
		w     int
		loose bool
	}{{40, true}, {80, false}, {120, false}} {
		ls := openAll(t, c.w)
		tight := rowsBetween(ls, "· いち", "· さん")
		if len(tight) != 3 {
			t.Errorf("width %d: the tight list is %d rows: %q", c.w, len(tight), tight)
		}
		loose := rowsBetween(ls, "長い箇条", "· さん")
		blank := 0
		for _, l := range loose {
			if strings.TrimSpace(l) == "" {
				blank++
			}
		}
		if (blank >= 2) != c.loose {
			t.Errorf("width %d: loose %v, rows %q", c.w, c.loose, loose)
		}
	}
}

// FIX19: "**head**: body" stays run-in when it fits one row, else the
// head goes alone on its row and the body one step in under it.
func TestHeadWordLayout(t *testing.T) {
	ls := openAll(t, 60)
	var head, body string
	for i, l := range ls {
		if strings.Contains(l, "· 理由") {
			head, body = l, ls[i+1]
		}
	}
	if head == "" || strings.Contains(head, "理由:") || strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(head), "·")) != "理由" {
		t.Fatalf("head row %q", head)
	}
	if strings.Index(body, "これは") <= strings.Index(head, "理由") {
		t.Errorf("body not one step in: %q / %q", head, body)
	}
	if !strings.Contains(strings.Join(ls, "\n"), "短い: 一行") {
		t.Errorf("a short one is not run-in:\n%s", strings.Join(ls, "\n"))
	}
	// Measure: no list text runs past 72 columns of text on a wide screen.
	for _, l := range openAll(t, 200) {
		if strings.Contains(l, "これは") && width(strings.TrimRight(l, " ")) > 72+lgutter+2*markW+2 {
			t.Errorf("past the measure: %d %q", width(l), l)
		}
	}
}
