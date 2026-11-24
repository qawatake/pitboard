package web

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

const src = `<Notice id="note" at="2026-11-23 21:46">開き直してください</Notice>
<Title>題</Title>

<Section title="あなたへ">
  <Ask id="unit">ページの単位
    <Option key="a">セッションごと</Option>
    <Option key="b">プロジェクトごと</Option>
  </Ask>
  <Item id="hook">hook を入れるか決める
    中身
  </Item>
  <Item id="impl" state="doing" look="y">実装する</Item>
  - ただの行
</Section>
`

type fixture struct {
	t    *testing.T
	st   *board.Store
	s    *Server
	ts   *httptest.Server
	host string
	base string // the board's own path: /s/<id>
	// things is the lines of the last page fetched, by key.
	things map[string]*thingJSON
}

func setup(t *testing.T) *fixture { t.Helper(); return setupWith(t, src) }

func setupWith(t *testing.T, src string) *fixture {
	t.Helper()
	t.Setenv("PITBOARD_SEND", "")
	base := t.TempDir()
	st := board.Open(base, "sess1234")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{Boards: []*board.Store{st}})
	if err != nil {
		t.Fatal(err)
	}
	id := s.idOf(st)
	if id == "" {
		t.Fatal("board not registered")
	}
	ts := httptest.NewServer(s)
	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	s.SetPort(port)
	t.Cleanup(func() { s.Close(); ts.Close() })
	return &fixture{t: t, st: st, s: s, ts: ts, host: u.Host, base: "/s/" + id}
}

// req sends a request with the cookie (auth), the right Origin on POST,
// and tab as the browser tab.
func (f *fixture) req(method, path string, body any, mod func(*http.Request)) *http.Response {
	f.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	// The board's own calls are under its path.
	if strings.HasPrefix(path, "/api/") && !boardless[path] {
		path = f.base + path
	}
	r, _ := http.NewRequest(method, f.ts.URL+path, rd)
	r.AddCookie(&http.Cookie{Name: f.s.CookieName(), Value: f.s.Token()})
	r.Header.Set(TabHeader, "tab1")
	if method == http.MethodPost {
		r.Header.Set("Origin", "http://"+f.host)
		r.Header.Set("Content-Type", "application/json")
	}
	if mod != nil {
		mod(r)
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(r)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// boardless is the calls that are not a board's.
var boardless = map[string]bool{"/api/boards": true, "/api/register": true, "/api/forget": true, "/api/events?index": true}

func (f *fixture) page(tab string) PageJSON {
	f.t.Helper()
	resp := f.req("GET", "/api/page", nil, func(r *http.Request) { r.Header.Set(TabHeader, tab) })
	if resp.StatusCode != 200 {
		f.t.Fatalf("page: %d", resp.StatusCode)
	}
	var p struct {
		PageJSON
		Notices, Things []*thingJSON
	}
	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, &p); err != nil {
		f.t.Fatal(err)
	}
	f.things = map[string]*thingJSON{}
	var walk func([]*thingJSON)
	walk = func(ts []*thingJSON) {
		for _, t := range ts {
			f.things[t.Key] = t
			walk(t.Kids)
		}
	}
	walk(p.Notices)
	walk(p.Things)
	return p.PageJSON
}

// thingJSON is a Thing as the browser gets it.
type thingJSON struct {
	Kind, Key, Token string
	Stop             bool
	Kids             []*thingJSON
	Actions          page.Actions
}

func (f *fixture) thing(key string) *thingJSON {
	f.t.Helper()
	t := f.things[key]
	if t == nil {
		f.t.Fatalf("no line %q", key)
	}
	return t
}

func (f *fixture) act(tab string, a Act) Result {
	f.t.Helper()
	resp := f.req("POST", "/api/act", a, func(r *http.Request) { r.Header.Set(TabHeader, tab) })
	var res Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		f.t.Fatalf("act %s: %d %v", a.Action, resp.StatusCode, err)
	}
	return res
}

func (f *fixture) ops() []string {
	var out []string
	for _, e := range f.st.Events() {
		out = append(out, e.Op)
	}
	return out
}

func checkHeaders(t *testing.T, resp *http.Response) {
	t.Helper()
	want := map[string]string{
		"Content-Security-Policy": CSP,
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s %s: %s = %q, want %q", resp.Request.Method, resp.Request.URL.Path, k, got, v)
		}
	}
}

