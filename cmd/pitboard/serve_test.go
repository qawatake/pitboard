package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// serveBoard is a board with session "srv1" and a report, as cwd.
func serveBoard(t *testing.T) *board.Store {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	st := board.Open(dir, "srv1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(st.ReportPath(), []byte("<Title>題</Title>\n"), 0o644)
	t.Chdir(dir)
	t.Setenv("PITBOARD_TEST_AS_MAIN", "1")
	t.Setenv("PITBOARD_SESSION", "")
	t.Cleanup(func() {
		if s, ok := runningServe(st); ok {
			syscall.Kill(s.PID, syscall.SIGTERM)
		}
	})
	return st
}

func serveCmd(t *testing.T, args ...string) string {
	t.Helper()
	var b bytes.Buffer
	if err := run(append([]string{"-s", "srv1", "serve"}, args...), &b); err != nil {
		t.Fatalf("serve %v: %v", args, err)
	}
	return strings.TrimSpace(b.String())
}

func TestServeBackground(t *testing.T) {
	st := serveBoard(t)
	url := serveCmd(t, "--no-open")
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.Contains(url, "/?t=") {
		t.Fatalf("url %q", url)
	}
	s, ok := runningServe(st)
	if !ok || s.PID == os.Getpid() || url != s.url() {
		t.Fatalf("state %+v %v", s, ok)
	}
	if fi, err := os.Stat(statePath(st)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("serve.json mode %v %v", fi.Mode(), err)
	}
	// It answers, and the token works.
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(url)
	if err != nil || resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %v %v", resp, err)
	}
	resp.Body.Close()
	// Again: the same server, the same URL.
	if again := serveCmd(t, "--no-open"); again != url {
		t.Errorf("second serve %q, want %q", again, url)
	}
	if s2, _ := runningServe(st); s2.PID != s.PID {
		t.Errorf("another server started: %d %d", s2.PID, s.PID)
	}
	if out := serveCmd(t, "--status"); !strings.Contains(out, "srv1") || !strings.Contains(out, strconv.Itoa(s.Port)) || strings.Contains(out, s.Token) {
		t.Errorf("status %q", out)
	}
	if out := serveCmd(t, "--status", "--url"); !strings.Contains(out, url) {
		t.Errorf("status --url %q", out)
	}
	// Stop.
	if out := serveCmd(t, "--stop"); !strings.Contains(out, "stopped") {
		t.Errorf("stop %q", out)
	}
	if _, err := os.Stat(statePath(st)); !os.IsNotExist(err) {
		t.Errorf("serve.json left: %v", err)
	}
	if answers(s.Port) {
		t.Error("still answering")
	}
	if out := serveCmd(t, "--stop"); !strings.Contains(out, "no serve running") {
		t.Errorf("stop again %q", out)
	}
	logb, _ := os.ReadFile(logPath(st))
	if strings.Contains(string(logb), s.Token) || !strings.Contains(string(logb), "listening") {
		t.Errorf("log: %s", logb)
	}
}

// A serve.json whose process is gone is not a server: a new one starts.
func TestServeStaleState(t *testing.T) {
	st := serveBoard(t)
	dead := exec.Command("true")
	dead.Run()
	writeState(st, serveState{PID: dead.Process.Pid, Port: 1, Token: "old", Started: time.Now()})
	url := serveCmd(t, "--no-open")
	if strings.Contains(url, "t=old") {
		t.Fatalf("stale state reused: %s", url)
	}
	if s, ok := runningServe(st); !ok || s.Token == "old" {
		t.Errorf("state %+v", s)
	}
}

// The background server stops by itself when no page is connected, but
// not while one is.
func TestServeIdles(t *testing.T) {
	st := serveBoard(t)
	t.Setenv("PITBOARD_SERVE_IDLE", "600ms")
	serveCmd(t, "--no-open")
	s, _ := runningServe(st)
	// A page connected (its event stream) keeps it.
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/events", s.Port), nil)
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
	if _, err := os.Stat(statePath(st)); !os.IsNotExist(err) {
		t.Errorf("serve.json left after idling out: %v", err)
	}
	if logb, _ := os.ReadFile(logPath(st)); !strings.Contains(string(logb), "stopped (idle)") {
		t.Errorf("log: %s", logb)
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
