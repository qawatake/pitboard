// Package web is pitboard's second screen: the board in a browser
// (pitboard serve). It is built only on internal/page, so it shows the
// same parts with the same words, marks and rules as the terminal viewer,
// and both may run at once on the same board.
//
// The server listens on 127.0.0.1 only. A random token, given once in the
// URL it prints, becomes an HttpOnly cookie; every other request needs it.
// Requests for another Host are refused (DNS rebinding), and so are state
// changes from another Origin. Every response forbids anything but this
// server's own scripts, styles and images.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

//go:embed static
var staticFS embed.FS

// CSP is the Content-Security-Policy of every response.
const CSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// PollInterval is how often the board is checked for changes (the
// terminal viewer's tick).
var PollInterval = 500 * time.Millisecond

// tabIdle is how long a browser tab's board (its undo history) is kept
// without a request.
const tabIdle = time.Hour

// Options is how 'pitboard serve' runs.
type Options struct {
	Base  string       // where boards are looked for (as the viewer's)
	Store *board.Store // the session; nil: the first one to appear under Base
	Port  int          // 0: a free port chosen by the system
	Open  bool         // open the URL in the default browser
	Out   io.Writer    // where the URL is printed
}

// Server is the board in a browser.
type Server struct {
	token string
	port  int
	base  string

	mu    sync.Mutex
	st    *board.Store
	gen   int // counts up when st changes: tabs of an older one start anew
	tabs  map[string]*tab
	subs  map[chan struct{}]bool
	done  chan struct{}
	watch *page.Board // the board the watcher polls
}

// tab is one browser tab's board: its own undo history (page.Board.Hist),
// and the lines as it was last shown them (by token).
type tab struct {
	mu   sync.Mutex
	b    *page.Board
	gen  int
	used time.Time
	seen map[string]seenLine
	// again is the press last dropped while the cursor's line settled
	// (page.Touch.Again).
	again string
	// say is what to tell the person with the next page (a hidden line
	// came back), once.
	say string
}

// seenLine is a line as a tab was shown it.
type seenLine struct {
	seen   page.Seen
	target string       // what comments on it are recorded against
	title  string       // its title then (report.Node.Title)
	node   *report.Node // the line then (to tell whether it was rewritten)
}

// New makes a server for o (not yet listening).
func New(o Options) (*Server, error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	return &Server{
		token: hex.EncodeToString(tok), port: o.Port, base: o.Base, st: o.Store,
		tabs: map[string]*tab{}, subs: map[chan struct{}]bool{}, done: make(chan struct{}),
	}, nil
}

// Run serves until ctx ends: prints the URL, opens it unless told not to,
// and shuts down cleanly.
func Run(ctx context.Context, o Options) error {
	s, err := New(o)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(o.Port)))
	if err != nil {
		return err
	}
	s.port = ln.Addr().(*net.TCPAddr).Port
	url := s.URL()
	fmt.Fprintln(o.Out, url)
	if o.Open {
		openBrowser(url)
	}
	hs := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go s.watchLoop()
	errc := make(chan error, 1)
	go func() { errc <- hs.Serve(ln) }()
	select {
	case <-ctx.Done():
	case err := <-errc:
		s.Close()
		return err
	}
	s.Close()
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := hs.Shutdown(sctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

// Close ends the watcher and every open event stream.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// URL is the address to open, with the token.
func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", s.port, s.token)
}

// SetPort is for tests that serve without Run.
func (s *Server) SetPort(p int) { s.port = p }

// Token is for tests.
func (s *Server) Token() string { return s.token }

// CookieName is the cookie holding the token. Cookies are per host, not
// per port, so two servers at once each have their own.
func (s *Server) CookieName() string { return "pitboard_" + strconv.Itoa(s.port) }

func openBrowser(url string) {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	c := exec.Command(cmd, url)
	c.Stdout, c.Stderr = nil, nil
	if err := c.Start(); err == nil {
		go c.Wait() // failure is not an error: the URL is printed
	}
}

