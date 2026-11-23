package web

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// newBoard makes dir/.pitboard/session with a report.
func newBoard(t *testing.T, dir, session, src string) *board.Store {
	t.Helper()
	st := board.Open(dir, session)
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return st
}

func (f *fixture) register(dir, session string, mod func(*http.Request)) (*http.Response, Registration) {
	f.t.Helper()
	resp := f.req("POST", "/api/register", BoardRef{Dir: dir, Session: session}, func(r *http.Request) {
		r.Header.Del("Cookie")
		r.Header.Del("Origin")
		r.Header.Set("Authorization", "Bearer "+f.s.Token())
		if mod != nil {
			mod(r)
		}
	})
	var reg Registration
	json.NewDecoder(resp.Body).Decode(&reg)
	return resp, reg
}

// One server shows many boards, each under its own path; ids are the
// shortest prefix of the session no other board shares, and do not
// change once given.
func TestBoardsByPath(t *testing.T) {
	f := setup(t)
	dir2, _ := filepath.EvalSymlinks(t.TempDir())
	newBoard(t, dir2, "abcdefgh1111", "<Title>二つめ</Title>\n<Item id=\"x\" ask=\"y\">x</Item>\n")
	newBoard(t, dir2, "abcdefgh2222", "<Title>三つめ</Title>\n")
	resp, reg := f.register(dir2, "abcdefgh1111", nil)
	if resp.StatusCode != 200 || reg.ID != "abcdefgh" {
		t.Fatalf("register: %d %+v", resp.StatusCode, reg)
	}
	if _, reg := f.register(dir2, "abcdefgh2222", nil); reg.ID != "abcdefgh2" {
		t.Errorf("second id %+v", reg)
	}
	if _, reg := f.register(dir2, "abcdefgh1111", nil); reg.ID != "abcdefgh" {
		t.Errorf("again: %+v", reg)
	}
	// Each board's page is its own.
	get := func(p string) (int, string) {
		resp := f.req("GET", p, nil, nil)
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if c, b := get("/s/abcdefgh/api/page"); c != 200 || !strings.Contains(b, "二つめ") || strings.Contains(b, "ページの単位") {
		t.Errorf("board 2: %d", c)
	}
	if c, b := get(f.base + "/api/page"); c != 200 || !strings.Contains(b, "ページの単位") {
		t.Errorf("board 1: %d", c)
	}
	if c, b := get("/s/abcdefgh/"); c != 200 || !strings.Contains(b, "/main.js") {
		t.Errorf("board page: %d", c)
	}
	if resp := f.req("GET", "/s/abcdefgh", nil, nil); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/s/abcdefgh/" {
		t.Errorf("no slash: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	// The index lists them all, with their titles and counts.
	c, b := get("/api/boards")
	var idx Index
	json.Unmarshal([]byte(b), &idx)
	if c != 200 || len(idx.Boards) != 3 || idx.Boards[1].Title != "二つめ" || idx.Boards[1].Dir != dir2 || !strings.Contains(b, "反応待ち 1") {
		t.Errorf("index %d %s", c, b)
	}
	if c, b := get("/"); c != 200 || !strings.Contains(b, "/home.js") {
		t.Errorf("index page %d", c)
	}
	// Undo is per board and tab: an act on one board is not undone from
	// another with the same tab id.
	if r := f.act("tab1", Act{Action: "comment", Text: "x"}); !r.OK {
		t.Fatal(r)
	}
	resp = f.req("POST", "/s/abcdefgh/api/act", Act{Action: "undo"}, nil)
	var r Result
	json.NewDecoder(resp.Body).Decode(&r)
	if r.OK {
		t.Errorf("undone from another board: %+v", r)
	}
	// Forget: its path is gone, the others stay.
	resp = f.req("POST", "/api/forget", BoardRef{Dir: dir2, Session: "abcdefgh2222"}, func(r *http.Request) {
		r.Header.Del("Cookie")
		r.Header.Del("Origin")
		r.Header.Set("Authorization", "Bearer "+f.s.Token())
	})
	json.NewDecoder(resp.Body).Decode(&reg)
	if reg.ID != "abcdefgh2" {
		t.Errorf("forget %+v", reg)
	}
	if c, _ := get("/s/abcdefgh2/api/page"); c != 404 {
		t.Errorf("forgotten board: %d", c)
	}
	if c, _ := get("/s/abcdefgh/api/page"); c != 200 {
		t.Errorf("kept board: %d", c)
	}
}

// A board id in the path is not a secret and grants nothing: no cookie, no
// board; a made-up or traversing id is no board.
func TestBoardPathSecurity(t *testing.T) {
	f := setup(t)
	noCookie := func(r *http.Request) { r.Header.Del("Cookie") }
	for _, p := range []string{f.base + "/", f.base + "/api/page", f.base + "/api/events", f.base + "/api/image/1", "/api/boards", "/api/events"} {
		if resp := f.req("GET", p, nil, noCookie); resp.StatusCode != 403 {
			t.Errorf("%s without cookie: %d", p, resp.StatusCode)
		}
	}
	for _, p := range []string{"/s/nope/api/page", "/s/../api/page", "/s/..%2f..%2fetc/api/page", "/s/" + "%2e%2e" + "/api/page", "/s//api/page", "/s/" + strings.TrimPrefix(f.base, "/s/") + "x/api/page", "/s/sess/api/page"} {
		resp := f.req("GET", p, nil, nil)
		if resp.StatusCode == 200 {
			t.Errorf("%s: %d", p, resp.StatusCode)
		}
		checkHeaders(t, resp)
	}
	// Login goes to the board in the path, or else the index: never
	// elsewhere.
	for path, want := range map[string]string{f.base + "/": f.base + "/", "/": "/", "/s/nope/": "/", "//evil.example/": "/", "/s/..%2f/": "/"} {
		resp := f.req("GET", path+"?t="+f.s.Token(), nil, noCookie)
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != want {
			t.Errorf("login at %s: %d %q", path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	// Registering needs the token in the header: the cookie (a page) is
	// not enough, and no Origin may come with it but the server's own.
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	newBoard(t, dir, "other123", "<Title>t</Title>\n")
	if resp := f.req("POST", "/api/register", BoardRef{Dir: dir, Session: "other123"}, nil); resp.StatusCode == 200 {
		t.Errorf("register with the cookie: %d", resp.StatusCode)
	}
	if resp, _ := f.register(dir, "other123", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+strings.Repeat("0", 64)) }); resp.StatusCode != 403 {
		t.Errorf("register with a wrong token: %d", resp.StatusCode)
	}
	if resp, _ := f.register(dir, "other123", func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }); resp.StatusCode != 403 {
		t.Errorf("register from another origin: %d", resp.StatusCode)
	}
	// What may not be registered.
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(dir, link)
	linked, _ := filepath.EvalSymlinks(t.TempDir())
	os.MkdirAll(filepath.Join(linked, ".pitboard"), 0o755)
	os.Symlink(filepath.Join(dir, ".pitboard", "other123"), filepath.Join(linked, ".pitboard", "lnk12345"))
	pbLink, _ := filepath.EvalSymlinks(t.TempDir())
	os.Symlink(filepath.Join(dir, ".pitboard"), filepath.Join(pbLink, ".pitboard"))
	file := filepath.Join(dir, ".pitboard", "afile123")
	os.WriteFile(file, nil, 0o644)
	for _, c := range []BoardRef{
		{"/", "other123"}, {"/", ".."}, {dir, ".."}, {dir, "../other123"}, {dir, ""}, {"relative", "other123"},
		{dir, "nosuch12"}, {dir, "afile123"}, {linked, "lnk12345"}, {pbLink, "other123"}, {"/nonexistent/x", "other123"},
	} {
		if resp, reg := f.register(c.Dir, c.Session, nil); resp.StatusCode == 200 || reg.ID != "" {
			t.Errorf("registered %+v: %d %+v", c, resp.StatusCode, reg)
		}
	}
	// A link to the project directory itself is fine: its real path is
	// what is kept.
	if _, reg := f.register(link, "other123", nil); reg.ID != "other123" {
		t.Errorf("via a linked directory: %+v", reg)
	} else if b := f.s.board("other123"); b.st.Base != dir {
		t.Errorf("kept %s, want %s", b.st.Base, dir)
	}
}

// An image that is a link (or in an images/ that is a link) is not served,
// even when what it points at is an image.
func TestImageLinksNotServed(t *testing.T) {
	f := setup(t)
	outside := filepath.Join(t.TempDir(), "x.png")
	os.WriteFile(outside, pngBytes(t), 0o644)
	os.MkdirAll(filepath.Join(f.st.Dir, "images"), 0o700)
	os.Symlink(outside, filepath.Join(f.st.Dir, "images", "1.png"))
	if resp := f.req("GET", "/api/image/1", nil, nil); resp.StatusCode != 404 {
		t.Errorf("linked image: %d", resp.StatusCode)
	}
	os.RemoveAll(filepath.Join(f.st.Dir, "images"))
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "1.png"), pngBytes(t), 0o644)
	os.Symlink(other, filepath.Join(f.st.Dir, "images"))
	if resp := f.req("GET", "/api/image/1", nil, nil); resp.StatusCode != 404 {
		t.Errorf("linked images/: %d", resp.StatusCode)
	}
}