func TestHeadersOnEveryResponse(t *testing.T) {
	f := setup(t)
	noCookie := func(r *http.Request) { r.Header.Del("Cookie") }
	for _, resp := range []*http.Response{
		f.req("GET", "/", nil, nil),
		f.req("GET", "/main.js", nil, nil),
		f.req("GET", "/api/page", nil, nil),
		f.req("GET", "/nope", nil, nil),
		f.req("GET", "/", nil, noCookie),
		f.req("GET", "/", nil, func(r *http.Request) { r.Host = "evil.example:80" }),
		f.req("POST", "/api/act", Act{Action: "undo"}, func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }),
		f.req("GET", "/?t=wrong", nil, noCookie),
	} {
		checkHeaders(t, resp)
	}
	if ct := f.req("GET", "/main.js", nil, nil).Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("main.js: %s", ct)
	}
}

func TestAuth(t *testing.T) {
	f := setup(t)
	noCookie := func(r *http.Request) { r.Header.Del("Cookie") }
	for _, p := range []string{"/", "/main.js", "/api/page", "/api/events", "/api/image/1"} {
		resp := f.req("GET", p, nil, noCookie)
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 403 || !strings.Contains(string(b), "pitboard serve") {
			t.Errorf("%s without cookie: %d %q", p, resp.StatusCode, b)
		}
	}
	bad := f.req("GET", "/api/page", nil, func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: f.s.CookieName(), Value: strings.Repeat("0", 64)})
	})
	if bad.StatusCode != 403 {
		t.Errorf("wrong cookie: %d", bad.StatusCode)
	}
	if resp := f.req("GET", "/?t=wrong", nil, noCookie); resp.StatusCode != 403 {
		t.Errorf("wrong token: %d", resp.StatusCode)
	}
	resp := f.req("GET", "/?t="+f.s.Token(), nil, noCookie)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("login: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	sc := resp.Header.Get("Set-Cookie")
	for _, w := range []string{f.s.CookieName() + "=" + f.s.Token(), "HttpOnly", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(sc, w) {
			t.Errorf("Set-Cookie %q lacks %q", sc, w)
		}
	}
	if len(f.s.Token()) < 32 {
		t.Errorf("token too short: %d hex", len(f.s.Token()))
	}
	if !strings.HasPrefix(f.s.URL(), "http://127.0.0.1:") {
		t.Errorf("URL %s", f.s.URL())
	}
}

func TestHostAndOrigin(t *testing.T) {
	f := setup(t)
	port := strings.Split(f.host, ":")[1]
	for host, ok := range map[string]bool{
		"127.0.0.1:" + port: true, "localhost:" + port: true,
		"evil.example:" + port: false, "127.0.0.1:1": false, "localhost": false, "[::1]:" + port: false,
	} {
		resp := f.req("GET", "/api/page", nil, func(r *http.Request) { r.Host = host })
		if (resp.StatusCode == 200) != ok {
			t.Errorf("Host %s: %d", host, resp.StatusCode)
		}
	}
	for origin, ok := range map[string]bool{
		"http://127.0.0.1:" + port: true, "http://localhost:" + port: true,
		"": false, "null": false, "http://evil.example": false, "http://127.0.0.1:1": false, "https://127.0.0.1:" + port: false,
	} {
		resp := f.req("POST", "/api/act", Act{Action: "undo"}, func(r *http.Request) { r.Header.Set("Origin", origin) })
		if (resp.StatusCode == 200) != ok {
			t.Errorf("Origin %q: %d", origin, resp.StatusCode)
		}
	}
	if resp := f.req("POST", "/api/act", Act{Action: "undo"}, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain POST: %d", resp.StatusCode)
	}
	if resp := f.req("GET", "/api/act", nil, nil); resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/act: %d", resp.StatusCode)
	}
}

