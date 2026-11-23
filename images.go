package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Images in thoughts: ctrl+v in the editor (or pasting the path of an image
// file) saves the image as .pitboard/<session>/images/<N>.<ext> and puts
// "[Image #N]" in the text. N counts up per session and is never reused.
// 'changes' gives the AI the path of each image a thought still holds.

const (
	imageMax     = 20 << 20 // 20 MiB
	clipTimeout  = 3 * time.Second
	imageCounter = "last" // in images/: the last number given
)

var (
	reImageRef = regexp.MustCompile(`\[Image #([0-9]+)\]`)
	imageExts  = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

	errNoImage  = errors.New("クリップボードに画像がありません")
	errTooLarge = fmt.Errorf("画像が大きすぎるので貼りませんでした (%d MiB まで)", imageMax>>20)
)

func (s *Store) imagesDir() string { return filepath.Join(s.dir, "images") }

// imagesIn is the image numbers in a text, in order, each once.
func imagesIn(text string) []int {
	var out []int
	seen := map[int]bool{}
	for _, m := range reImageRef.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// imagePath is the absolute path of image n, or "" if there is none.
func (s *Store) imagePath(n int) string {
	for _, ext := range imageExts {
		p := filepath.Join(s.imagesDir(), strconv.Itoa(n)+ext)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

// saveImage stores data as the next image (with the extension ext) and
// returns its number.
func (s *Store) saveImage(data []byte, ext string) (int, error) {
	if len(data) > imageMax {
		return 0, errTooLarge
	}
	if len(data) == 0 {
		return 0, errNoImage
	}
	dir := s.imagesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	os.Chmod(dir, 0o700)
	n := s.lastImage()
	for range 100 {
		n++
		if s.imagePath(n) != "" {
			continue // taken (under another extension)
		}
		f, err := os.OpenFile(filepath.Join(dir, strconv.Itoa(n)+ext), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		_, err = f.Write(data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(f.Name())
			return 0, err
		}
		writeFile(filepath.Join(dir, imageCounter), []byte(strconv.Itoa(n)+"\n"))
		return n, nil
	}
	return 0, errors.New("画像の番号を決められませんでした")
}

// lastImage is the highest number given so far: the counter, or the files
// (whichever is higher), so a number is not reused even if files were
// removed.
func (s *Store) lastImage() int {
	last := 0
	if b, err := os.ReadFile(filepath.Join(s.imagesDir(), imageCounter)); err == nil {
		last, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	ents, _ := os.ReadDir(s.imagesDir())
	for _, e := range ents {
		name := e.Name()
		if n, err := strconv.Atoi(strings.TrimSuffix(name, filepath.Ext(name))); err == nil && n > last {
			last = n
		}
	}
	return last
}

// imageFileOf reads pasted text as the path of one image file (what
// dropping a file on a terminal pastes): quoted, or with spaces escaped by
// \, or file://. "" if it is not one.
func imageFileOf(pasted string) string {
	p := strings.TrimSpace(pasted)
	if p == "" || strings.ContainsAny(p, "\n\r") {
		return ""
	}
	if len(p) >= 2 && (p[0] == '\'' || p[0] == '"') && p[len(p)-1] == p[0] {
		p = p[1 : len(p)-1]
	} else {
		p = strings.NewReplacer(`\ `, " ", `\(`, "(", `\)`, ")", `\'`, "'", `\&`, "&").Replace(p)
	}
	p = strings.TrimPrefix(p, "file://")
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(p))
	if !contains(imageExts, ext) {
		return ""
	}
	if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return p
}

// copyImage stores the image file at path as the next image.
func (s *Store) copyImage(path string) (int, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if fi.Size() > imageMax {
		return 0, errTooLarge
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, imageMax+1))
	if err != nil {
		return 0, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	return s.saveImage(data, ext)
}

// clipboardImage gets the image on the clipboard as PNG (tests replace
// it). errNoImage when there is none, or no way to get it.
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
			} else if errors.Is(err, errTooLarge) {
				return nil, err
			}
		}
		return nil, errNoImage
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
			return nil, errNoImage
		}
		if err := exec.CommandContext(ctx, "sips", "-s", "format", "png", tiff, "--out", png).Run(); err != nil {
			return nil, errNoImage
		}
	}
	fi, err := os.Stat(png)
	if err != nil || fi.Size() == 0 {
		return nil, errNoImage
	}
	if fi.Size() > imageMax {
		return nil, errTooLarge
	}
	return os.ReadFile(png)
}

// limitedOutput runs c and returns its output, up to imageMax.
func limitedOutput(ctx context.Context, c *exec.Cmd) ([]byte, error) {
	var out bytes.Buffer
	c.Stdout = &limitWriter{w: &out, n: imageMax + 1}
	c.Stderr = io.Discard
	c.WaitDelay = 500 * time.Millisecond
	err := c.Run()
	if out.Len() > imageMax {
		return nil, errTooLarge
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
		err = errNoImage
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("クリップボードの画像を %v で取り出せませんでした", clipTimeout)
		}
		a.msg = err.Error()
		return
	}
	n, err := a.st.saveImage(data, ".png")
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
	p := imageFileOf(string(e.text[start:e.pos]))
	if p == "" {
		return
	}
	n, err := a.st.copyImage(p)
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
		m := reImageRef.FindStringIndex(s)
		if m == nil {
			return append(t, plainText(s, base)...)
		}
		t = append(t, plainText(s[:m[0]], base)...)
		t = append(t, plainText(s[m[0]:m[1]], base+dim)...)
		s = s[m[1]:]
	}
}
