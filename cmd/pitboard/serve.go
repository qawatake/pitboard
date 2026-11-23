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

// 'pitboard serve' runs in the background: it starts a server for the
// session (or finds the one running), prints its URL, opens it and
// returns. The server keeps .pitboard/<session>/serve.json (pid, when the
// process started, port, token) while it runs, writes what it has to say
// to serve.log (never the token), and stops by itself when no page has
// been connected for serveIdle.

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

func statePath(st *board.Store) string { return filepath.Join(st.Dir, "serve.json") }
func logPath(st *board.Store) string   { return filepath.Join(st.Dir, "serve.log") }

// runningServe is the session's server, if one runs (a stale file is
// ignored).
func runningServe(st *board.Store) (serveState, bool) {
	var s serveState
	b, err := os.ReadFile(statePath(st))
	if err != nil || json.Unmarshal(b, &s) != nil {
		return s, false
	}
	return s, board.ProcessLive(s.PID, s.Start) && s.Port > 0 && s.Token != ""
}

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

func writeState(st *board.Store, s serveState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(st) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(st))
}

// removeState removes serve.json if it is this process's.
func removeState(st *board.Store) {
	var s serveState
	if b, err := os.ReadFile(statePath(st)); err == nil && json.Unmarshal(b, &s) == nil && s.PID == os.Getpid() {
		os.Remove(statePath(st))
	}
}

type serveArgs struct {
	o                        web.Options
	foreground, stop, status bool
	showURL                  bool
}

const serveUsage = "usage: pitboard serve [--port N] [--no-open] [--foreground] | serve --stop | serve --status [--url]"

func parseServe(args []string) (serveArgs, error) {
	a := serveArgs{o: web.Options{Open: true}}
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
		case "--url", "-url":
			err = flag(&a.showURL, true)
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
	if a.stop && a.status {
		return a, errors.New("serve: give --stop or --status, not both")
	}
	return a, nil
}

// runServe is 'pitboard serve'.
func runServe(base string, st *board.Store, args []string, out io.Writer) error {
	a, err := parseServe(args)
	if err != nil {
		return err
	}
	switch {
	case a.status:
		return serveStatus(base, a.showURL, out)
	case a.stop:
		if st == nil {
			return errors.New("serve --stop: no session (give -s SESSION)")
		}
		return serveStop(st, out)
	}
	a.o.Base, a.o.Store = base, st
	// Without a session yet (nothing to keep its state in), it stays in
	// the foreground, waiting for the first session as the viewer does.
	hasDir := st != nil && isDir(st.Dir)
	if a.foreground || !hasDir {
		if !a.foreground {
			fmt.Fprintln(os.Stderr, "pitboard: no session yet; serving in the foreground (ctrl+c stops)")
		}
		return serveForeground(st, hasDir, a.o, out)
	}
	if s, ok := runningServe(st); ok && answers(s.Port) {
		return showServe(s, a.o.Open, out)
	}
	s, err := startBackground(st, a.o.Port)
	if err != nil {
		return err
	}
	return showServe(s, a.o.Open, out)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func showServe(s serveState, open bool, out io.Writer) error {
	fmt.Fprintln(out, s.url())
	if open {
		web.OpenBrowser(s.url())
	}
	return nil
}

// serveForeground serves until ctrl+c (or, for the background process,
// SIGTERM or idling out), keeping serve.json while it runs.
func serveForeground(st *board.Store, keepState bool, o web.Options, out io.Writer) error {
	detached := os.Getenv(detachedEnv) == "1"
	o.Out = out
	if detached {
		// Its output is the log: the URL (the token) never goes there.
		o.Out = nil
		o.Idle = serveIdle
		if d, err := time.ParseDuration(os.Getenv("PITBOARD_SERVE_IDLE")); err == nil && d > 0 {
			o.Idle = d
		}
	}
	if keepState {
		if s, ok := runningServe(st); ok && s.PID != os.Getpid() && answers(s.Port) {
			return fmt.Errorf("serve: already running for session %s (pid %d): %s", st.Session, s.PID, s.url())
		}
		o.Listening = func(port int, token string) {
			s := serveState{PID: os.Getpid(), Start: board.ProcessStart(os.Getpid()), Port: port, Token: token, Started: time.Now()}
			if err := writeState(st, s); err != nil {
				fmt.Fprintln(os.Stderr, "pitboard: serve.json:", err)
			}
			if detached {
				fmt.Fprintf(os.Stderr, "%s pitboard serve: listening on 127.0.0.1:%d (session %s, pid %d)\n", time.Now().Format(time.RFC3339), port, st.Session, os.Getpid())
			}
		}
		defer removeState(st)
	}
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

// startBackground starts the session's server as a process of its own and
// waits until it answers.
func startBackground(st *board.Store, port int) (serveState, error) {
	exe, err := os.Executable()
	if err != nil {
		return serveState{}, err
	}
	logf, err := os.OpenFile(logPath(st), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return serveState{}, err
	}
	defer logf.Close()
	os.Remove(statePath(st)) // stale: no live server holds it
	cmd := exec.Command(exe, "-s", st.Session, "serve", "--foreground", "--no-open", "--port", strconv.Itoa(port))
	cmd.Dir = st.Base // where .pitboard/ is
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
			return serveState{}, fmt.Errorf("serve: the server stopped at once (%v); see %s:\n%s", err, logPath(st), logTail(logPath(st)))
		case <-deadline:
			cmd.Process.Signal(syscall.SIGTERM)
			return serveState{}, fmt.Errorf("serve: the server did not answer; see %s", logPath(st))
		case <-time.After(50 * time.Millisecond):
		}
		if s, ok := runningServe(st); ok && s.PID == cmd.Process.Pid && answers(s.Port) {
			return s, nil
		}
	}
}

func logTail(p string) string {
	b, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	return strings.Join(lines[max(len(lines)-5, 0):], "\n")
}

// serveStop stops the session's server.
func serveStop(st *board.Store, out io.Writer) error {
	s, ok := runningServe(st)
	if !ok {
		fmt.Fprintf(out, "no serve running for session %s\n", st.Session)
		return nil
	}
	if err := syscall.Kill(s.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("serve --stop: %w", err)
	}
	for range 100 {
		if !board.ProcessLive(s.PID, s.Start) {
			fmt.Fprintf(out, "stopped serve for session %s (port %d, pid %d)\n", st.Session, s.Port, s.PID)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("serve --stop: pid %d did not stop", s.PID)
}

// serveStatus lists the servers of every session of the board.
func serveStatus(base string, showURL bool, out io.Writer) error {
	var b bytes.Buffer
	for _, name := range board.Sessions(base) {
		st := board.Open(base, name)
		s, ok := runningServe(st)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%s  port %d  pid %d  since %s", board.ShortSession(st), s.Port, s.PID, s.Started.Format("2006-01-02 15:04"))
		if showURL {
			fmt.Fprintf(&b, "  %s", s.url())
		}
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		fmt.Fprintln(out, "no serve running")
		return nil
	}
	_, err := out.Write(b.Bytes())
	return err
}
