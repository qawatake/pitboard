package viewer

import (
	"os"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// A report that fails check is not shown: the last good version stays,
// with a quiet line at the top, and the cursor is not locked by it.
func TestFixingKeepsTheGoodVersion(t *testing.T) {
	st := testStore(t)
	good, _ := os.ReadFile(st.ReportPath())
	a := testApp(t, st)
	pv := a.top()
	a.render()
	cur := pv.cur
	bad := strings.Replace(string(good), "hook を入れるか決める", "hook を入れない", 1) +
		"\n<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"
	rewriteReport(t, a, bad)
	a.tickReload()
	lines, _ := screen(t, a)
	s := strings.Join(lines, "\n")
	if !strings.Contains(s, page.MsgFixing) || strings.Contains(s, page.MsgProblems) {
		t.Errorf("broken: no quiet line, or problems shown:\n%s", s)
	}
	if !strings.Contains(s, "hook を入れるか決める") || strings.Contains(s, "長すぎる id") {
		t.Errorf("broken: not the good version:\n%s", s)
	}
	if pv.lock || pv.cur != cur {
		t.Errorf("cursor moved or locked: %q lock %v", pv.cur, pv.lock)
	}
	// Fixed: the new version, and the line goes.
	rewriteReport(t, a, strings.Replace(string(good), "hook を入れるか決める", "hook を入れない", 1))
	a.tickReload()
	lines, _ = screen(t, a)
	s = strings.Join(lines, "\n")
	if strings.Contains(s, page.MsgFixing) || !strings.Contains(s, "hook を入れない") {
		t.Errorf("fixed:\n%s", s)
	}
}

// Started on a broken report: its problems are shown.
func TestFixingStartedBroken(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"), 0o644)
	a := testApp(t, st)
	lines, _ := screen(t, a)
	s := strings.Join(lines, "\n")
	if strings.Contains(s, page.MsgFixing) || !strings.Contains(s, page.MsgProblems) {
		t.Errorf("started broken:\n%s", s)
	}
}
