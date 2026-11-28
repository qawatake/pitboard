package viewer

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// On the comments screen a narrow line of keys leaves e out before enter
// and g: reading a comment and going to its line come first.
func TestCommentsHintsNarrow(t *testing.T) {
	st := testStore(t)
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n\n<Item id=\"a\">項目</Item>\n"), 0o644)
	st.Append(board.Event{Op: "thought", ID: "a", Title: "項目", Text: "これ"})
	for w := 30; w <= 100; w += 2 {
		a := testApp(t, st)
		a.t.w, a.t.h = w, 24
		wait(t, time.Hour)
		a.handle(key{r: 'l'})
		a.render()
		s := hintLine(a)
		if strings.Contains(s, "e 直す") && (!strings.Contains(s, "enter 開く") || !strings.Contains(s, "g 行へ")) {
			t.Errorf("%d: e kept over enter or g: %q", w, s)
		}
		if w == 60 && !strings.Contains(s, "enter 開く") {
			t.Errorf("%d: no enter: %q", w, s)
		}
	}
}