func TestPageJSON(t *testing.T) {
	f := setup(t)
	resp := f.req("GET", "/api/page", nil, nil)
	b, _ := io.ReadAll(resp.Body)
	s := string(b)
	for _, w := range []string{`"Kind":"notice"`, `"Kind":"ask"`, `"Kind":"item"`, `"Kind":"option"`, `"Kind":"section"`,
		`"Mark":"waiting"`, `"Mark":"new"`, `"Tone":"move"`, `"Style":"plain"`, `"Token":"`, `"Sign":"?"`, `"Words":{`,
		`"AtText":"`, `"Summary":[{"Text":"あなたの番","Tone":"normal"},{"Text":"反応待ち 1"`, `"反応待ち 1"`, `"未読 1"`, `"JumpTo":["note","unit"]`, `"Session":"sess1234"`, `"Next":[`} {
		if !strings.Contains(s, w) {
			t.Errorf("page lacks %s", w)
		}
	}
	for _, w := range []string{`"Node"`, `"Seen"`, `"Sig"`, `"Jumps"`} {
		if strings.Contains(s, w) {
			t.Errorf("page has %s", w)
		}
	}
}

func TestActRoundTrips(t *testing.T) {
	f := setup(t)
	f.page("tab1")
	hook := f.thing("hook")
	if len(hook.Actions.Options) != 0 || !hook.Actions.Comment || !hook.Actions.Hide {
		t.Fatalf("hook actions %+v", hook.Actions)
	}
	steps := []struct {
		a    Act
		op   string
		want string
	}{
		{Act{Action: "react", Target: "unit", Seen: f.thing("unit").Token, Value: "a", Cur: Cur{Open: true}}, "react", "答え a を付けました: ページの単位 (u で戻す)"},
		{Act{Action: "key", Key: "b", Target: "unit", Seen: f.thing("unit").Token, Cur: Cur{Open: true}}, "react", "答え b を付けました: ページの単位 (u で戻す)"},
		{Act{Action: "comment", Target: "hook", Seen: hook.Token, Text: "なぜ?"}, "thought", page.MsgCommented},
		{Act{Action: "comment", Text: "宛先なし"}, "thought", page.MsgCommented},
		{Act{Action: "edit", N: 3, Text: "なぜ? (直した)"}, "edit", page.MsgEdited},
		{Act{Action: "retract", N: 4}, "retract", "取り消しました (u で戻す)"},
		{Act{Action: "hide", Target: "-1:ただの行", Seen: f.thing("-1:ただの行").Token}, "hide", "隠しました (H で表示、u で戻す)"},
	}
	for _, st := range steps {
		r := f.act("tab1", st.a)
		if !r.OK || r.Message != st.want {
			t.Fatalf("%s: %+v", st.a.Action, r)
		}
		ops := f.ops()
		if ops[len(ops)-1] != st.op {
			t.Fatalf("%s: last op %v", st.a.Action, ops)
		}
	}
	ev := f.st.Events()
	if ev[0].Value != "a" || ev[0].ID != "unit" || ev[0].Option != "セッションごと" || ev[1].Value != "b" || ev[2].ID != "hook" || ev[2].Title != "hook を入れるか決める" || ev[3].ID != "" {
		t.Errorf("events %+v", ev)
	}
	// read: the unread line's green mark goes.
	p := f.page("tab1")
	if !f.thing("impl").Actions.Read {
		t.Fatal("impl cannot be read")
	}
	if r := f.act("tab1", Act{Action: "read", Target: "impl", Seen: f.thing("impl").Token}); !r.OK {
		t.Fatalf("read %+v", r)
	}
	if p = f.page("tab1"); f.thing("impl").Actions.Read {
		t.Error("impl still unread")
	}
	if r := f.act("tab1", Act{Action: "readall"}); !r.OK || r.Message != "いまある行をすべて読んだことにしました" {
		t.Errorf("readall %+v", r)
	}
	// undo: the newest first, in this tab only.
	if r := f.act("tab2", Act{Action: "undo"}); r.OK || r.Message != page.ErrNothingToUndo.Error() {
		t.Errorf("undo in another tab: %+v", r)
	}
	if r := f.act("tab1", Act{Action: "undo"}); !r.OK || !strings.HasPrefix(r.Message, "A (") {
		t.Errorf("undo readall: %+v", r)
	}
	r := f.act("tab1", Act{Action: "undo"})
	if !r.OK || !strings.Contains(r.Message, "r (読んだ)") {
		t.Errorf("undo: %+v", r)
	}
	if r := f.act("tab1", Act{Action: "undo"}); !r.OK || !strings.HasPrefix(r.Message, "隠したのを取り消しました") {
		t.Errorf("undo hide: %+v", r)
	}
	if ops := f.ops(); ops[len(ops)-1] != "undo" {
		t.Errorf("ops %v", ops)
	}
	// send: marks what is unsent as sent ($PITBOARD_SEND is a no-op here).
	t.Setenv("PITBOARD_SEND", "true")
	resp := f.req("POST", "/api/send", struct{}{}, nil)
	var sr Result
	json.NewDecoder(resp.Body).Decode(&sr)
	if !sr.OK || sr.Message != "送りました" {
		t.Errorf("send %+v", sr)
	}
	if p = f.page("tab1"); p.Unsent != 0 {
		t.Errorf("unsent %d after send", p.Unsent)
	}
	// Sent: the AI's turn, first at the top right.
	if len(p.Summary) == 0 || p.Summary[0] != (page.Word{Text: page.WordAITurn, Tone: page.ToneQuiet}) {
		t.Errorf("summary after send %+v", p.Summary)
	}
	if r := f.act("tab1", Act{Action: "nope"}); r.OK {
		t.Error("unknown action done")
	}
}

