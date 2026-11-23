package web

import (
	"regexp"
	"slices"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// The browser's colors are the palette's: app.css defines a token for each
// name (light and dark), and no other color token but the neutral ones.
func TestPaletteCSS(t *testing.T) {
	b, err := staticFS.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	neutral := []string{"bg", "fg", "line", "shadow", "scrim", "on-scrim", "veil"}
	var names []string
	for _, c := range page.Palette {
		names = append(names, c.Name)
	}
	re := regexp.MustCompile(`--([a-z-]+): (#[0-9a-f]{3,8}|rgb\([^)]*\))`)
	count := map[string]int{}
	for _, m := range re.FindAllSubmatch(b, -1) {
		n := string(m[1])
		if !slices.Contains(names, n) && !slices.Contains(neutral, n) {
			t.Errorf("app.css defines the color %q, which is no palette name", n)
		}
		count[n]++
	}
	for _, n := range names {
		if count[n] < 3 { // light, dark by the system, dark by choice
			t.Errorf("app.css defines %q %d times, want light and both darks", n, count[n])
		}
	}
	// Colors are used through the tokens only.
	if m := regexp.MustCompile(`(?m)^[^-\n]*color: #`).Find(b); m != nil {
		t.Errorf("a color outside the tokens: %s", m)
	}
}
