package page

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// What a screen that is not the terminal needs from the same rules: names
// for the kinds (to send the page as JSON), a token for Seen, what starts
// open, the counts at the top right, one call that says whether anything
// changed on disk, and the comments' images.

var (
	kindNames  = []string{"notice", "title", "section", "item", "ask", "option", "link", "fold", "para", "list", "code"}
	styleNames = []string{"plain", "bold", "code", "link"}
	toneNames  = []string{"quiet", "need", "move"}
	markNames  = []string{"none", "new", "waiting"}
)

func name(names []string, i int) ([]byte, error) {
	if i < 0 || i >= len(names) {
		return nil, fmt.Errorf("page: no name for %d", i)
	}
	return []byte(names[i]), nil
}

func (k Kind) MarshalText() ([]byte, error)  { return name(kindNames, int(k)) }
func (s Style) MarshalText() ([]byte, error) { return name(styleNames, int(s)) }
func (t Tone) MarshalText() ([]byte, error)  { return name(toneNames, int(t)) }
func (m Mark) MarshalText() ([]byte, error)  { return name(markNames, int(m)) }

func unname(names []string, b []byte) (int, error) {
	for i, n := range names {
		if n == string(b) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("page: unknown name %q", b)
}

func (k *Kind) UnmarshalText(b []byte) (err error) {
	i, err := unname(kindNames, b)
	*k = Kind(i)
	return err
}
func (s *Style) UnmarshalText(b []byte) (err error) {
	i, err := unname(styleNames, b)
	*s = Style(i)
	return err
}
func (t *Tone) UnmarshalText(b []byte) (err error) {
	i, err := unname(toneNames, b)
	*t = Tone(i)
	return err
}
func (m *Mark) UnmarshalText(b []byte) (err error) {
	i, err := unname(markNames, b)
	*m = Mark(i)
	return err
}

// Next is a line "next marked line" goes to, and the Folds to open on the
// way.
type Next struct {
	Key   string
	Folds []string
}

// Token stands for s: equal tokens, equal Seen.
func (s Seen) Token() string {
	h := sha256.Sum256([]byte(s.Key + "\x00" + s.Sig + "\x00" + s.HideKey))
	return hex.EncodeToString(h[:16])
}

// StartsOpen says whether n is open when a screen first shows it: every
// line starts closed (Folds, Asks, notices and Items alike), and opens only
// when the person opens it.
func StartsOpen(n *report.Node) bool { return false }

// Summary is the counts at the top right of the page, in order: 反応待ち N
// (need), 未読 N (move), AI 待機中, 送信済 N (quiet), ● 未送信 N (need).
// Only what is there is listed.
func (b *Board) Summary(showHidden bool) []Word {
	var out []Word
	w, f := b.Counts(showHidden)
	if w > 0 {
		out = append(out, Word{fmt.Sprintf("反応待ち %d", w), ToneNeed})
	}
	if f > 0 {
		out = append(out, Word{fmt.Sprintf("未読 %d", f), ToneMove})
	}
	if b.Waiter {
		out = append(out, Word{"AI 待機中", ToneQuiet})
	}
	if b.Unread > 0 {
		out = append(out, Word{fmt.Sprintf("送信済 %d", b.Unread), ToneQuiet})
	}
	if b.Unsent > 0 {
		out = append(out, Word{fmt.Sprintf("● 未送信 %d", b.Unsent), ToneNeed})
	}
	return out
}

// Changed reads again whatever changed on disk since it was read (the
// report, the record, what was looked at, whether an AI waits) and says
// whether anything did: for a screen that shows the page anew on a change.
func (b *Board) Changed() bool {
	ch := false
	if b.Report.Path != "" && b.Report.Stale() {
		b.Report.Load()
		ch = true
	}
	if b.Store == nil || b.NoSession {
		return ch
	}
	vm, w := b.viewedMod, b.Waiter
	if b.Poll() {
		ch = true
	}
	return ch || !b.viewedMod.Equal(vm) || b.Waiter != w
}

// ErrNotImage is SaveImage given something that is not a PNG, JPEG, GIF
// or WebP image.
var ErrNotImage = errors.New("画像ではないので貼りませんでした (PNG JPEG GIF WebP のみ)")

// imageTypes is the image types accepted, by their sniffed content type.
var imageTypes = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
}

// SaveImage stores data as the session's next image, if its content is an
// image (sniffed, whatever it is called), within board.ImageMax. It
// returns the number for "[Image #N]".
func (b *Board) SaveImage(data []byte) (int, error) {
	if !b.CanWrite() {
		return 0, errCannotWrite
	}
	if len(data) > board.ImageMax {
		return 0, board.ErrTooLarge
	}
	if len(data) == 0 {
		return 0, ErrNotImage
	}
	ext, ok := imageTypes[http.DetectContentType(data)]
	if !ok {
		return 0, ErrNotImage
	}
	return b.Store.SaveImage(data, ext)
}

// ImageFile is the file of the session's image n ("" when there is
// none). What it holds is the screen's to check before showing it.
func (b *Board) ImageFile(n int) string {
	if b.Store == nil || b.NoSession || n <= 0 {
		return ""
	}
	return b.Store.ImageFile(n)
}

// IsImageType says whether ctype (sniffed) is an image type SaveImage takes.
func IsImageType(ctype string) bool { _, ok := imageTypes[ctype]; return ok }
