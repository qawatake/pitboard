package board

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/report"
)

const badReport = "<Item id=\"a\" wait=\"me\">x</Item>\n<Item id=\"b\" wait=\"me\">y</Item>\n"

func badStore(t *testing.T) *Store {
	t.Helper()
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte(badReport), 0o644)
	if _, probs, _ := report.Parse(badReport); len(probs) != 2 {
		t.Fatalf("the bad report has %d problems", len(probs))
	}
	return st
}

const wantNote = "report.md has 2 problem(s). Run: pitboard -s s1 check\n"

func changes(t *testing.T, st *Store) string {
	t.Helper()
	sendAll(st)
	lines, err := st.Changes()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

// sendChanges sends what is on the board, then delivers it.
func sendChanges(st *Store) ([]string, error) {
	sendAll(st)
	return st.Changes()
}

func shortPoll(t *testing.T) {
	prev := waitPoll
	waitPoll = 10 * time.Millisecond
	t.Cleanup(func() { waitPoll = prev })
}

// sendAll marks everything so far as sent (what s does on the board).
func sendAll(st *Store) {
	st.Append(Event{Op: "send"})
}

func testStore(t *testing.T) *Store {
	t.Helper()
	st := Open(t.TempDir(), "s1")
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
