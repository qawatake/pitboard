package viewer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// An Item with re="N" shows thought N on its first line (the cursor's):
// > and the text, in the title's color; the title follows on the next
// rows, aligned with the text. Long thoughts end in … on the 3rd row. No
// such thought: a plain Item. Nothing is drawn above the first line.
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
		q := rowOf(ls, "> これは要る？ [Image #1]")
		if q < 1 || !strings.Contains(ls[q+1], "要ります") || !strings.Contains(ls[q-1], "あなたへ") {
			t.Fatalf("w=%d: quote line:\n%s", w, strings.Join(ls, "\n"))
		}
		if !strings.Contains(ls[q], "▸") || !strings.Contains(ls[q], "●") {
			t.Errorf("w=%d: mark not on the quote line:\n%s", w, ls[q])
		}
		// The title starts where the quote's text starts.
		col := func(l, s string) int { return visibleWidth(l[:strings.Index(l, s)]) }
		if col(ls[q], "これ") != col(ls[q+1], "要ります") {
			t.Errorf("w=%d: title column:\n%s\n%s", w, ls[q], ls[q+1])
		}
		if strings.Contains(raw, dim+"> ") {
			t.Errorf("w=%d: the quote is faint", w)
		}
		lq := rowOf(ls, "> 長い質問")
		if !strings.HasSuffix(strings.TrimRight(ls[lq+2], " "), "…") || !strings.Contains(ls[lq+3], "長い答え") || strings.Contains(ls[lq+3], "長い質問") {
			t.Errorf("w=%d: long quote:\n%s", w, strings.Join(ls, "\n"))
		}
		for _, s := range []string{"消えた問い", "無い問い"} {
			if r := rowOf(ls, s); strings.Contains(ls[r], "> ") || strings.Contains(ls[r-1], "> ") {
				t.Errorf("w=%d: quote on %s:\n%s", w, s, strings.Join(ls, "\n"))
			}
		}
		// j from a1 goes to a2's quote line.
		pv := a.top()
		pv.cur = "a1"
		a.handle(key{r: 'j'})
		a.render()
		if pv.cur != "a2" {
			t.Errorf("w=%d: cursor %q", w, pv.cur)
		}
		ls, _ = screen(t, a)
		if r := rowOf(ls, "> 長い質問"); !strings.HasPrefix(ls[r], ">") || strings.HasPrefix(ls[r+1], ">") {
			t.Errorf("w=%d: cursor row:\n%s", w, strings.Join(ls, "\n"))
		}
	}
}