func TestStaleSeen(t *testing.T) {
	f := setup(t)
	f.page("tab1")
	old := f.thing("hook").Token
	n := len(f.st.Events())
	// The AI rewrites the line after the page was shown.
	os.WriteFile(f.st.ReportPath(), []byte(strings.Replace(src, "hook を入れるか決める", "hook を入れない", 1)), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(f.st.ReportPath(), future, future)
	// The line under the cursor was rewritten: locked, as in the terminal.
	for _, a := range []Act{
		{Action: "hide", Target: "hook", Seen: old},
	} {
		if r := f.act("tab1", a); r.OK || r.Message != page.MsgMoved {
			t.Errorf("%s %s: %+v", a.Action, a.Target, r)
		}
	}
	for _, a := range []Act{
		{Action: "comment", Target: "hook", Seen: "made-up", Text: "x"},
		{Action: "react", Target: "unit", Seen: old, Value: "a"}, // another line's token
		{Action: "read", Target: "impl", Seen: old},
	} {
		r := f.act("tab1", a)
		if r.OK || !r.Changed || r.Message != page.MsgChanged {
			t.Errorf("%s %s: %+v", a.Action, a.Target, r)
		}
	}
	if len(f.st.Events()) != n {
		t.Errorf("recorded on a stale line: %v", f.ops())
	}
	// A comment begun before the rewrite keeps the title it was begun on.
	f2 := setup(t)
	f2.page("tab1")
	tok := f2.thing("hook").Token
	os.WriteFile(f2.st.ReportPath(), []byte(strings.Replace(src, "hook を入れるか決める", "hook を入れない", 1)), 0o644)
	os.Chtimes(f2.st.ReportPath(), future, future)
	if r := f2.act("tab1", Act{Action: "comment", Target: "hook", Seen: tok, Text: "書いた"}); !r.OK {
		t.Fatalf("comment %+v", r)
	}
	if e := f2.st.Events()[0]; e.Title != "hook を入れるか決める" {
		t.Errorf("comment title %q", e.Title)
	}
}

func pngBytes(t *testing.T) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestImages(t *testing.T) {
	f := setup(t)
	up := func(data []byte) Result {
		resp := f.req("POST", "/api/image", ImageUpload{Data: base64.StdEncoding.EncodeToString(data)}, nil)
		var r Result
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatalf("image: %d %v", resp.StatusCode, err)
		}
		return r
	}
	r := up(pngBytes(t))
	if !r.OK || r.Image != 1 {
		t.Fatalf("upload %+v", r)
	}
	if r := up([]byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")); r.OK || r.Message != page.ErrNotImage.Error() {
		t.Errorf("svg %+v", r)
	}
	if r := up([]byte("<html><script>alert(1)</script>")); r.OK {
		t.Errorf("html taken %+v", r)
	}
	big := append(pngBytes(t), make([]byte, board.ImageMax)...)
	if r := up(big); r.OK || r.Message != board.ErrTooLarge.Error() {
		t.Errorf("big %+v", r)
	}
	resp := f.req("GET", "/api/image/1", nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("get 1: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	checkHeaders(t, resp)
	// Something not an image placed among the images is not served.
	os.WriteFile(f.st.Dir+"/images/2.png", []byte("<html>"), 0o600)
	for _, p := range []string{"/api/image/2", "/api/image/3", "/api/image/01", "/api/image/-1", "/api/image/..%2freport.md", "/api/image/1.png"} {
		if resp := f.req("GET", p, nil, nil); resp.StatusCode != 404 {
			t.Errorf("%s: %d", p, resp.StatusCode)
		}
	}
}

func TestEvents(t *testing.T) {
	old := PollInterval
	PollInterval = 20 * time.Millisecond
	defer func() { PollInterval = old }()
	f := setup(t)
	go f.s.watchLoop()
	resp := f.req("GET", "/api/events", nil, nil)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("events: %s", ct)
	}
	checkHeaders(t, resp)
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	time.Sleep(100 * time.Millisecond)
	f.act("tab1", Act{Action: "comment", Text: "x"})
	deadline := time.After(3 * time.Second)
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatal("stream ended")
			}
			if l == "event: changed" {
				f.s.Close()
				return
			}
		case <-deadline:
			t.Fatal("no changed event")
		}
	}
}

