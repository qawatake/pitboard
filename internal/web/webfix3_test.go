package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/page"
)

// A plain server never lets a page be put in a state (?ui=): only one
// started for the gallery's check says Gallery; and every page carries the
// palette for the keys list.
func TestGalleryOnlyWhenAsked(t *testing.T) {
	f := setup(t)
	r := f.req(http.MethodGet, "/api/page", nil, nil)
	var p map[string]any
	b, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if _, ok := p["Gallery"]; ok {
		t.Errorf("a plain server's page says Gallery: %s", b)
	}
	pal, _ := p["Palette"].([]any)
	if len(pal) != len(page.Palette) {
		t.Errorf("palette %v", p["Palette"])
	}
	f.s.gallery = true
	for _, h := range f.s.boards {
		h.gallery = true
	}
	r = f.req(http.MethodGet, "/api/page", nil, nil)
	b, _ = io.ReadAll(r.Body)
	if !strings.Contains(string(b), `"Gallery":true`) {
		t.Errorf("a gallery server's page does not say Gallery")
	}
}
