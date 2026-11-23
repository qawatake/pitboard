package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// An Item with re="N" shows thought N above its title, faint, after a >
// in the title's column; long thoughts end in … on the 3rd row. No such
// thought, no quote. The cursor stops on the title only.
func TestQuoteAThought(t *testing.T) {
	for _, w := range []int{80, 40} {
		st := testStore(t)
		short, _ := st.Append(board.Event{Op: "thought", Text: "これは要る？ [Image #1]"})
		long, _ := st.Append(board.Event{Op: "thought", Text: strings.Repeat("長い質問です。", 40)})
		gone, _ := st.Append(board.Event{Op: "thought", Text: "取り消した"})
		st.Append(board.Event{Op: "retract", Thought: gone.N})
		src := fmt.Sprintf("<Title>題</Title>\n\n<Section title=\"あなたへ\">\n<Item id=\"a1\" re=\"%d\" look=\"y\">要ります\n理由。\n</Item>\n<Item id=\"a2\" re=\"%d\">長い答え</Item>\n<Item id=\"a3\" re=\"%d\">消えた問い</Item>\n<Item id=\"a4\" re=\"999\">無い問い</Item>\n</Section>\n", short.N, long.N, gone.N)
		os.WriteFile(st.ReportPath(), []byte(src), 0o644)
		a := testApp(t, st)
		a.t.w = w
		wait(t, time.Hour)
		a.render()
		ls, raw := screen(t, a)
		title := rowOf(ls, "要ります")
		if title < 1 || !strings.Contains(ls[title-1], "> これは要る？ [Image #1]") {
			t.Fatalf("w=%d: no quote above the title:\n%s", w, strings.Join(ls, "\n"))
		}
		// The > of the quote is where the title's text starts.
		col := func(l, s string) int { return visibleWidth(l[:strings.Index(l, s)]) }
		if col(ls[title-1], "> これ") != col(ls[title], "要ります") {
			t.Errorf("w=%d: quote column:\n%s\n%s", w, ls[title-1], ls[title])
		}
		if !strings.Contains(raw, dim+"> ") {
			t.Errorf("w=%d: the quote is not faint", w)
		}
		lt := rowOf(ls, "長い答え")
		if !strings.HasSuffix(strings.TrimRight(ls[lt-1], " "), "…") || !strings.Contains(ls[lt-3], "> 長い質問") || strings.Contains(ls[lt-4], "長い質問") {
			t.Errorf("w=%d: long quote:\n%s", w, strings.Join(ls, "\n"))
		}
		for _, s := range []string{"消えた問い", "無い問い"} {
			if r := rowOf(ls, s); strings.Contains(ls[r-1], "> ") {
				t.Errorf("w=%d: quote over %s:\n%s", w, s, strings.Join(ls, "\n"))
			}
		}
		// j from a1 goes to a2's title, not its quote.
		pv := a.top()
		pv.cur = "a1"
		a.handle(key{r: 'j'})
		a.render()
		if pv.cur != "a2" {
			t.Errorf("w=%d: cursor %q", w, pv.cur)
		}
		ls, _ = screen(t, a)
		if r := rowOf(ls, "長い答え"); !strings.HasPrefix(ls[r], ">") || strings.HasPrefix(ls[r-1], ">") {
			t.Errorf("w=%d: cursor row:\n%s", w, strings.Join(ls, "\n"))
		}
	}
}
