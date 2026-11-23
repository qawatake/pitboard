package board

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Images in thoughts: ctrl+v in the editor (or pasting the path of an image
// file) saves the image as .pitboard/<session>/images/<N>.<ext> and puts
// "[Image #N]" in the text. N counts up per session and is never reused.
// 'changes' gives the AI the path of each image a thought still holds.

const (
	ImageMax     = 20 << 20 // 20 MiB
	imageCounter = "last"   // in images/: the last number given
)

var (
	ReImageRef = regexp.MustCompile(`\[Image #([0-9]+)\]`)
	imageExts  = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

	ErrNoImage  = errors.New("クリップボードに画像がありません")
	ErrTooLarge = fmt.Errorf("画像が大きすぎるので貼りませんでした (%d MiB まで)", ImageMax>>20)
)

func (s *Store) imagesDir() string { return filepath.Join(s.Dir, "images") }

// imagesIn is the image numbers in a text, in order, each once.
func imagesIn(text string) []int {
	var out []int
	seen := map[int]bool{}
	for _, m := range ReImageRef.FindAllStringSubmatch(text, -1) {
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

// SaveImage stores data as the next image (with the extension ext) and
// returns its number.
func (s *Store) SaveImage(data []byte, ext string) (int, error) {
	if len(data) > ImageMax {
		return 0, ErrTooLarge
	}
	if len(data) == 0 {
		return 0, ErrNoImage
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

// CopyImage stores the image file at path as the next image.
func (s *Store) CopyImage(path string) (int, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if fi.Size() > ImageMax {
		return 0, ErrTooLarge
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, ImageMax+1))
	if err != nil {
		return 0, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	return s.SaveImage(data, ext)
}

// ImageFileOf reads pasted text as the path of one image file (what
// dropping a file on a terminal pastes): quoted, or with spaces escaped by
// \, or file://. "" if it is not one.
func ImageFileOf(pasted string) string {
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
	if !slices.Contains(imageExts, ext) {
		return ""
	}
	if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return p
}
