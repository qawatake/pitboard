package web

import (
	"os"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// After a send, a report rewritten to fail check is not the AI's answer
// on any screen: the board list and a tab opened afterwards say AI の番
// too.
func TestTurnOnBrokenReportWeb(t *testing.T) {
	f := setup(t)
	t.Setenv("PITBOARD_SEND", "true")
	past := time.Now().Add(-time.Hour)
	os.Chtimes(f.st.ReportPath(), past, past)
	b := page.Open(f.st)
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	bad := src + "\n<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"
	if err := os.WriteFile(f.st.ReportPath(), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	os.Chtimes(f.st.ReportPath(), future, future)
	if p := f.page("fresh"); len(p.Summary) == 0 || p.Summary[0].Text != page.WordAITurn {
		t.Errorf("new tab: %+v", p.Summary)
	}
	ix := f.s.index()
	if len(ix.Boards) != 1 || len(ix.Boards[0].Summary) == 0 || ix.Boards[0].Summary[0].Text != page.WordAITurn {
		t.Errorf("list: %+v", ix.Boards)
	}
}
