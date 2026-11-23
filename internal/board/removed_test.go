package board

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/report"
)

func writeReport(t *testing.T, st *Store, src string) {
	t.Helper()
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A row the person has not answered (look, ask, an Ask), removed, is
// warned of until acknowledged or back; one they reacted to, hid or read
// is not.
func TestRemovedUnanswered(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, `<Item id="a-flat" look="y">> storybook がいいとは言っていない、フラットな一覧はメンテナブルか</Item>
<Item id="ok" ask="y">y を付けた行</Item>
<Item id="hid" look="y">隠した行</Item>
<Item id="plain">ただの項目</Item>
`)
	if w := st.Removed(); len(w) != 0 {
		t.Fatalf("nothing removed yet: %q", w)
	}
	st.Append(Event{Op: "react", ID: "ok", Value: "y"})
	nodes, _, _ := report.Parse("<Item id=\"hid\" look=\"y\">隠した行</Item>\n")
	st.Append(Event{Op: "hide", Key: report.HideKey(nodes[0])})
	writeReport(t, st, "<Item id=\"other\">別の行</Item>\n")
	w := st.Removed()
	want := `warning: row "a-flat" (> storybook がいいとは言っていない…) was removed but the person has not hidden or answered it`
	if len(w) < 1 || w[0] != want {
		t.Fatalf("warnings %q", w)
	}
	for _, x := range w {
		if strings.Contains(x, `"ok"`) || strings.Contains(x, `"plain"`) {
			t.Errorf("warned of an answered or plain row: %q", x)
		}
	}
	// It stays until acknowledged.
	if w2 := st.Removed(); len(w2) != len(w) {
		t.Errorf("second time %q", w2)
	}
	if note := st.ProblemNote(); !strings.Contains(note, "0 problem(s), ") || !strings.Contains(note, "warning(s)") {
		t.Errorf("note %q", note)
	}
	if n, err := st.AckRemoved(); err != nil || n == 0 {
		t.Fatalf("ack %d %v", n, err)
	}
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("after ack: %q", w)
	}
	if note := st.ProblemNote(); note != "" {
		t.Errorf("note after ack %q", note)
	}
}

// A removed row with a comment not settled done is warned of until the
// comment is settled done, or the row comes back.
func TestRemovedOpenComment(t *testing.T) {
	st := testStore(t)
	writeReport(t, st, "<Item id=\"x\">行</Item>\n")
	st.Removed()
	e, _ := st.Append(Event{Op: "thought", ID: "x", Text: "これは？"})
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	want := `warning: row "x" (行) was removed but it has an open comment (` + strconv.Itoa(e.N) + `)`
	if w := st.Removed(); len(w) != 1 || w[0] != want {
		t.Fatalf("warnings %q", w)
	}
	// Back: no warning.
	writeReport(t, st, "<Item id=\"x\">行</Item>\n")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("back: %q", w)
	}
	writeReport(t, st, "<Item id=\"y\">別の行</Item>\n")
	if w := st.Removed(); len(w) != 1 {
		t.Errorf("gone again: %q", w)
	}
	st.SettleAll([]Settlement{{e.N, "doing", "見ています"}})
	if w := st.Removed(); len(w) != 1 {
		t.Errorf("doing is still open: %q", w)
	}
	st.Settle(e.N, "答えました")
	if w := st.Removed(); len(w) != 0 {
		t.Errorf("settled done: %q", w)
	}
}
