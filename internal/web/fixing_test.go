package web

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// A report that fails check is not shown: the page keeps the last good
// version and says MsgFixing; a line the same in the broken file can
// still be acted on, a rewritten one cannot.
func TestFixingKeepsTheGoodVersion(t *testing.T) {
	f := setup(t)
	f.page("tab1")
	hook := f.thing("hook").Token
	bad := strings.Replace(src, "hook を入れるか決める", "hook を入れない", 1) +
		"<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n"
	os.WriteFile(f.st.ReportPath(), []byte(bad), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(f.st.ReportPath(), future, future)
	p := f.page("tab1")
	if p.Fixing != page.MsgFixing || len(p.Problems) != 0 {
		t.Fatalf("broken: fixing %q problems %v", p.Fixing, p.Problems)
	}
	if f.things["averyveryverylongid17"] != nil || f.thing("hook").Token != hook {
		t.Errorf("the broken version is shown")
	}
	if r := f.act("tab1", Act{Action: "react", Target: "unit", Seen: f.thing("unit").Token, Value: "a"}); !r.OK {
		t.Errorf("answer on a line the same in the file: %+v", r)
	}
	if r := f.act("tab1", Act{Action: "hide", Target: "hook", Seen: hook}); r.OK || r.Message != page.MsgChanged {
		t.Errorf("hide on a line rewritten in the file: %+v", r)
	}
	// Fixed.
	os.WriteFile(f.st.ReportPath(), []byte(strings.Replace(src, "hook を入れるか決める", "hook を入れない", 1)), 0o644)
	future = future.Add(2 * time.Second)
	os.Chtimes(f.st.ReportPath(), future, future)
	if p := f.page("tab1"); p.Fixing != "" || f.thing("hook").Token == hook {
		t.Errorf("fixed: fixing %q", p.Fixing)
	}
}
