package viewer

import (
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/report"
)

func TestTruncLongLineIsFast(t *testing.T) {
	long := plainText(strings.Repeat("あ", 300000), "")
	start := time.Now()
	got := long.trunc(70)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("trunc of 300k cells took %v", d)
	}
	if got.width() > 70 || got[len(got)-1].r != '…' {
		t.Errorf("trunc = %q (width %d)", got.plain(), got.width())
	}
	if s := plainText("abc", "").trunc(3).plain(); s != "abc" {
		t.Errorf("a fitting text is kept: %q", s)
	}
	if s := plainText("abcd", "").trunc(3).plain(); s != "ab…" {
		t.Errorf("trunc(3) = %q", s)
	}
	if s := plainText("あいう", "").trunc(5).plain(); s != "あい…" && s != "あ…" {
		t.Errorf("wide trunc = %q", s)
	}
	if s := plainText("あいう", "").trunc(5); s.width() > 5 {
		t.Errorf("wide trunc too wide: %q", s.plain())
	}
}

func TestPrintable(t *testing.T) {
	s := plainText("a\nb\r\tc\x1b[2Jd\x00", "").String()
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("control character %q reaches the terminal: %q", r, s)
		}
	}
}

func TestLinkURL(t *testing.T) {
	nodes, probs, _ := report.Parse(`<Link id="pr" url="https://example.com/a?b=1">PR</Link>`)
	if len(probs) > 0 || report.ByID(nodes, "pr").URL != "https://example.com/a?b=1" {
		t.Fatalf("link: %v %+v", probs, report.ByID(nodes, "pr"))
	}
	s := plainText("ab", "").hyperlink("https://x.test/\x1b]8;;evil\x07").String()
	if !strings.HasPrefix(s, "\x1b]8;;https://x.test/]8;;evil\x1b\\ab") || !strings.HasSuffix(s, "\x1b]8;;\x1b\\") {
		t.Errorf("hyperlink = %q", s)
	}
}
