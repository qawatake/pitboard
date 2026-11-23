package viewer

import (
	"strings"
	"testing"
)

// The scroll keys, in the forms terminals send them.
func TestScrollKeyParse(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b[1;3A": "alt+up", "\x1b[1;3B": "alt+down", "\x1b[1;9B": "alt+down",
		"\x1b\x1b[A": "alt+up", "\x1b\x1b[B": "alt+down",
		"\x1b[5~": "pgup", "\x1b[6~": "pgdn", "\x19": "ctrl+y", "\x05": "ctrl+e",
		"\x04": "ctrl+d", "\x15": "ctrl+u", "\x1b[A": "up", "\x1b[B": "down",
	} {
		pasting := false
		k, n := parseKey([]byte(in), &pasting)
		if k.name != want || n != len(in) {
			t.Errorf("%q: %q (%d), want %q", in, k.name, n, want)
		}
		if scrollKey(k, 20) == 0 {
			t.Errorf("%q is no scroll key", want)
		}
	}
	// ctrl+n / ctrl+p stay the cursor's keys (↑ ↓ scroll).
	if k := imeFree(key{name: "ctrl+n"}); k.r != 'j' {
		t.Errorf("ctrl+n: %+v", k)
	}
}

// Alternate scroll (the wheel as ↑ ↓, no mouse capture) is asked for on
// entering, and the mode as it was is put back on leaving.
func TestAlternateScroll(t *testing.T) {
	if !strings.Contains(scrollOn, "\x1b[?1007h") || !strings.Contains(scrollOn, "\x1b[?1007s") || scrollOff != "\x1b[?1007r" {
		t.Errorf("scrollOn %q scrollOff %q", scrollOn, scrollOff)
	}
	for _, s := range []string{scrollOn, keysOn} {
		if strings.Contains(s, "?1000") || strings.Contains(s, "?1006") || strings.Contains(s, "?1002") {
			t.Errorf("the mouse is captured: %q", s)
		}
	}
}
