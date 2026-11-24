package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
)

// WEB8: the cookie outlives the browser session (Max-Age, renewed when a
// page or its event stream opens), still HttpOnly and SameSite=Strict.
func TestCookieKeptAndRenewed(t *testing.T) {
	f := setup(t)
	noCookie := func(r *http.Request) { r.Header.Del("Cookie") }
	resp := f.req("GET", f.base+"/?t="+f.s.Token(), nil, noCookie)
	sc := resp.Header.Get("Set-Cookie")
	for _, w := range []string{"Max-Age=2592000", "HttpOnly", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(sc, w) {
			t.Errorf("login Set-Cookie %q lacks %q", sc, w)
		}
	}
	// A page opened with the cookie alone (a bookmark) renews it.
	if resp := f.req("GET", f.base+"/", nil, nil); resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Set-Cookie"), "Max-Age=2592000") {
		t.Errorf("page: %d %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
	if resp := f.req("GET", "/", nil, nil); !strings.Contains(resp.Header.Get("Set-Cookie"), "Max-Age=") {
		t.Errorf("index did not renew: %q", resp.Header.Get("Set-Cookie"))
	}
	// Not every call: the page's data and the scripts do not.
	for _, p := range []string{"/api/page", "/main.js"} {
		if resp := f.req("GET", p, nil, nil); resp.Header.Get("Set-Cookie") != "" {
			t.Errorf("%s renews: %q", p, resp.Header.Get("Set-Cookie"))
		}
	}
	// Without the cookie nothing is renewed (nor given).
	if resp := f.req("GET", f.base+"/", nil, noCookie); resp.StatusCode != 403 || resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("no cookie: %d %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
}

// The token is the caller's when given (kept across restarts): a server
// started again with it takes the cookie of the one before; another
// token (rotated) does not.
func TestGivenToken(t *testing.T) {
	tok, _ := NewToken()
	if !ValidToken(tok) || ValidToken("abc") || ValidToken(strings.Repeat("z", 64)) {
		t.Fatal("ValidToken")
	}
	if _, err := New(Options{Token: "short"}); err == nil {
		t.Error("a bad token taken")
	}
	a, err := New(Options{Token: tok})
	if err != nil || a.Token() != tok {
		t.Fatalf("given token: %v %q", err, a.Token())
	}
	f := setup(t)
	old := f.s.Token()
	again, _ := New(Options{Token: old, Boards: f.s.boardsOf()})
	again.SetPort(portOfURL(f.ts.URL))
	f.s = again
	f.ts.Config.Handler = again
	if resp := f.req("GET", "/api/page", nil, nil); resp.StatusCode != 200 {
		t.Errorf("same token, old cookie: %d", resp.StatusCode)
	}
	rot, _ := NewToken()
	rotated, _ := New(Options{Token: rot, Boards: again.boardsOf()})
	rotated.SetPort(portOfURL(f.ts.URL))
	f.ts.Config.Handler = rotated
	if resp := f.req("GET", "/api/page", nil, nil); resp.StatusCode != 403 {
		t.Errorf("rotated token, old cookie: %d", resp.StatusCode)
	}
}

// /api/watching says whether a page of a board has its stream open; the
// command line's call only.
func TestWatching(t *testing.T) {
	f := setup(t)
	id := strings.TrimPrefix(f.base, "/s/")
	boardless["/api/watching?id="+id] = true
	bearer := func(r *http.Request) { r.Header.Del("Cookie"); r.Header.Set("Authorization", "Bearer "+f.s.Token()) }
	watching := func() bool {
		resp := f.req("GET", "/api/watching?id="+id, nil, bearer)
		var w Watching
		if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
			t.Fatalf("watching: %d %v", resp.StatusCode, err)
		}
		return w.Watching
	}
	if watching() {
		t.Error("watching with no page")
	}
	if resp := f.req("GET", "/api/watching?id="+id, nil, nil); resp.StatusCode == 200 {
		t.Error("watching answered a cookie")
	}
	ev := f.req("GET", "/api/events", nil, nil)
	defer ev.Body.Close()
	if !f.s.awaitWatcher(id, time.Now().Add(2*time.Second)) || !watching() {
		t.Error("not watching with the stream open")
	}
	if f.s.Watching("") {
		t.Error("the index watched")
	}
}

func (s *Server) boardsOf() []*board.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*board.Store
	for _, h := range s.order {
		out = append(out, h.st)
	}
	return out
}

func portOfURL(u string) int {
	_, p, _ := strings.Cut(strings.TrimPrefix(u, "http://"), ":")
	n, _ := strconv.Atoi(p)
	return n
}
