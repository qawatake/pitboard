package viewer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// clipTimeout is how long getting the clipboard's image may take.
const clipTimeout = 3 * time.Second

// clipboardImage gets the image on the clipboard as PNG (tests replace
// it). board.ErrNoImage when there is none, or no way to get it.
var clipboardImage = func(ctx context.Context) ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		return macClipboard(ctx)
	default:
		for _, c := range [][]string{
			{"wl-paste", "--type", "image/png"},
			{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"},
		} {
			if _, err := exec.LookPath(c[0]); err != nil {
				continue
			}
			if b, err := limitedOutput(ctx, exec.CommandContext(ctx, c[0], c[1:]...)); err == nil && len(b) > 0 {
				return b, nil
			} else if errors.Is(err, board.ErrTooLarge) {
				return nil, err
			}
		}
		return nil, board.ErrNoImage
	}
}

// macClipboard writes the clipboard's PNG (or TIFF, turned into PNG with
// sips) to a temporary file through osascript.
func macClipboard(ctx context.Context) ([]byte, error) {
	dir, err := os.MkdirTemp("", "pitboard-clip-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	png, tiff := filepath.Join(dir, "clip.png"), filepath.Join(dir, "clip.tiff")
	save := func(class, path string) error {
		script := []string{
			"set d to (the clipboard as «class " + class + "»)",
			"set f to open for access (POSIX file " + strconv.Quote(path) + ") with write permission",
			"write d to f",
			"close access f",
		}
		var args []string
		for _, l := range script {
			args = append(args, "-e", l)
		}
		return exec.CommandContext(ctx, "osascript", args...).Run()
	}
	if err := save("PNGf", png); err != nil {
		if err := save("TIFF", tiff); err != nil {
			return nil, board.ErrNoImage
		}
		if err := exec.CommandContext(ctx, "sips", "-s", "format", "png", tiff, "--out", png).Run(); err != nil {
			return nil, board.ErrNoImage
		}
	}
	fi, err := os.Stat(png)
	if err != nil || fi.Size() == 0 {
		return nil, board.ErrNoImage
	}
	if fi.Size() > board.ImageMax {
		return nil, board.ErrTooLarge
	}
	return os.ReadFile(png)
}

// limitedOutput runs c and returns its output, up to board.ImageMax.
func limitedOutput(ctx context.Context, c *exec.Cmd) ([]byte, error) {
	var out bytes.Buffer
	c.Stdout = &limitWriter{w: &out, n: board.ImageMax + 1}
	c.Stderr = io.Discard
	c.WaitDelay = 500 * time.Millisecond
	err := c.Run()
	if out.Len() > board.ImageMax {
		return nil, board.ErrTooLarge
	}
	return out.Bytes(), err
}

type limitWriter struct {
	w io.Writer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil // dropped: already too large
	}
	k := min(len(p), l.n)
	l.n -= k
	if _, err := l.w.Write(p[:k]); err != nil {
		return 0, err
	}
	return len(p), nil
}

// pasteImage is ctrl+v in the editor: the clipboard's image goes in as
// [Image #N] at the cursor.
func (a *app) pasteImage() {
	if !a.canWrite() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipTimeout)
	defer cancel()
	data, err := clipboardImage(ctx)
	if err == nil && len(data) == 0 {
		err = board.ErrNoImage
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("クリップボードの画像を %v で取り出せませんでした", clipTimeout)
		}
		a.msg = err.Error()
		return
	}
	n, err := a.st.SaveImage(data, ".png")
	if err != nil {
		a.msg = err.Error()
		return
	}
	a.in.ed.Insert([]rune(fmt.Sprintf("[Image #%d]", n))...)
}

// pastedImage is called when a paste ends in the editor: if what was pasted
// (from the rune index start to the cursor) is the path of one image file,
// it is copied and replaced by [Image #N].
func (a *app) pastedImage(start int) {
	e := &a.in.ed
	if start < 0 || start > e.pos || !a.canWrite() {
		return
	}
	p := board.ImageFileOf(string(e.text[start:e.pos]))
	if p == "" {
		return
	}
	n, err := a.st.CopyImage(p)
	if err != nil {
		a.msg = err.Error()
		return
	}
	e.cut(start, e.pos)
	e.Insert([]rune(fmt.Sprintf("[Image #%d]", n))...)
}

// withImages is s drawn in style base, with "[Image #N]" faint.
func withImages(s, base string) text {
	var t text
	for {
		m := board.ReImageRef.FindStringIndex(s)
		if m == nil {
			return append(t, plainText(s, base)...)
		}
		t = append(t, plainText(s[:m[0]], base)...)
		t = append(t, plainText(s[m[0]:m[1]], base+dim)...)
		s = s[m[1]:]
	}
}
