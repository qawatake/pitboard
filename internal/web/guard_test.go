package web

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/page/guardtable"
)

// The browser gets what page's guard table says, key by key: the page only
// reports how the cursor came to the line.
func TestGuardTable(t *testing.T) {
	for _, c := range guardtable.Cases {
		t.Run(c.Name, func(t *testing.T) {
			f := setupWith(t, guardtable.Report)
			f.page("tab1")
			tok := f.thing(c.Line).Token
			cur := Cur{How: "person", Open: c.Open, Sel: tok}
			switch c.How {
			case guardtable.Placed:
				cur.Start = true
			case guardtable.Locked:
				cur.Sel = f.thing("restart").Token // the line it came to went
				if c.Line == "restart" {
					cur.Sel = f.thing("a").Token
				}
			case guardtable.Settling:
				cur.How, cur.Ago = "app", 50
			case guardtable.Settled:
				cur.How, cur.Ago = "app", int(page.SettleDelay.Milliseconds())
			}
			a := Act{Action: "key", Key: c.Key, Target: c.Line, Seen: tok, Cur: cur}
			r := f.act("tab1", a)
			if c.Twice {
				r = f.act("tab1", a)
			}
			if r.Do != string(c.Do) || (c.Do == page.DoNothing && r.Message != c.Msg) {
				t.Fatalf("%+v, want do %q msg %q", r, c.Do, c.Msg)
			}
			op := ""
			if evs := f.st.Events(); len(evs) > 0 {
				op = evs[len(evs)-1].Op
			}
			if op != c.Op {
				t.Errorf("recorded %q, want %q", op, c.Op)
			}
			// The buttons never offer a key that is refused.
			if c.Do == page.DoNothing && !c.Twice {
				h := f.act("tab1", Act{Action: "hint", Target: c.Line, Seen: tok, Cur: cur})
				for _, x := range h.Offer.Hints {
					if x.Key == c.Key {
						t.Errorf("offered %q", c.Key)
					}
				}
			}
		})
	}
}

func TestHint(t *testing.T) {
	f := setupWith(t, guardtable.Report)
	f.page("tab1")
	tok := f.thing("a").Token
	h := f.act("tab1", Act{Action: "hint", Target: "a", Seen: tok, Cur: Cur{How: "app", Ago: 100, Sel: tok}})
	if h.Offer == nil || h.Offer.Recheck != 300 || len(h.Offer.Hints) != 1 || h.Offer.Hints[0].Key != "c" {
		t.Fatalf("settling hint %+v", h.Offer)
	}
	h = f.act("tab1", Act{Action: "hint", Target: "a", Seen: tok, Cur: Cur{Start: true, How: "person", Sel: tok}})
	var keys []string
	for _, x := range h.Offer.Hints {
		keys = append(keys, x.Key+" "+x.Word)
	}
	if got := strings.Join(keys, "|"); got != "j k 動かすと y n が効く|c 行に書く|x 隠す" {
		t.Errorf("placed hint %s", got)
	}
}

// A line whose status changed (not what it is) is not locked, but what the
// person saw is checked: MsgChanged.
func TestStatusChangeIsChanged(t *testing.T) {
	f := setupWith(t, guardtable.Report)
	f.page("tab1")
	tok := f.thing("a").Token
	rewrite(t, f, strings.Replace(guardtable.Report, `<Item id="a" ask="y">`, `<Item id="a" ask="y" status="見て">`, 1))
	r := f.act("tab1", Act{Action: "key", Key: "y", Target: "a", Seen: tok, Cur: Cur{How: "person", Sel: tok}})
	if r.OK || !r.Changed || r.Message != page.MsgChanged {
		t.Errorf("%+v", r)
	}
}

func rewrite(t *testing.T, f *fixture, src string) {
	t.Helper()
	os.WriteFile(f.st.ReportPath(), []byte(src), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(f.st.ReportPath(), future, future)
}

// A hidden line the AI rewrote comes back, and the page says so once.
func TestReappearedSaid(t *testing.T) {
	f := setupWith(t, guardtable.Report)
	f.page("tab1")
	if r := f.act("tab1", Act{Action: "hide", Target: "a", Seen: f.thing("a").Token}); !r.OK {
		t.Fatal(r)
	}
	f.page("tab1")
	rewrite(t, f, strings.Replace(guardtable.Report, "項目 a", "項目 A", 1))
	if p := f.page("tab1"); p.Say != page.MsgReappeared+"項目 A" {
		t.Errorf("say %q", p.Say)
	}
	if p := f.page("tab1"); p.Say != "" {
		t.Errorf("said again %q", p.Say)
	}
}

func TestPageETag(t *testing.T) {
	f := setup(t)
	resp := f.req("GET", "/api/page", nil, nil)
	tag := resp.Header.Get("ETag")
	if tag == "" {
		t.Fatal("no etag")
	}
	resp = f.req("GET", "/api/page", nil, func(r *http.Request) { r.Header.Set("If-None-Match", tag) })
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("same page: %d", resp.StatusCode)
	}
	checkHeaders(t, resp)
	f.act("tab1", Act{Action: "comment", Text: "x"})
	resp = f.req("GET", "/api/page", nil, func(r *http.Request) { r.Header.Set("If-None-Match", tag) })
	if b, _ := io.ReadAll(resp.Body); resp.StatusCode != 200 || len(b) == 0 {
		t.Errorf("changed page: %d", resp.StatusCode)
	}
}

func TestRefusals(t *testing.T) {
	f := setup(t)
	n := len(f.st.Events())
	if r := f.act("tab1", Act{Action: "retract", N: 99999}); r.OK || r.Message != page.ErrNoComment.Error() {
		t.Errorf("retract %+v", r)
	}
	if r := f.act("tab1", Act{Action: "edit", N: 99999, Text: "x"}); r.OK || r.Message != page.ErrNoComment.Error() {
		t.Errorf("edit %+v", r)
	}
	if len(f.st.Events()) != n {
		t.Errorf("recorded %v", f.ops())
	}
	r := f.act("tab1", Act{Action: "comment", Text: strings.Repeat("あ", maxJSON)})
	if r.OK || r.Message != page.MsgTooLong {
		t.Errorf("too long %+v", r)
	}
}
