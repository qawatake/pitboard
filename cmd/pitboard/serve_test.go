package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/web"
)

// serve refuses what it does not know before listening.
func TestServeArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	for args, want := range map[string]string{
		"serve --bogus":       "unknown argument",
		"serve --port":        "--port needs",
		"serve --port x":      "give a port number",
		"serve --port=70000":  "give a port number",
		"serve --no-open=yes": "takes no value",
	} {
		err := run(append([]string{"-s", "abc"}, strings.Fields(args)...), io.Discard)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", args, err)
		}
	}
}

// TestMain lets the test binary be pitboard itself: the background server
// is this binary started again.
func TestMain(m *testing.M) {
	if os.Getenv("PITBOARD_TEST_AS_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// serveBoard is a board with session "srv1" and a report, as cwd; the
// server's state goes to a state directory of the test's own (never the
// person's), and the server it starts is stopped at the end.
func serveBoard(t *testing.T) (*board.Store, string) {
	t.Helper()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	st := newServeBoard(t, dir, "srv1")
	t.Chdir(dir)
	t.Setenv("PITBOARD_TEST_AS_MAIN", "1")
	t.Setenv("PITBOARD_SESSION", "")
	sd := filepath.Join(state, "pitboard")
	t.Cleanup(func() {
		if s, ok := runningServe(sd); ok {
			syscall.Kill(s.PID, syscall.SIGTERM)
		}
	})
	return st, sd
}

func newServeBoard(t *testing.T, dir, session string) *board.Store {
	t.Helper()
	st := board.Open(dir, session)
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(st.ReportPath(), []byte("<Title>題 "+session+"</Title>\n"), 0o644)
	return st
}

// serveCmd runs serve for session srv1 (or -s given first); a server it
// starts takes any free port, never the person's 7482.
func serveCmd(t *testing.T, args ...string) string {
	t.Helper()
	s := []string{"-s", "srv1"}
	if len(args) > 1 && args[0] == "-s" {
		s, args = slices.Clone(args[:2]), args[2:]
	}
	full := append(append(s, "serve"), args...)
	if !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--st") || a == "--forget" }) {
		full = append(full, "--port", "0")
	}
	var b bytes.Buffer
	if err := run(full, &b); err != nil {
		t.Fatalf("serve %v: %v", args, err)
	}
	return strings.TrimSpace(b.String())
}

func login(t *testing.T, url string) *http.Response {
	t.Helper()
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestServeBackground(t *testing.T) {
	st, sd := serveBoard(t)
	url := serveCmd(t, "--no-open")
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.Contains(url, "/s/srv1/?t=") {
		t.Fatalf("url %q", url)
	}
	s, ok := runningServe(sd)
	if !ok || s.PID == os.Getpid() || url != s.boardURL("srv1") {
		t.Fatalf("state %+v %v", s, ok)
	}
	for _, p := range []string{statePath(sd), logPath(sd), registryPath(sd)} {
		if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v %v", p, fi, err)
		}
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "serve.json")); !os.IsNotExist(err) {
		t.Errorf("serve.json in the session: %v", err)
	}
	// It answers, and the token works: to the board's page.
	if resp := login(t, url); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/s/srv1/" {
		t.Fatalf("login: %v", resp.Status)
	}
	// Again: the same server, the same URL.
	if again := serveCmd(t, "--no-open"); again != url {
		t.Errorf("second serve %q, want %q", again, url)
	}
	// Another project's board: the same server, a path of its own.
	other, _ := filepath.EvalSymlinks(t.TempDir())
	newServeBoard(t, other, "srv2")
	t.Chdir(other)
	url2 := serveCmd(t, "-s", "srv2", "--no-open")
	if s2, _ := runningServe(sd); s2.PID != s.PID {
		t.Errorf("another server started: %d %d", s2.PID, s.PID)
	}
	if !strings.Contains(url2, fmt.Sprintf(":%d/s/srv2/?t=%s", s.Port, s.Token)) {
		t.Errorf("second board %q", url2)
	}
	out := serveCmd(t, "--status")
	for _, w := range []string{"port " + strconv.Itoa(s.Port), "srv1  " + st.Base, "srv2  " + other} {
		if !strings.Contains(out, w) {
			t.Errorf("status lacks %q: %q", w, out)
		}
	}
	if strings.Contains(out, s.Token) {
		t.Errorf("status shows the token: %q", out)
	}
	if out := serveCmd(t, "--status", "--url"); !strings.Contains(out, url) || !strings.Contains(out, url2) {
		t.Errorf("status --url %q", out)
	}
	// Forget one: its path goes, the other stays.
	if out := serveCmd(t, "-s", "srv2", "--forget"); !strings.Contains(out, "forgot srv2") {
		t.Errorf("forget %q", out)
	}
	if resp := login(t, url2); resp.Header.Get("Location") != "/" {
		t.Errorf("forgotten board still served: %s", resp.Header.Get("Location"))
	}
	t.Chdir(st.Base)
	// Stop.
	if out := serveCmd(t, "--stop"); !strings.Contains(out, "stopped") {
		t.Errorf("stop %q", out)
	}
	if _, err := os.Stat(statePath(sd)); !os.IsNotExist(err) {
		t.Errorf("serve.json left: %v", err)
	}
	if answers(s.Port) {
		t.Error("still answering")
	}
	if out := serveCmd(t, "--stop"); !strings.Contains(out, "no serve running") {
		t.Errorf("stop again %q", out)
	}
	logb, _ := os.ReadFile(logPath(sd))
	if strings.Contains(string(logb), s.Token) || !strings.Contains(string(logb), "listening") {
		t.Errorf("log: %s", logb)
	}
	// Started again, it shows what was registered (not the forgotten one)
	// at the same paths.
	serveCmd(t, "-s", "srv1", "--no-open")
	s3, _ := runningServe(sd)
	var idx web.Index
	if err := call(s3, http.MethodGet, "/api/boards", nil, &idx); err != nil || len(idx.Boards) != 1 || idx.Boards[0].ID != "srv1" || idx.Boards[0].Title != "題 srv1" {
		t.Errorf("restored %+v %v", idx, err)
	}
}

