package viewer

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// A terminal viewer started after a send, on a report rewritten to fail
// check, says AI の番 as the screens open before do.
func TestTurnFreshViewerOnBrokenReport(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	st := testStore(t)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(st.ReportPath(), past, past)
	b := page.Open(st)
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"), 0o644)
	future := time.Now().Add(time.Second)
	os.Chtimes(st.ReportPath(), future, future)
	a := testApp(t, st)
	lines, _ := screen(t, a)
	if !strings.Contains(lines[0], page.WordAITurn) {
		t.Errorf("fresh viewer: %q", lines[0])
	}
}
