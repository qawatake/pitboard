package viewer

import (
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// Every color of the palette has a color on the terminal.
func TestPaletteMapped(t *testing.T) {
	for _, c := range page.Palette {
		if palette[c.Name] == "" {
			t.Errorf("no terminal color for %q", c.Name)
		}
	}
	if len(palette) != len(page.Palette) {
		t.Errorf("the terminal maps %d colors, the palette has %d", len(palette), len(page.Palette))
	}
}
