package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageNumbersAndDelivery(t *testing.T) {
	st := testStore(t)
	n1, err := st.SaveImage([]byte("a"), ".png")
	n2, _ := st.SaveImage([]byte("b"), ".png")
	if err != nil || n1 != 1 || n2 != 2 {
		t.Fatalf("numbers %d %d %v", n1, n2, err)
	}
	fi, _ := os.Stat(filepath.Join(st.imagesDir(), "1.png"))
	di, _ := os.Stat(st.imagesDir())
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Errorf("modes %v %v", fi.Mode(), di.Mode())
	}
	// Removed files are not reused.
	os.Remove(filepath.Join(st.imagesDir(), "2.png"))
	if n, _ := st.SaveImage([]byte("c"), ".png"); n != 3 {
		t.Errorf("after a removal: %d", n)
	}
	// Only images still in the text, and existing, are told.
	st.Append(Event{Op: "thought", Text: "見て [Image #1] と [Image #2] と [Image #3] [Image #1]"})
	sendAll(st)
	lines, _ := st.Changes()
	abs := func(n string) string { p, _ := filepath.Abs(filepath.Join(st.imagesDir(), n)); return p }
	want := []string{
		"thought 1: 見て [Image #1] と [Image #2] と [Image #3] [Image #1]",
		"  image 1: " + abs("1.png"),
		"  image 3: " + abs("3.png"),
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s", strings.Join(lines, "\n"))
	}
	if _, err := st.SaveImage(make([]byte, ImageMax+1), ".png"); err != ErrTooLarge {
		t.Errorf("too large: %v", err)
	}
}
