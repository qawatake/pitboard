package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/web"
)

// 'pitboard serve' shows boards in a browser. There is one server per
// user, in the background: serve starts it if it is not running, then
// registers the board (a call with the server's token) and opens its page,
// /s/<id>/. Running it for another session or project adds that board;
// nothing restarts. The server keeps its state in the user's state
// directory ($XDG_STATE_HOME/pitboard or ~/.local/state/pitboard):
// serve.json (pid, when the process started, port, token; 0600) while it
// runs, serve.log (never the token), and boards.json (the boards
// registered, restored when it starts again). It stops by itself when no
// page has been connected for serveIdle.

// serveIdle is how long a background server waits with no page connected
// before it stops ($PITBOARD_SERVE_IDLE overrides it, for tests).
const serveIdle = 30 * time.Minute

// detachedEnv marks the background server's own process.
const detachedEnv = "PITBOARD_SERVE_DETACHED"

// serveState is serve.json.
type serveState struct {
	PID     int
	Start   string // when the process started (ps), to tell a reused pid apart
	Port    int
	Token   string
	Started time.Time
}

func (s serveState) url() string { return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", s.Port, s.Token) }
func (s serveState) boardURL(id string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/s/%s/?t=%s", s.Port, id, s.Token)
}

// stateDir is where the server keeps its state ($XDG_STATE_HOME/pitboard,
// else ~/.local/state/pitboard). It is made (0700) when missing.
func stateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	d := filepath.Join(base, "pitboard")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func statePath(dir string) string    { return filepath.Join(dir, "serve.json") }
func logPath(dir string) string      { return filepath.Join(dir, "serve.log") }
func registryPath(dir string) string { return filepath.Join(dir, "boards.json") }

func readState(p string) (serveState, bool) {
	var s serveState
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &s) != nil {
		return s, false
	}
	// Start is how a pid is told from a reused one: without it, a live
	// process of that pid is never taken for the server.
	return s, s.Start != "" && board.ProcessLive(s.PID, s.Start) && s.Port > 0 && s.Token != ""
}

// runningServe is the server, if one runs (a stale file is ignored).
func runningServe(dir string) (serveState, bool) { return readState(statePath(dir)) }

// answers says whether a server listens at port (any HTTP answer).
func answers(port int) bool {
	c := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/favicon.svg", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func writeState(dir string, s serveState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return web.WritePrivate(statePath(dir), append(b, '\n'))
}

