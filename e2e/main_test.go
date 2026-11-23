// Package e2e_test is a black-box regression suite for docs/spec/SPEC.md and
// docs/spec/SPEC-format.md. It builds ./cmd/pitboard once and drives the
// binary only through its command line and, for the viewer, a pty. Nothing
// here imports the module's internal packages.
package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bin is the pitboard binary built by TestMain.
var bin string

// toolDir holds the only external commands the binary may find on PATH:
// stty (the viewer's raw mode and size), ps (the waiting file's liveness
// check) and sh (PITBOARD_SEND). No herdr, no osascript, no clipboard tools.
var toolDir string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "pitboard-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	bin = filepath.Join(dir, "pitboard")
	_, self, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(self))
	build := exec.Command("go", "build", "-o", bin, "./cmd/pitboard")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building pitboard: %v\n%s", err, out)
		return 1
	}
	toolDir = filepath.Join(dir, "tools")
	if err := os.Mkdir(toolDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, name := range []string{"stty", "ps", "sh", "cat", "printf", "sleep", "echo"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if err := os.Symlink(p, filepath.Join(toolDir, name)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return m.Run()
}

// sandbox is one isolated world: a home directory, a project inside it,
// and the environment the binary runs with.
type sandbox struct {
	t    *testing.T
	home string
	dir  string // the project directory (cwd of commands)
	env  map[string]string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	// Resolve symlinks (/var -> /private/var on macOS) so paths printed by
	// the binary compare equal.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &sandbox{t: t, home: home, dir: dir, env: map[string]string{
		"HOME":   home,
		"PATH":   toolDir,
		"TERM":   "xterm-256color",
		"LANG":   "en_US.UTF-8",
		"TZ":     "UTC",
		"VISUAL": "",
		"EDITOR": "",
	}}
}

func (s *sandbox) environ() []string {
	var out []string
	for k, v := range s.env {
		out = append(out, k+"="+v)
	}
	return out
}

type result struct {
	stdout, stderr string
	code           int
}

// run runs the binary in the sandbox's project directory.
func (s *sandbox) run(args ...string) result {
	s.t.Helper()
	return s.runIn(s.dir, "", args...)
}

func (s *sandbox) runIn(dir, stdin string, args ...string) result {
	s.t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Env = s.environ()
	c.Stdin = strings.NewReader(stdin)
	var o, e bytes.Buffer
	c.Stdout, c.Stderr = &o, &e
	err := c.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			s.t.Fatalf("running pitboard %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return result{o.String(), e.String(), code}
}

// mustRun runs and fails the test on a non-zero exit.
func (s *sandbox) mustRun(args ...string) string {
	s.t.Helper()
	r := s.run(args...)
	if r.code != 0 {
		s.t.Fatalf("pitboard %v: exit %d\nstdout:\n%s\nstderr:\n%s", args, r.code, r.stdout, r.stderr)
	}
	return r.stdout
}

// session makes a board for id with `pitboard path` and returns it.
func (s *sandbox) session(id string) *session {
	s.t.Helper()
	p := strings.TrimSpace(s.mustRun("-s", id, "path"))
	return &session{s: s, id: id, report: p, dir: filepath.Dir(p)}
}

type session struct {
	s      *sandbox
	id     string
	report string // path of report.md
	dir    string // .pitboard/<id>
}

func (ss *session) write(report string) {
	ss.s.t.Helper()
	writeFile(ss.s.t, ss.report, report)
}

func (ss *session) run(args ...string) result {
	ss.s.t.Helper()
	return ss.s.run(append([]string{"-s", ss.id}, args...)...)
}

func (ss *session) mustRun(args ...string) string {
	ss.s.t.Helper()
	return ss.s.mustRun(append([]string{"-s", ss.id}, args...)...)
}

// events reads events.jsonl ("" when it does not exist).
func (ss *session) events() string {
	b, _ := os.ReadFile(filepath.Join(ss.dir, "events.jsonl"))
	return string(b)
}

// appendEvents adds raw lines to events.jsonl, as the viewer would have.
func (ss *session) appendEvents(lines ...string) {
	ss.s.t.Helper()
	f, err := os.OpenFile(filepath.Join(ss.dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		ss.s.t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			ss.s.t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write to a temporary file and rename, so a viewer polling the file
	// never reads it half-written.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// dedent removes the common leading indentation and the first newline, so
// reports read naturally in Go source.
func dedent(s string) string {
	s = strings.TrimPrefix(s, "\n")
	lines := strings.Split(s, "\n")
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if min < 0 || n < min {
			min = n
		}
	}
	for i, l := range lines {
		if len(l) >= min && min > 0 {
			lines[i] = l[min:]
		} else if strings.TrimSpace(l) == "" {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