// The registry keeps the boards (and their ids) for the next server; a
// board whose directory went is dropped without a word.
func TestRegistryRestored(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "boards.json")
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	a := newBoard(t, dir, "keep5678abc", "<Title>a</Title>\n")
	b := newBoard(t, dir, "gone5678abc", "<Title>b</Title>\n")
	s, err := New(Options{Registry: reg, Boards: []*board.Store{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(reg); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("registry %v %v", fi, err)
	}
	ida := s.idOf(a)
	os.RemoveAll(b.Dir)
	s2, err := New(Options{Registry: reg})
	if err != nil {
		t.Fatal(err)
	}
	if s2.board(ida) == nil || len(s2.order) != 1 {
		t.Errorf("restored %d boards", len(s2.order))
	}
	if rs := ReadRegistry(reg); len(rs) != 1 || rs[0].ID != ida || rs[0].Added.After(time.Now()) {
		t.Errorf("registry %+v", rs)
	}
}

// A board whose directory went while the server runs is dropped: its
// event streams are told "gone", its calls answer MsgGone (404), the
// registry forgets it.
func TestBoardGone(t *testing.T) {
	old := PollInterval
	PollInterval = 20 * time.Millisecond
	t.Cleanup(func() { PollInterval = old })
	f := setup(t)
	go f.s.watchLoop()
	resp := f.req("GET", f.base+"/api/events", nil, nil)
	defer resp.Body.Close()
	if err := os.RemoveAll(f.st.Dir); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	go func() {
		b := make([]byte, 4096)
		var all string
		for {
			n, err := resp.Body.Read(b)
			all += string(b[:n])
			if strings.Contains(all, "event: gone") || err != nil {
				got <- all
				return
			}
		}
	}()
	select {
	case s := <-got:
		if !strings.Contains(s, "event: gone\ndata: "+MsgGone) {
			t.Errorf("stream: %q", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no gone event")
	}
	r := f.req("GET", f.base+"/api/page", nil, nil)
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusNotFound || !strings.Contains(string(b), MsgGone) {
		t.Errorf("page: %d %q", r.StatusCode, b)
	}
}

// The registry is written 0600 even when a stale .tmp of another mode is
// left.
func TestWritePrivateMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "boards.json")
	os.WriteFile(p+".tmp", []byte("old"), 0o644)
	if err := WriteRegistry(p, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v %v", fi.Mode().Perm(), err)
	}
}
