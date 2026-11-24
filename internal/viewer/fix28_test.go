package viewer

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// The AI answers a send with a good version, then writes one that fails
// check: the viewer open all along and one started now both say AI の番.
func TestTurnAnsweredThenBrokenViewer(t *testing.T) {
	t.Setenv("PITBOARD_SEND", "true")
	st := testStore(t)
	good, _ := os.ReadFile(st.ReportPath())
	past := time.Now().Add(-time.Hour)
	os.Chtimes(st.ReportPath(), past, past)
	a := testApp(t, st)
	b := page.Open(st)
	if _, err := b.Comment("", "", "どう？"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(nil); err != nil {
		t.Fatal(err)
	}
	a.loadEvents()
	at := time.Now()
	write := func(s string) {
		at = at.Add(time.Second)
		os.WriteFile(st.ReportPath(), []byte(s), 0o644)
		os.Chtimes(st.ReportPath(), at, at)
		a.tickReload()
	}
	check := func(name, want string) {
		t.Helper()
		for _, v := range []struct {
			name string
			a    *app
		}{{"open", a}, {"new", testApp(t, st)}} {
			if ls, _ := screen(t, v.a); !strings.Contains(ls[0], want) {
				t.Errorf("%s, %s viewer: %q", name, v.name, ls[0])
			}
		}
	}
	write(string(good))
	check("answered", page.WordYourTurn)
	write(string(good) + "\n<Item id=\"averyveryverylongid17\">長すぎる id</Item>\n")
	check("broken", page.WordAITurn)
}