func TestStaticHasNoInnerHTML(t *testing.T) {
	var jss []string
	fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".js") {
			jss = append(jss, strings.TrimPrefix(p, "static/"))
		}
		return nil
	})
	if len(jss) < 10 {
		t.Fatalf("only %q", jss)
	}
	for _, js := range jss {
		b, err := staticFS.ReadFile("static/" + js)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function"} {
			if bytes.Contains(b, []byte(w)) {
				t.Errorf("%s uses %s", js, w)
			}
		}
	}
	for _, h := range []string{"index.html", "home.html", "gallery.html"} {
		idx, _ := staticFS.ReadFile("static/" + h)
		if bytes.Contains(idx, []byte("<script>")) || bytes.Contains(idx, []byte("style=")) || bytes.Contains(idx, []byte("http")) {
			t.Errorf("%s has inline script/style or an external URL", h)
		}
	}
}

// A cookie left by another session's earlier server on the same port (the
// default port is shared by sessions in turn) just fails, and opening the
// printed URL replaces it.
func TestStaleCookieOnSamePort(t *testing.T) {
	f := setup(t)
	stale := func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: f.s.CookieName(), Value: strings.Repeat("a", 64)})
	}
	if resp := f.req("GET", "/api/page", nil, stale); resp.StatusCode != 403 {
		t.Errorf("stale cookie: %d", resp.StatusCode)
	}
	resp := f.req("GET", "/?t="+f.s.Token(), nil, stale)
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Set-Cookie"), f.s.CookieName()+"="+f.s.Token()) {
		t.Fatalf("login over a stale cookie: %d %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
	if resp := f.req("GET", "/api/page", nil, nil); resp.StatusCode != 200 {
		t.Errorf("after login: %d", resp.StatusCode)
	}
}

func portOf(ln net.Listener) int { return ln.Addr().(*net.TCPAddr).Port }

// The default is DefaultPort when free; when taken, the next ones; past
// them, any free port. --port is exact.
func TestListenPorts(t *testing.T) {
	if DefaultPort != 7482 {
		t.Errorf("DefaultPort %d", DefaultPort)
	}
	// The default itself, unless something on this machine holds it.
	if ln, err := net.Listen("tcp", "127.0.0.1:7482"); err == nil {
		ln.Close()
		got, err := Listen(PortDefault)
		if err != nil || portOf(got) != DefaultPort {
			t.Errorf("default: %v %v", got, err)
		}
		if got != nil {
			got.Close()
		}
	}
	// A range whose first port is taken: the next free one.
	hold, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()
	first := portOf(hold)
	got, err := listenFrom(first, 3)
	if err != nil {
		t.Fatal(err)
	}
	if p := portOf(got); p == first {
		t.Errorf("took the held port %d", p)
	}
	got.Close()
	// All of the range taken: some other free port.
	got, err = listenFrom(first, 1)
	if err != nil || portOf(got) == first {
		t.Errorf("fallback: %v %v", got, err)
	}
	got.Close()
	// Exact: an error naming the port.
	if ln, err := Listen(first); err == nil {
		ln.Close()
		t.Error("exact port taken: no error")
	} else if !strings.Contains(err.Error(), strconv.Itoa(first)) {
		t.Errorf("exact port error %q", err)
	}
}