// ServeHTTP checks every request, then routes it.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", CSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	if !s.hostOK(r.Host) {
		http.Error(w, "pitboard: このアドレスでは開けません。pitboard serve が出した URL を使ってください", http.StatusForbidden)
		return
	}
	if r.URL.Path == "/" && r.URL.Query().Has("t") {
		s.login(w, r)
		return
	}
	if !s.authed(r) {
		http.Error(w, "pitboard: 入れません。pitboard serve が出した URL (?t=… 付き) を開いてください", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost && !s.originOK(r.Header.Get("Origin")) {
		http.Error(w, "pitboard: ほかのページからの操作は受けません", http.StatusForbidden)
		return
	}
	switch {
	case r.URL.Path == "/api/page" && r.Method == http.MethodGet:
		s.handlePage(w, r)
	case r.URL.Path == "/api/events" && r.Method == http.MethodGet:
		s.handleEvents(w, r)
	case r.URL.Path == "/api/act" && r.Method == http.MethodPost:
		s.post(w, r, s.handleAct)
	case r.URL.Path == "/api/send" && r.Method == http.MethodPost:
		s.post(w, r, s.handleSend)
	case r.URL.Path == "/api/image" && r.Method == http.MethodPost:
		s.post(w, r, s.handleImage)
	case strings.HasPrefix(r.URL.Path, "/api/image/") && r.Method == http.MethodGet:
		s.handleImageGet(w, r, r.URL.Path[len("/api/image/"):])
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && !strings.HasPrefix(r.URL.Path, "/api/"):
		s.handleStatic(w, r)
	default:
		http.Error(w, "pitboard: この操作は受けません", http.StatusMethodNotAllowed)
	}
}

func (s *Server) hostOK(host string) bool {
	p := strconv.Itoa(s.port)
	return host == "127.0.0.1:"+p || host == "localhost:"+p
}

func (s *Server) originOK(origin string) bool {
	p := strconv.Itoa(s.port)
	return origin == "http://127.0.0.1:"+p || origin == "http://localhost:"+p
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.token)) != 1 {
		http.Error(w, "pitboard: 入れません。pitboard serve が出した URL をそのまま開いてください (起動し直すと変わります)", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.CookieName(), Value: s.token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(s.CookieName())
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	if name == "/" {
		name = "/index.html"
	}
	sub, _ := fs.Sub(staticFS, "static")
	b, err := fs.ReadFile(sub, name[1:])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := "text/plain; charset=utf-8"
	switch {
	case strings.HasSuffix(name, ".html"):
		ct = "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		ct = "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		ct = "image/svg+xml"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(b)
}

// store is the session served now. Without one, the first session to
// appear under base is taken (as the viewer does); a session given that
// did not exist yet is opened anew once it does.
func (s *Server) store() (*board.Store, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st == nil && s.base != "" {
		if ss := board.Sessions(s.base); len(ss) > 0 {
			s.st = board.Open(s.base, ss[0])
			s.gen++
		}
	}
	return s.st, s.gen
}

// tabOf is the board of the browser tab id, made anew when the session
// changed. Idle tabs are dropped.
func (s *Server) tabOf(id string) *tab {
	st, gen := s.store()
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, t := range s.tabs {
		if now.Sub(t.used) > tabIdle {
			delete(s.tabs, k)
		}
	}
	if len(id) == 0 || len(id) > 64 {
		id = "-"
	}
	t := s.tabs[id]
	if t == nil || t.gen != gen {
		t = &tab{b: page.Open(st), gen: gen, seen: map[string]seenLine{}}
		s.tabs[id] = t
	}
	t.used = now
	return t
}

// fresh reads again what changed on disk; a session that did not exist
// when the tab opened is opened once it does.
func (t *tab) fresh() {
	if t.b.NoSession {
		if _, err := os.Stat(t.b.Store.Dir); err == nil {
			t.b = page.Open(t.b.Store)
			return
		}
	}
	if t.b.Report.Path != "" && t.b.Report.Stale() {
		// As the terminal: a hidden line the AI rewrote comes back, and
		// the person is told (the record as it was, the report as it is).
		hidden := t.b.HiddenKeys()
		t.b.Report.Load()
		if msg := t.b.Reappeared(hidden); msg != "" {
			t.say = msg
		}
	}
	t.b.Changed()
}

func statDir(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}
