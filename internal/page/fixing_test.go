package page

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// broken is src with a line 'pitboard check' fails on (an id over 16
// characters), and with change applied.
func broken(change ...string) string {
	s := src + "<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"
	for i := 0; i+1 < len(change); i += 2 {
		s = strings.Replace(s, change[i], change[i+1], 1)
	}
	return s
}

// A report that fails check never reaches the person: the last good
// version stays, with a quiet line, until a good one comes.
func TestFixingKeepsTheGoodVersion(t *testing.T) {
	b := open(t, src)
	before := len(b.Page(false).Things)
	write(t, b.Store, broken("hook を **入れる** か決める", "hook を入れない"))
	b.Report.Load()
	p := b.Page(false)
	if p.Fixing != MsgFixing || len(p.Problems) != 0 || p.ProblemHead != "" {
		t.Fatalf("broken: fixing %q problems %v", p.Fixing, p.Problems)
	}
	if len(p.Things) != before || find(p.Things, "averyveryverylongid17") != nil {
		t.Errorf("the broken version is shown")
	}
	hook := find(p.Things, "hook")
	if hook == nil || !strings.Contains(hook.Node.Title(), "入れる") {
		t.Fatalf("hook is not the good version: %+v", hook)
	}
	// A line the same in the broken file: answering it is recorded (what is
	// recorded is true of both versions).
	unit := find(p.Things, "unit")
	if _, err := b.React(unit.Seen, "a"); err != nil {
		t.Errorf("answer on a line the same in the file: %v", err)
	}
	// A line rewritten in the broken file: refused, as for any rewrite.
	if _, _, err := b.Hide(hook.Seen, false); !errors.Is(err, ErrChanged) {
		t.Errorf("hide on a line rewritten in the file: %v", err)
	}
	if b.Page(false).Fixing != MsgFixing {
		t.Errorf("an action took the broken version in")
	}

	// A good version comes: it is shown, and the line goes.
	fixed := strings.Replace(src, "hook を **入れる** か決める", "hook を入れない", 1)
	write(t, b.Store, fixed)
	b.Report.Load()
	p = b.Page(false)
	if p.Fixing != "" || len(p.Problems) != 0 {
		t.Errorf("fixed: fixing %q problems %v", p.Fixing, p.Problems)
	}
	if h := find(p.Things, "hook"); h == nil || h.Node.Title() != "hook を入れない" {
		t.Errorf("fixed: hook %+v", h)
	}
}

// No good version yet (a broken first report, or only an empty one): the
// problems are shown, as they always were.
func TestFixingNeedsAGoodVersion(t *testing.T) {
	b := open(t, broken())
	p := b.Page(false)
	if p.Fixing != "" || len(p.Problems) == 0 {
		t.Errorf("broken first: fixing %q problems %v", p.Fixing, p.Problems)
	}
	b = open(t, "")
	write(t, b.Store, broken())
	b.Report.Load()
	if p := b.Page(false); p.Fixing != "" || len(p.Problems) == 0 {
		t.Errorf("empty, then broken: fixing %q problems %v", p.Fixing, p.Problems)
	}
	// Removed and written again broken: nothing good to keep.
	b = open(t, src)
	os.Remove(b.Store.ReportPath())
	b.Report.Load()
	write(t, b.Store, broken())
	b.Report.Load()
	if p := b.Page(false); p.Fixing != "" || len(p.Problems) == 0 {
		t.Errorf("removed, then broken: fixing %q problems %v", p.Fixing, p.Problems)
	}
}
