package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/web"
)

// cookieGet asks the server at port for path with the cookie tok only (a
// tab that outlived a restart).
func cookieGet(t *testing.T, port int, path, tok string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	req.AddCookie(&http.Cookie{Name: "pitboard_" + strconv.Itoa(port), Value: tok})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// WEB8: the token is made once (0600) and kept: a restart (stop, idling
// out) keeps the URL and the old cookie; --rotate makes a new one,
// restarts the running server, and the old cookie no longer works.
func TestServeTokenKept(t *testing.T) {
	_, sd := serveBoard(t)
	url := serveCmd(t, "--no-open")
	s, _ := runningServe(sd)
	fi, err := os.Lstat(tokenPath(sd))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v %v", fi, err)
	}
	if tok, ok := readToken(sd); !ok || tok != s.Token {
		t.Fatalf("token file %q, server %q", tok, s.Token)
	}
	serveCmd(t, "--stop")
	if _, ok := readToken(sd); !ok {
		t.Fatal("--stop deleted the token")
	}
	// Started again on the same port: the same URL, and the old cookie works.
	again := serveCmd(t, "--no-open", "--port", strconv.Itoa(s.Port))
	if again != url {
		t.Errorf("after restart %q, want %q", again, url)
	}
	if c := cookieGet(t, s.Port, "/s/srv1/api/page", s.Token); c != 200 {
		t.Errorf("old cookie after restart: %d", c)
	}
	// Rotate: started again with a new key; the old cookie is refused.
	out := serveCmd(t, "--rotate", "--no-open", "--port", strconv.Itoa(s.Port))
	if !strings.Contains(out, "new key") {
		t.Errorf("rotate said %q", out)
	}
	s2, ok := runningServe(sd)
	if !ok || s2.PID == s.PID || s2.Token == s.Token || s2.Port != s.Port {
		t.Fatalf("after rotate %+v (was pid %d port %d)", s2, s.PID, s.Port)
	}
	if tok, _ := readToken(sd); tok != s2.Token {
		t.Errorf("token file not rotated")
	}
	if c := cookieGet(t, s.Port, "/s/srv1/api/page", s.Token); c != 403 {
		t.Errorf("old cookie after rotate: %d", c)
	}
	if c := cookieGet(t, s2.Port, "/s/srv1/api/page", s2.Token); c != 200 {
		t.Errorf("new cookie: %d", c)
	}
	if !strings.Contains(out, s2.boardURL("srv1")) {
		t.Errorf("rotate did not print the new URL: %q", out)
	}
	// Rotate with no server: only the key changes.
	serveCmd(t, "--stop")
	if out := serveCmd(t, "--rotate"); !strings.Contains(out, "no serve running") {
		t.Errorf("rotate, none running: %q", out)
	}
	if tok, _ := readToken(sd); tok == s2.Token {
		t.Error("not rotated with no server")
	}
	if _, ok := runningServe(sd); ok {
		t.Error("rotate started a server")
	}
}

// A token file that is not good (loose, a link, not hex) is not used.
func TestServeTokenFile(t *testing.T) {
	d := t.TempDir()
	tok, err := serveToken(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := serveToken(d, false); again != tok {
		t.Error("not kept")
	}
	os.Chmod(tokenPath(d), 0o644)
	if _, ok := readToken(d); ok {
		t.Error("a 0644 token taken")
	}
	os.Chmod(tokenPath(d), 0o600)
	os.WriteFile(tokenPath(d), []byte("nothex\n"), 0o600)
	if _, ok := readToken(d); ok {
		t.Error("a bad token taken")
	}
	if fresh, _ := serveToken(d, false); fresh == tok || len(fresh) != 64 {
		t.Errorf("not remade: %q", fresh)
	}
	os.Remove(tokenPath(d))
	other := t.TempDir() + "/tok"
	os.WriteFile(other, []byte(tok+"\n"), 0o600)
	os.Symlink(other, tokenPath(d))
	if _, ok := readToken(d); ok {
		t.Error("a link taken")
	}
}

// serve opens the browser unless a tab of the board is (or comes back
// within web.OpenWait of a restart); --open always, --no-open never.
func TestServeOpensOnce(t *testing.T) {
	_, sd := serveBoard(t)
	var mu sync.Mutex
	var opened []string
	openBrowser = func(u string) { mu.Lock(); opened = append(opened, u); mu.Unlock() }
	t.Cleanup(func() { openBrowser = web.OpenBrowser })
	n := func() int { mu.Lock(); defer mu.Unlock(); return len(opened) }
	serveCmd(t)
	if n() != 1 {
		t.Fatalf("first serve opened %d", n())
	}
	s, _ := runningServe(sd)
	// A tab: its event stream.
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/s/srv1/api/events", s.Port), nil)
	req.AddCookie(&http.Cookie{Name: "pitboard_" + strconv.Itoa(s.Port), Value: s.Token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	serveCmd(t)
	if n() != 1 {
		t.Errorf("opened again with a tab open: %d", n())
	}
	serveCmd(t, "--open")
	if n() != 2 {
		t.Errorf("--open did not open: %d", n())
	}
	resp.Body.Close()
	// Restarted, the tab comes back within the wait: no second tab.
	serveCmd(t, "--stop")
	go func() {
		for range 40 {
			time.Sleep(100 * time.Millisecond)
			if s2, ok := runningServe(sd); ok {
				req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/s/srv1/api/events", s2.Port), nil)
				req.AddCookie(&http.Cookie{Name: "pitboard_" + strconv.Itoa(s2.Port), Value: s2.Token})
				if r, err := http.DefaultClient.Do(req); err == nil {
					t.Cleanup(func() { r.Body.Close() })
					return
				}
			}
		}
	}()
	serveCmd(t, "--port", strconv.Itoa(s.Port))
	if n() != 2 {
		t.Errorf("restart with the tab coming back opened another: %d", n())
	}
	serveCmd(t, "--no-open")
	if n() != 2 {
		t.Errorf("--no-open opened")
	}
}