// A serve.json whose process is gone is not a server: a new one starts.
// One an older pitboard left in the session's directory is deleted.
func TestServeStaleState(t *testing.T) {
	st, sd := serveBoard(t)
	dead := exec.Command("true")
	dead.Run()
	writeState(sd, serveState{PID: dead.Process.Pid, Port: 1, Token: "old", Started: time.Now()})
	old := filepath.Join(st.Dir, "serve.json")
	os.WriteFile(old, []byte(`{"PID":1,"Port":1,"Token":"x"}`), 0o600)
	url := serveCmd(t, "--no-open")
	if strings.Contains(url, "t=old") {
		t.Fatalf("stale state reused: %s", url)
	}
	if s, ok := runningServe(sd); !ok || s.Token == "old" {
		t.Errorf("state %+v", s)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old per-session serve.json left: %v", err)
	}
}

// --stop also stops a per-session server of an older pitboard, if one
// still runs (its serve.json in the session's directory).
func TestServeStopsOldServer(t *testing.T) {
	st, _ := serveBoard(t)
	old := exec.Command("sleep", "30")
	if err := old.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { old.Wait(); close(done) }()
	writeStateFile(t, filepath.Join(st.Dir, "serve.json"), serveState{PID: old.Process.Pid, Start: board.ProcessStart(old.Process.Pid), Port: 1, Token: "x"})
	serveCmd(t, "--stop")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		old.Process.Kill()
		t.Fatal("old server not stopped")
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "serve.json")); !os.IsNotExist(err) {
		t.Errorf("old serve.json left: %v", err)
	}
}

