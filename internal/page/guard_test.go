package page_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/page/guardtable"
	"github.com/qawatake/pitboard/internal/report"
)

func openBoard(t *testing.T, src string) (*page.Board, *board.Store) {
	t.Helper()
	st := board.Open(t.TempDir(), "s1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return page.Open(st), st
}

func nodeOf(b *page.Board, key string) *report.Node {
	var found *report.Node
	report.WalkAll(b.Report.Nodes, func(n *report.Node) {
		if found == nil && n.Key == key {
			found = n
		}
	})
	return found
}

// TouchOf is the Touch a case's How stands for.
func touchOf(c guardtable.Case) page.Touch {
	t := page.Touch{Open: c.Open}
	switch c.How {
	case guardtable.Placed:
		t.Placed = true
	case guardtable.Locked:
		t.Locked = true
	case guardtable.Settling:
		t.Auto, t.Shown = true, 50*time.Millisecond
	case guardtable.Settled:
		t.Auto, t.Shown = true, page.SettleDelay
	}
	return t
}

func TestGuardTable(t *testing.T) {
	for _, c := range guardtable.Cases {
		t.Run(c.Name, func(t *testing.T) {
			b, st := openBoard(t, guardtable.Report)
			n := nodeOf(b, c.Line)
			if n == nil {
				t.Fatalf("no line %q", c.Line)
			}
			tc := touchOf(c)
			v := b.Allow(n, c.Key, tc)
			if c.Twice {
				tc.Again = v.Again
				v = b.Allow(n, c.Key, tc)
			}
			if v.Do != c.Do || v.Msg != c.Msg {
				t.Fatalf("Allow = %+v, want do %q msg %q", v, c.Do, c.Msg)
			}
			switch v.Do {
			case page.DoReact:
				if _, err := b.React(page.SeenOf(n), v.Value); err != nil {
					t.Fatal(err)
				}
			case page.DoHide:
				if _, _, err := b.Hide(page.SeenOf(n), false); err != nil {
					t.Fatal(err)
				}
			}
			op := ""
			if evs := st.Events(); len(evs) > 0 {
				op = evs[len(evs)-1].Op
			}
			if op != c.Op {
				t.Errorf("recorded %q, want %q", op, c.Op)
			}
			// The offer never holds a key the guard refuses.
			for _, h := range b.Offer(n, touchOf(c)).Hints {
				if h.Key == c.Key && c.Do == page.DoNothing && !c.Twice {
					t.Errorf("offered %q, which is refused", h.Key)
				}
			}
		})
	}
}

// A press dropped while settling works again only with the same key on
// the same line.
func TestGuardAgainIsPerKey(t *testing.T) {
	b, _ := openBoard(t, guardtable.Report)
	n := nodeOf(b, "a")
	tc := page.Touch{Auto: true}
	v := b.Allow(n, "x", tc)
	tc.Again = v.Again
	if v := b.Allow(n, "y", tc); v.Do != page.DoNothing || v.Msg != page.MsgSettlingReact {
		t.Errorf("y after a dropped x: %+v", v)
	}
}

func TestOffer(t *testing.T) {
	b, _ := openBoard(t, guardtable.Report)
	words := func(o page.Offer) string {
		var out []string
		for _, h := range o.Hints {
			out = append(out, h.Key+" "+h.Word)
		}
		return strings.Join(out, " | ")
	}
	cases := []struct {
		line string
		t    page.Touch
		want string
	}{
		{"a", page.Touch{}, "y よし | n だめ | c 行に書く | x 隠す"},
		{"a", page.Touch{Placed: true}, "j k 動かすと y n が効く | c 行に書く | x 隠す"},
		{"a", page.Touch{Locked: true}, ""},
		{"a", page.Touch{Auto: true}, "c 行に書く"},
		{"b", page.Touch{Placed: true}, "r 読んだ | j k 動かすと y n が効く | c 行に書く | x 隠す"},
		{"b", page.Touch{Auto: true}, "r 読んだ | c 行に書く"},
		{"q", page.Touch{}, "enter 開く | c 行に書く | x 隠す"},
		{"q", page.Touch{Open: true}, "a b 答える | enter 閉じる | c 行に書く | x 隠す"},
		{"q", page.Touch{Open: true, Placed: true}, "j k 動かすと答えられる | enter 閉じる | c 行に書く | x 隠す"},
		{"restart", page.Touch{Placed: true}, "j k 動かすと y n が効く | c 行に書く | x 隠す"},
	}
	for _, c := range cases {
		if got := words(b.Offer(nodeOf(b, c.line), c.t)); got != c.want {
			t.Errorf("%s %+v:\n got %s\nwant %s", c.line, c.t, got, c.want)
		}
	}
	if o := b.Offer(nodeOf(b, "a"), page.Touch{Auto: true, Shown: 100 * time.Millisecond}); o.Recheck != 300*time.Millisecond {
		t.Errorf("recheck %v", o.Recheck)
	}
}

func TestRewrittenAndReappeared(t *testing.T) {
	b, st := openBoard(t, guardtable.Report)
	old := nodeOf(b, "a")
	if _, _, err := b.Hide(page.SeenOf(old), false); err != nil {
		t.Fatal(err)
	}
	hidden := b.HiddenKeys()
	os.WriteFile(st.ReportPath(), []byte(strings.Replace(guardtable.Report, "項目 a", "項目 A", 1)), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(st.ReportPath(), future, future)
	b.Report.Load()
	if msg := b.Reappeared(hidden); msg != page.MsgReappeared+"項目 A" {
		t.Errorf("reappeared %q", msg)
	}
	if !page.Rewritten(old, nodeOf(b, "a")) || page.Rewritten(nodeOf(b, "q"), nodeOf(b, "q")) || !page.Rewritten(old, nil) {
		t.Error("rewritten")
	}
}

func TestSignsAndHeads(t *testing.T) {
	b, _ := openBoard(t, guardtable.Report+`<Fold title="f">
  - x
</Fold>
`)
	p := b.Page(false)
	signs := map[string][2]string{}
	var walk func(ts []*page.Thing)
	walk = func(ts []*page.Thing) {
		for _, x := range ts {
			if x.Stop {
				signs[x.Key] = [2]string{x.Sign, x.OpenSign}
			}
			walk(x.Kids)
		}
	}
	walk(p.Notices)
	walk(p.Things)
	want := map[string][2]string{"restart": {"!", "!"}, "a": {"·", "·"}, "q": {"?", "?"}, "-1:ただの行": {"·", "·"}, "fold:1": {"▸", "▾"}}
	for k, w := range want {
		if signs[k] != w {
			t.Errorf("%s: %v, want %v", k, signs[k], w)
		}
	}
	if got := page.Cut("あいうえおかきくけこ", page.AnswerHead); got != "あいうえお…" {
		t.Errorf("cut %q", got)
	}
	if page.JumpLetter(6) != "h" || page.JumpIndex('h', 9) != 6 || page.JumpIndex('g', 9) != -1 {
		t.Error("jump letters")
	}
}
