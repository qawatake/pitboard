package main

import (
	"os"
	"strings"
	"testing"
)

func TestHeadMarksAndDoingWord(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.reportPath(), []byte(`<Item id="a">まだの行</Item>
<Item id="b" state="doing">作業している行</Item>
<Item id="c" state="doing" status="順番待ち">別の語の行</Item>
<Item id="d" state="done">済んだ行</Item>
<Item id="e">開ける行
中身
</Item>
`), 0o644)
	a := testApp(t, st)
	s := shown(a)
	for _, want := range []string{"· まだの行", "· 作業している行", "· 別の語の行", "✓ 済んだ行", "▸ 開ける行"} {
		if !strings.Contains(s, want) {
			t.Errorf("want %q in\n%s", want, s)
		}
	}
	if strings.ContainsAny(s, "○~") {
		t.Errorf("an old mark:\n%s", s)
	}
	for _, l := range strings.Split(s, "\n") {
		switch {
		case strings.Contains(l, "作業している行"):
			if !strings.HasSuffix(l, "作業中") {
				t.Errorf("doing without a status: %q", l)
			}
		case strings.Contains(l, "別の語の行"):
			if !strings.HasSuffix(l, "順番待ち") || strings.Contains(l, "作業中") {
				t.Errorf("doing with a status: %q", l)
			}
		case strings.Contains(l, "まだの行"), strings.Contains(l, "済んだ行"):
			if strings.Contains(l, "作業中") {
				t.Errorf("not doing: %q", l)
			}
		}
	}
	// 作業中 is green, as a doing status is.
	pv := a.top()
	if r := a.right(pv, byID(pv.nodes, "b")); r.plain() != "作業中" || r[0].st != green {
		t.Errorf("right %q", r.plain())
	}
}
