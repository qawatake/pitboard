package board

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ShortSession is the shortest head of the session id, 8 characters or
// more, that no other session on the same board directory starts with (so
// -s resolves it to this session).
func ShortSession(st *Store) string {
	r := []rune(st.Session)
	others := Sessions(st.Base)
	for l := min(8, len(r)); l < len(r); l++ {
		p, unique := string(r[:l]), true
		for _, o := range others {
			if o != st.Session && strings.HasPrefix(o, p) {
				unique = false
				break
			}
		}
		if unique {
			return p
		}
	}
	return st.Session
}

// A board is per session, in .pitboard/<session>/ under the directory the
// commands run in (base):
//
//	report.md     written by the AI only (with its usual file edits)
//	events.jsonl  written by the viewer (and `pitboard settle`), append-only
//	seen          the last event number `pitboard changes` has delivered
//	waiting       the pid of the process waiting for the person's send
//	stophook      left by `pitboard hook stop`: the Stop hook is installed
type Store struct {
	Base    string // the directory holding .pitboard/; Link paths are relative to it
	Session string
	Dir     string
}

// Open is the board of session under base (it need not exist yet).
func Open(base, session string) *Store {
	return &Store{Base: base, Session: session, Dir: filepath.Join(base, ".pitboard", session)}
}

var ReSession = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Sessions lists the boards under base, most recently updated first.
func Sessions(base string) []string {
	ents, err := os.ReadDir(filepath.Join(base, ".pitboard"))
	if err != nil {
		return nil
	}
	type s struct {
		name string
		t    time.Time
	}
	var ss []s
	for _, e := range ents {
		if !e.IsDir() || !ReSession.MatchString(e.Name()) {
			continue
		}
		st := Open(base, e.Name())
		t := modTime(st.Dir)
		for _, p := range []string{st.ReportPath(), st.EventsPath(), st.SeenPath()} {
			if m := modTime(p); m.After(t) {
				t = m
			}
		}
		ss = append(ss, s{e.Name(), t})
	}
	sort.SliceStable(ss, func(i, j int) bool { return ss[i].t.After(ss[j].t) })
	out := make([]string, len(ss))
	for i, x := range ss {
		out[i] = x.name
	}
	return out
}

// resolveSession turns what was given (-s or $PITBOARD_SESSION) into a
// session name: an existing session by its name or a unique prefix, or a new
// name.
func resolveSession(base, given string) (string, error) {
	if !ReSession.MatchString(given) {
		return "", fmt.Errorf("session %q: use A-Z a-z 0-9 _ -", given)
	}
	var hits []string
	for _, s := range Sessions(base) {
		if s == given {
			return s, nil
		}
		if strings.HasPrefix(s, given) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 0:
		return given, nil // a new session
	case 1:
		return hits[0], nil
	}
	sort.Strings(hits)
	return "", fmt.Errorf("session %q matches %s", given, strings.Join(hits, ", "))
}

// ancestors is dir and the directories above it, nearest first. Under the
// home directory the search stops at home: a board is never looked for
// above it.
func ancestors(dir string) []string {
	dir = filepath.Clean(dir)
	home, _ := os.UserHomeDir()
	if home != "" {
		home = filepath.Clean(home)
	}
	var out []string
	for {
		out = append(out, dir)
		if dir == home {
			return out
		}
		up := filepath.Dir(dir)
		if up == dir {
			return out
		}
		dir = up
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// FindSession looks for the board of the session given, from dir upwards:
// the first directory with .pitboard/<session>/ wins. With prefix, an
// existing session may be given by a unique prefix (checked level by level).
// ok is false when there is no such board anywhere.
func FindSession(dir, given string, prefix bool) (base, name string, ok bool, err error) {
	if !ReSession.MatchString(given) {
		return "", "", false, fmt.Errorf("session %q: use A-Z a-z 0-9 _ -", given)
	}
	for _, d := range ancestors(dir) {
		name := given
		if prefix {
			if name, err = resolveSession(d, given); err != nil {
				return "", "", false, err
			}
		}
		if isDir(Open(d, name).Dir) {
			return d, name, true, nil
		}
	}
	return "", "", false, nil
}

// minIDHead is the shortest head of a session id a board may be named by
// and still be found from the whole id (the help tells the AI to name the
// board by the first 8 characters of its Claude Code session id).
const minIDHead = 8

// FindSessionByID looks for the board of the session whose whole id is
// given (what Claude Code hands a hook), from dir upwards. A board is named
// by the id itself or by a head of it, minIDHead characters or more; in one
// directory the id itself wins, then the longest head. name is the board's
// name. ok is false when there is no such board anywhere.
func FindSessionByID(dir, id string) (base, name string, ok bool) {
	if !ReSession.MatchString(id) {
		return "", "", false
	}
	for _, d := range ancestors(dir) {
		if isDir(Open(d, id).Dir) {
			return d, id, true
		}
		best := ""
		for _, s := range Sessions(d) {
			if len(s) >= minIDHead && len(s) > len(best) && strings.HasPrefix(id, s) {
				best = s
			}
		}
		if best != "" {
			return d, best, true
		}
	}
	return "", "", false
}

// Root is where boards live for commands run in dir: the nearest
// directory (from dir upwards) that has a .pitboard/ directory, or dir.
func Root(dir string) string {
	for _, d := range ancestors(dir) {
		if isDir(filepath.Join(d, ".pitboard")) {
			return d
		}
	}
	return dir
}

func modTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

func (s *Store) ReportPath() string { return filepath.Join(s.Dir, "report.md") }

func (s *Store) EventsPath() string { return filepath.Join(s.Dir, "events.jsonl") }

func (s *Store) SeenPath() string { return filepath.Join(s.Dir, "seen") }