func writeStateFile(t *testing.T, p string, s serveState) {
	b, _ := json.Marshal(s)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The background server stops by itself when no page is connected, but
// not while one is.
func TestServeIdles(t *testing.T) {
	_, sd := serveBoard(t)
	t.Setenv("PITBOARD_SERVE_IDLE", "600ms")
	serveCmd(t, "--no-open")
	s, _ := runningServe(sd)
	// A page connected (its event stream) keeps it.
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/s/srv1/api/events", s.Port), nil)
	req.AddCookie(&http.Cookie{Name: "pitboard_" + strconv.Itoa(s.Port), Value: s.Token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("events: %v %v", resp, err)
	}
	time.Sleep(1500 * time.Millisecond)
	if !board.ProcessLive(s.PID, s.Start) {
		t.Fatal("stopped with a page connected")
	}
	resp.Body.Close()
	for range 60 {
		if !board.ProcessLive(s.PID, s.Start) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if board.ProcessLive(s.PID, s.Start) {
		t.Fatal("did not idle out")
	}
	if _, err := os.Stat(statePath(sd)); !os.IsNotExist(err) {
		t.Errorf("serve.json left after idling out: %v", err)
	}
	if logb, _ := os.ReadFile(logPath(sd)); !strings.Contains(string(logb), "stopped (idle)") {
		t.Errorf("log: %s", logb)
	}
}

// The state directory is $XDG_STATE_HOME/pitboard, else
// ~/.local/state/pitboard.
func TestStateDir(t *testing.T) {
	x := t.TempDir()
	t.Setenv("XDG_STATE_HOME", x)
	if d, err := stateDir(); err != nil || d != filepath.Join(x, "pitboard") {
		t.Errorf("xdg: %s %v", d, err)
	}
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)
	if d, err := stateDir(); err != nil || d != filepath.Join(home, ".local", "state", "pitboard") {
		t.Errorf("home: %s %v", d, err)
	}
	if fi, err := os.Stat(filepath.Join(home, ".local", "state", "pitboard")); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("made %v %v", fi, err)
	}
}

// Without --port, serve takes the default (web.Listen tries 7482 and on);
// with --port it is exact, and a taken one is an error naming it.
func TestServePort(t *testing.T) {
	a, err := parseServe(nil)
	if err != nil || a.o.Port != web.PortDefault {
		t.Errorf("default: %+v %v", a.o.Port, err)
	}
	if a, _ := parseServe([]string{"--port", "0"}); a.o.Port != 0 {
		t.Errorf("--port 0: %d", a.o.Port)
	}
	serveBoard(t)
	hold, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()
	p := strconv.Itoa(hold.Addr().(*net.TCPAddr).Port)
	err = run([]string{"-s", "srv1", "serve", "--no-open", "--port", p}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "port "+p) {
		t.Errorf("taken --port: %v", err)
	}
}

// serves run at once all find the same server and print the same URL
// (REVIEW13 c: they took a lock-free race, and the losers failed).
func TestServeConcurrent(t *testing.T) {
	serveBoard(t)
	const n = 4
	urls := make(chan string, n)
	errs := make(chan error, n)
	for range n {
		go func() {
			var b bytes.Buffer
			if err := run([]string{"-s", "srv1", "serve", "--no-open", "--port", "0"}, &b); err != nil {
				errs <- err
				return
			}
			urls <- strings.TrimSpace(b.String())
		}()
	}
	var got []string
	for range n {
		select {
		case u := <-urls:
			got = append(got, u)
		case err := <-errs:
			t.Fatal(err)
		case <-time.After(30 * time.Second):
			t.Fatal("a serve did not return")
		}
	}
	for _, u := range got[1:] {
		if u != got[0] {
			t.Errorf("different URLs: %q", got)
		}
	}
}

// A state file without the process's start time never takes a live
// process of that pid for the server.
func TestServeStateNeedsStart(t *testing.T) {
	_, sd := serveBoard(t)
	other := exec.Command("sleep", "30")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Process.Kill(); other.Wait() })
	os.MkdirAll(sd, 0o700)
	writeStateFile(t, statePath(sd), serveState{PID: other.Process.Pid, Port: 1, Token: "x"})
	if _, ok := runningServe(sd); ok {
		t.Fatal("a state with no Start was taken as running")
	}
	var b bytes.Buffer
	if err := run([]string{"serve", "--stop"}, &b); err != nil {
		t.Fatal(err)
	}
	if err := other.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("--stop stopped another process: %v", err)
	}
}

// serve.json is 0600 even when a stale serve.json.tmp of another mode is
// left.
func TestStateFileMode(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(statePath(d)+".tmp", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeState(d, serveState{PID: 1, Start: "x", Port: 1, Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(statePath(d))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("serve.json mode %v", fi.Mode().Perm())
	}
}

func TestServeHelp(t *testing.T) {
	var b bytes.Buffer
	if err := run([]string{"serve", "-h"}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "usage: pitboard") || !strings.Contains(b.String(), "--forget") {
		t.Errorf("serve -h: %q", b.String())
	}
}
