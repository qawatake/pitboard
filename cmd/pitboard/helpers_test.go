package main

import (
	"os"
	"testing"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

const badReport = "<Item id=\"a\" wait=\"me\">x</Item>\n<Item id=\"b\" wait=\"me\">y</Item>\n"

func badStore(t *testing.T) *board.Store {
	t.Helper()
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(badReport), 0o644)
	if _, probs, _ := report.Parse(badReport); len(probs) != 2 {
		t.Fatalf("the bad report has %d problems", len(probs))
	}
	return st
}

const wantNote = "report.md has 2 problem(s), 0 warning(s). Run: pitboard -s s1 check\n"

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