// lockServe holds the state directory's lock until the returned func is
// called: one serve at a time looks for the server and starts it, so
// serves run at once all find the same one.
func lockServe(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, "serve.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// removeState removes serve.json if it is this process's.
func removeState(dir string) {
	var s serveState
	if b, err := os.ReadFile(statePath(dir)); err == nil && json.Unmarshal(b, &s) == nil && s.PID == os.Getpid() {
		os.Remove(statePath(dir))
	}
}

type serveArgs struct {
	o                                web.Options
	foreground, stop, status, forget bool
	showURL, help                    bool
}

const serveUsage = "usage: pitboard [-s SESSION] serve [--port N] [--no-open] [--foreground] | serve --stop | serve --status [--url] | [-s SESSION] serve --forget"

// serveHelp is 'pitboard serve -h'.
const serveHelp = serveUsage + `

  (no flag)       start the server if it is not running, add the board, print
                  its URL and open it
  --port N        listen on port N exactly (0: any free one); default 7482,
                  else the next free up to 7491, else any free one
  --no-open       only print the URL
  --foreground    serve in this terminal (ctrl+c stops it)
  --stop          stop the server
  --status        show the server and its boards (--url: with their URLs)
  --forget        take the board of -s SESSION off the server

The server keeps serve.json, serve.log and boards.json in
$XDG_STATE_HOME/pitboard/ (or ~/.local/state/pitboard/), and stops by itself
after 30 minutes with no page open. 'pitboard -h' says more ("Browser").
`

func parseServe(args []string) (serveArgs, error) {
	a := serveArgs{o: web.Options{Open: true, Port: web.PortDefault}}
	for len(args) > 0 {
		k := args[0]
		v, hasV := "", false
		if kk, val, ok := strings.Cut(k, "="); ok {
			k, v, hasV = kk, val, true
		}
		flag := func(p *bool, val bool) error {
			if hasV {
				return fmt.Errorf("serve: %s takes no value", k)
			}
			*p, args = val, args[1:]
			return nil
		}
		var err error
		switch k {
		case "--no-open", "-no-open":
			err = flag(&a.o.Open, false)
		case "--foreground", "-foreground":
			err = flag(&a.foreground, true)
		case "--stop", "-stop":
			err = flag(&a.stop, true)
		case "--status", "-status":
			err = flag(&a.status, true)
		case "--forget", "-forget":
			err = flag(&a.forget, true)
		case "--url", "-url":
			err = flag(&a.showURL, true)
		case "-h", "--help", "-help":
			err = flag(&a.help, true)
		case "--port", "-port":
			if !hasV {
				if len(args) < 2 {
					return a, errors.New("--port needs a number")
				}
				v, args = args[1], args[1:]
			}
			args = args[1:]
			p, perr := strconv.Atoi(v)
			if perr != nil || p < 0 || p > 65535 {
				return a, fmt.Errorf("--port %q: give a port number (0: any free one)", v)
			}
			a.o.Port = p
		default:
			return a, fmt.Errorf("serve: unknown argument %q (%s)", args[0], serveUsage)
		}
		if err != nil {
			return a, err
		}
	}
	n := 0
	for _, b := range []bool{a.stop, a.status, a.forget} {
		if b {
			n++
		}
	}
	if n > 1 {
		return a, errors.New("serve: give one of --stop, --status, --forget")
	}
	return a, nil
}

// runServe is 'pitboard serve'.
func runServe(base string, st *board.Store, args []string, out io.Writer) error {
	a, err := parseServe(args)
	if err != nil {
		return err
	}
	if a.help {
		_, err := fmt.Fprint(out, serveHelp)
		return err
	}
	dir, err := stateDir()
	if err != nil {
		return err
	}
	// The background server itself: it serves the boards of its registry.
	if os.Getenv(detachedEnv) == "1" {
		return serveForeground(dir, nil, a.o, out)
	}
	if st != nil {
		dropOldState(st, a.stop)
	}
	switch {
	case a.status:
		return serveStatus(dir, a.showURL, out)
	case a.stop:
		for _, name := range board.Sessions(base) {
			dropOldState(board.Open(base, name), true)
		}
		return serveStop(dir, out)
	case a.forget:
		if st == nil {
			return errors.New("serve --forget: no session (give -s SESSION)")
		}
		return serveForget(dir, st, out)
	}
	if st != nil && !isDir(st.Dir) {
		return fmt.Errorf("no session %q yet (pitboard -s %s path creates it)", st.Session, st.Session)
	}
	if a.foreground {
		var bs []*board.Store
		if st != nil {
			bs = []*board.Store{st}
		}
		return serveForeground(dir, bs, a.o, out)
	}
	unlock, err := lockServe(dir)
	if err != nil {
		return err
	}
	s, ok := runningServe(dir)
	if !ok || !answers(s.Port) {
		if s, err = startBackground(dir, a.o.Port); err != nil {
			unlock()
			return err
		}
	}
	unlock()
	url := s.url()
	if st != nil {
		id, err := register(s, "/api/register", st)
		if err != nil {
			return err
		}
		url = s.boardURL(id)
	}
	fmt.Fprintln(out, url)
	if a.o.Open {
		web.OpenBrowser(url)
	}
	return nil
}

// dropOldState deletes the serve.json a per-session server of an older
// pitboard left in the session's directory; with stop, it stops that
// server first if it still runs (best effort).
func dropOldState(st *board.Store, stop bool) {
	p := filepath.Join(st.Dir, "serve.json")
	if s, live := readState(p); live && stop {
		syscall.Kill(s.PID, syscall.SIGTERM)
	}
	os.Remove(p)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// call sends the server a request with its token (the command line's
// way in: no page has it).
func call(s serveState, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", s.Port, path), rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(b, out) != nil && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("serve: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// register shows st on the server s (path: /api/register or /api/forget),
// and says its id.
func register(s serveState, path string, st *board.Store) (string, error) {
	var r web.Registration
	if err := call(s, http.MethodPost, path, web.BoardRef{Dir: st.Base, Session: st.Session}, &r); err != nil {
		return "", err
	}
	if r.Error != "" {
		return "", fmt.Errorf("serve: %s", r.Error)
	}
	return r.ID, nil
}

// serveForeground serves until ctrl+c (or, for the background process,
// SIGTERM or idling out), keeping serve.json while it runs.
func serveForeground(dir string, bs []*board.Store, o web.Options, out io.Writer) error {
	detached := os.Getenv(detachedEnv) == "1"
	o.Out, o.Boards, o.Registry = out, bs, registryPath(dir)
	if detached {
		// Its output is the log: the URL (the token) never goes there.
		o.Out, o.Open = nil, false
		o.Idle = serveIdle
		if d, err := time.ParseDuration(os.Getenv("PITBOARD_SERVE_IDLE")); err == nil && d > 0 {
			o.Idle = d
		}
	}
	if s, ok := runningServe(dir); ok && s.PID != os.Getpid() && answers(s.Port) {
		return fmt.Errorf("serve: already running (pid %d, port %d); pitboard serve --stop stops it", s.PID, s.Port)
	}
	o.Listening = func(port int, token string) {
		s := serveState{PID: os.Getpid(), Start: board.ProcessStart(os.Getpid()), Port: port, Token: token, Started: time.Now()}
		if err := writeState(dir, s); err != nil {
			fmt.Fprintln(os.Stderr, "pitboard: serve.json:", err)
		}
		if detached {
			fmt.Fprintf(os.Stderr, "%s pitboard serve: listening on 127.0.0.1:%d (pid %d)\n", time.Now().Format(time.RFC3339), port, os.Getpid())
		}
	}
	defer removeState(dir)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := web.Run(ctx, o)
	if detached {
		fmt.Fprintf(os.Stderr, "%s pitboard serve: stopped (%v)\n", time.Now().Format(time.RFC3339), errOrNone(err, ctx))
	}
	return err
}

func errOrNone(err error, ctx context.Context) string {
	switch {
	case err != nil:
		return err.Error()
	case ctx.Err() != nil:
		return "signal"
	}
	return "idle"
}

// startBackground starts the server as a process of its own and waits
// until it answers.
func startBackground(dir string, port int) (serveState, error) {
	exe, err := os.Executable()
	if err != nil {
		return serveState{}, err
	}
	logf, err := os.OpenFile(logPath(dir), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return serveState{}, err
	}
	defer logf.Close()
	os.Remove(statePath(dir)) // stale: no live server holds it
	args := []string{"serve", "--foreground", "--no-open"}
	if port != web.PortDefault {
		args = append(args, "--port", strconv.Itoa(port))
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir // no board of its own: it serves its registry's
	cmd.Env = append(os.Environ(), detachedEnv+"=1")
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return serveState{}, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case err := <-exited:
			return serveState{}, fmt.Errorf("serve: the server stopped at once (%v); see %s:\n%s", err, logPath(dir), logTail(logPath(dir)))
		case <-deadline:
			cmd.Process.Signal(syscall.SIGTERM)
			return serveState{}, fmt.Errorf("serve: the server did not answer; see %s", logPath(dir))
		case <-time.After(50 * time.Millisecond):
		}
		if s, ok := runningServe(dir); ok && s.PID == cmd.Process.Pid && answers(s.Port) {
			return s, nil
		}
	}
}

func logTail(p string) string {
	b, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	return strings.Join(lines[max(len(lines)-5, 0):], "\n")
}

// serveStop stops the server.
func serveStop(dir string, out io.Writer) error {
	s, ok := runningServe(dir)
	if !ok {
		fmt.Fprintln(out, "no serve running")
		return nil
	}
	if err := syscall.Kill(s.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("serve --stop: %w", err)
	}
	for range 100 {
		if !board.ProcessLive(s.PID, s.Start) {
			fmt.Fprintf(out, "stopped serve (port %d, pid %d)\n", s.Port, s.PID)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("serve --stop: pid %d did not stop", s.PID)
}

// serveForget stops showing st: on the running server, or in the registry
// kept for the next one.
func serveForget(dir string, st *board.Store, out io.Writer) error {
	id := ""
	if s, ok := runningServe(dir); ok && answers(s.Port) {
		var err error
		if id, err = register(s, "/api/forget", st); err != nil {
			return err
		}
	} else {
		var keep []web.Registered
		for _, r := range web.ReadRegistry(registryPath(dir)) {
			if filepath.Join(r.Dir, ".pitboard", r.Session) == st.Dir || sameBoard(r, st) {
				id = r.ID
				continue
			}
			keep = append(keep, r)
		}
		if id != "" {
			if err := web.WriteRegistry(registryPath(dir), keep); err != nil {
				return err
			}
		}
	}
	if id == "" {
		fmt.Fprintf(out, "session %s is not served\n", st.Session)
		return nil
	}
	fmt.Fprintf(out, "forgot %s (session %s)\n", id, st.Session)
	return nil
}

func sameBoard(r web.Registered, st *board.Store) bool {
	a, err1 := filepath.EvalSymlinks(filepath.Join(r.Dir, ".pitboard", r.Session))
	b, err2 := filepath.EvalSymlinks(st.Dir)
	return err1 == nil && err2 == nil && a == b
}

// serveStatus says whether the server runs, and lists its boards.
func serveStatus(dir string, showURL bool, out io.Writer) error {
	s, ok := runningServe(dir)
	if !ok || !answers(s.Port) {
		fmt.Fprintln(out, "no serve running")
		return nil
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "serve  port %d  pid %d  since %s", s.Port, s.PID, s.Started.Format("2006-01-02 15:04"))
	if showURL {
		fmt.Fprintf(&b, "  %s", s.url())
	}
	b.WriteByte('\n')
	var idx web.Index
	if err := call(s, http.MethodGet, "/api/boards", nil, &idx); err != nil {
		return err
	}
	for _, r := range idx.Boards {
		fmt.Fprintf(&b, "  %s  %s  (session %s)", r.ID, r.Dir, r.Session)
		if showURL {
			fmt.Fprintf(&b, "  %s", s.boardURL(r.ID))
		}
		b.WriteByte('\n')
	}
	if len(idx.Boards) == 0 {
		fmt.Fprintln(&b, "  (no boards)")
	}
	_, err := out.Write(b.Bytes())
	return err
}
