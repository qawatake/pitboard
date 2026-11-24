package viewer

import "testing"

func TestPasteKeys(t *testing.T) {
	ks := parseAll("\x1b[200~/a b.png\x1b[201~")
	if len(ks) < 2 || ks[0].name != "paste-start" || ks[len(ks)-1].name != "paste-end" {
		t.Errorf("keys %+v", ks)
	}
}

// ---- keys that work with an input method on ----

func TestKeysWithInputMethod(t *testing.T) {
	st := testStore(t)
	a := testApp(t, st)
	pv := a.top()
	a.render()
	a.handle(key{r: 'ｊ'}) // full-width j
	a.render()
	if pv.cur != "hook" || pv.placed {
		t.Fatalf("ｊ must move like j: cur %q placed %v", pv.cur, pv.placed)
	}
	a.handle(key{r: 'ｃ'})
	if a.in == nil {
		t.Fatal("ｃ must open the comment editor like c")
	}
	a.handle(key{name: "esc"})
	a.in = nil
	a.handle(key{name: "ctrl+p"})
	a.render()
	if pv.cur != "unit" {
		t.Errorf("ctrl+p must move up: %q", pv.cur)
	}
	a.handle(key{name: "ctrl+n"})
	if pv.cur != "hook" {
		t.Errorf("ctrl+n must move down: %q", pv.cur)
	}
	// In the editor, full-width letters stay letters.
	a.handle(key{r: 'ｍ'})
	if a.in == nil {
		t.Fatal("ｍ must open the editor")
	}
	a.handle(key{r: 'ｊ'})
	a.handle(key{r: '　'})
	if got := a.in.ed.String(); got != "ｊ　" {
		t.Errorf("editor text %q", got)
	}
}

// parseAll reads every key in b.
func parseAll(b string) []key {
	var out []key
	pasting := false
	buf := []byte(b)
	for len(buf) > 0 {
		k, n := parseKey(buf, &pasting)
		if n == 0 {
			break
		}
		buf = buf[n:]
		if k != (key{}) {
			out = append(out, k)
		}
	}
	return out
}

func keyName(k key) string {
	if k.name != "" {
		return k.name
	}
	return string(k.r)
}

// Every key the viewer uses reads the same with the terminal's key
// extensions off (legacy bytes), with the kitty protocol on (disambiguate),
// and with modifyOtherKeys.
func TestKeysSameWithExtensions(t *testing.T) {
	for _, c := range []struct {
		want   string
		legacy string
		kitty  string
		mok    string // modifyOtherKeys, where it changes the bytes
	}{
		{"esc", "\x1b", "\x1b[27u", ""},
		{"enter", "\r", "\r", ""},
		{"newline", "\n", "\x1b[106;5u", "\x1b[27;5;106~"},   // ctrl+j
		{"newline", "", "\x1b[13;2u", "\x1b[27;2;13~"},       // shift+enter
		{"newline", "\x1b\r", "\x1b[13;3u", "\x1b[27;3;13~"}, // alt+enter
		{"tab", "\t", "\t", ""},
		{"shift+tab", "\x1b[Z", "\x1b[9;2u", ""},
		{"backspace", "\x7f", "\x7f", ""},
		{"backspace", "\x08", "\x1b[104;5u", ""}, // ctrl+h
		{"alt+backspace", "\x1b\x7f", "\x1b[127;3u", ""},
		{"ctrl+c", "\x03", "\x1b[99;5u", "\x1b[27;5;99~"},
		{"ctrl+a", "\x01", "\x1b[97;5u", ""},
		{"ctrl+e", "\x05", "\x1b[101;5u", ""},
		{"ctrl+g", "\x07", "\x1b[103;5u", ""},
		{"ctrl+k", "\x0b", "\x1b[107;5u", ""},
		{"ctrl+n", "\x0e", "\x1b[110;5u", ""},
		{"ctrl+p", "\x10", "\x1b[112;5u", ""},
		{"ctrl+s", "\x13", "\x1b[115;5u", ""},
		{"ctrl+u", "\x15", "\x1b[117;5u", ""},
		{"ctrl+w", "\x17", "\x1b[119;5u", ""},
		{"alt+b", "\x1bb", "\x1b[98;3u", ""},
		{"alt+f", "\x1bf", "\x1b[102;3u", ""},
		{"up", "\x1b[A", "\x1b[A", ""},
		{"down", "\x1b[B", "\x1b[B", ""},
		{"left", "\x1b[D", "\x1b[D", ""},
		{"right", "\x1b[C", "\x1b[C", ""},
		{"alt+left", "\x1b[1;3D", "\x1b[1;3D", ""},
		{"alt+right", "\x1b[1;3C", "\x1b[1;3C", ""},
		{"pgup", "\x1b[5~", "\x1b[5~", ""},
		{"pgdn", "\x1b[6~", "\x1b[6~", ""},
		{"home", "\x1b[H", "\x1b[H", ""},
		{"end", "\x1b[F", "\x1b[F", ""},
		{"delete", "\x1b[3~", "\x1b[3~", ""},
		{"j", "j", "j", ""},
		{"A", "A", "\x1b[97;2u", ""},
		{" ", " ", " ", ""},
		{"ｊ", "ｊ", "ｊ", ""},
	} {
		for form, b := range map[string]string{"legacy": c.legacy, "kitty": c.kitty, "modifyOtherKeys": c.mok} {
			if b == "" {
				continue
			}
			ks := parseAll(b)
			if len(ks) != 1 || keyName(ks[0]) != c.want {
				var got []string
				for _, k := range ks {
					got = append(got, keyName(k))
				}
				t.Errorf("%s %q: got %q, want %q", form, b, got, c.want)
			}
		}
	}
}

func TestPasteCRLF(t *testing.T) {
	pasting := true
	var got []rune
	b := []byte("three\r\nfour\rfive\nsix")
	for len(b) > 0 {
		k, n := parseKey(b, &pasting)
		if n == 0 {
			break
		}
		b = b[n:]
		got = append(got, k.r)
	}
	if string(got) != "three\nfour\nfive\nsix" {
		t.Errorf("pasted %q", string(got))
	}
}
