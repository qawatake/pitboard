package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
)

// The boards a server shows are registered by 'pitboard serve' (a call
// with the token, which only the command line has), kept in the registry
// file, and restored when a server starts again. A board's id is the
// shortest prefix of its session (8 characters at least) that no other
// board registered shares; once given it does not change, so open tabs
// keep their address. An id is never given to another board, not even
// after its board is forgotten or dropped: the registry keeps a tombstone
// (the id, the directory, the session), and the same board registered
// again gets its id back, so its open tabs work again.

// Registered is a board as the registry keeps it.
type Registered struct {
	ID      string
	Dir     string // where .pitboard/ is
	Session string
	Added   time.Time
	// Gone: no longer shown (forgotten, or its directory went). Only its
	// id is kept, for the same board.
	Gone bool `json:",omitempty"`
}

type registryFile struct {
	Boards []Registered
}

// CheckBoard is the board at dir/.pitboard/session, if it may be shown:
// dir an existing directory (its real path is taken), .pitboard and the
// session's directory real directories (not links) owned by this user.
func CheckBoard(dir, session string) (*board.Store, error) {
	if !board.ReSession.MatchString(session) {
		return nil, fmt.Errorf("session %q: not a session name", session)
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("%s: not an absolute path", dir)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{real, filepath.Join(real, ".pitboard"), filepath.Join(real, ".pitboard", session)} {
		fi, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("%s: not a directory", p)
		}
		if p == real {
			continue
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
			return nil, fmt.Errorf("%s: not yours", p)
		}
	}
	return board.Open(real, session), nil
}

// board is the board with id (nil none).
func (s *Server) board(id string) *hosted {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.boards[id]
}

// idOf is the id of st if it is registered ("" if not).
func (s *Server) idOf(st *board.Store) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.order {
		if b.st.Dir == st.Dir || sameDir(b.st.Dir, st.Dir) {
			return b.id
		}
	}
	return ""
}

func sameDir(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// newID is the id for session among the boards registered and gone
// (s.mu held).
func (s *Server) newID(session string) string {
	free := func(p string) bool {
		if s.taken(p) {
			return false
		}
		for _, b := range s.order {
			if strings.HasPrefix(b.st.Session, p) {
				return false
			}
		}
		for _, r := range s.gone {
			if strings.HasPrefix(r.Session, p) {
				return false
			}
		}
		return true
	}
	r := []rune(session)
	for l := min(8, len(r)); l <= len(r); l++ {
		if p := string(r[:l]); free(p) {
			return p
		}
	}
	for n := 2; ; n++ { // the same session in another directory
		if p := session + "~" + strconv.Itoa(n); free(p) {
			return p
		}
	}
}

// Register shows the board at dir/.pitboard/session (see CheckBoard), and
// says its id. A board registered already keeps its id.
func (s *Server) Register(dir, session string) (string, error) {
	st, err := CheckBoard(dir, session)
	if err != nil {
		return "", err
	}
	id, err := s.add(st, "", time.Now())
	if err == nil {
		s.save()
		s.notify("")
	}
	return id, err
}

// taken says whether id is a board's, shown or gone (s.mu held).
func (s *Server) taken(id string) bool {
	if s.boards[id] != nil {
		return true
	}
	for _, r := range s.gone {
		if r.ID == id {
			return true
		}
	}
	return false
}

// add registers st (with id, when restored) unless it is already. A board
// gone before gets its id back.
func (s *Server) add(st *board.Store, id string, added time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.order {
		if b.st.Dir == st.Dir {
			return b.id, nil
		}
	}
	revived := false
	for i, r := range s.gone {
		if r.Session == st.Session && (filepath.Join(r.Dir, ".pitboard", r.Session) == st.Dir || sameDir(filepath.Join(r.Dir, ".pitboard", r.Session), st.Dir)) {
			id, revived = r.ID, true
			s.gone = append(s.gone[:i:i], s.gone[i+1:]...)
			break
		}
	}
	if !revived && (id == "" || !validID(id) || s.taken(id)) {
		id = s.newID(st.Session)
	}
	b := &hosted{id: id, st: st, added: added, tabs: map[string]*tab{}, gallery: s.gallery}
	s.boards[id] = b
	s.order = append(s.order, b)
	return id, nil
}

func validID(id string) bool {
	s, n, _ := strings.Cut(id, "~")
	if _, err := strconv.Atoi(n); n != "" && err != nil {
		return false
	}
	return board.ReSession.MatchString(s)
}

// Forget stops showing the board at dir/.pitboard/session; it says the
// id it had ("" none). Its open pages are told "gone" at once, and its
// calls answer MsgGone, as for a board dropped.
func (s *Server) Forget(dir, session string) string {
	want := filepath.Join(dir, ".pitboard", session)
	s.mu.Lock()
	id := ""
	for _, b := range s.order {
		if b.st.Dir == want || sameDir(b.st.Dir, want) {
			id = b.id
			s.bury(b)
			break
		}
	}
	s.mu.Unlock()
	if id != "" {
		s.save()
		s.notify(id) // its streams, and the index's
	}
	return id
}

// bury stops showing b and keeps its tombstone, so its id goes to no
// other board (s.mu held).
func (s *Server) bury(b *hosted) {
	delete(s.boards, b.id)
	for i, x := range s.order {
		if x == b {
			s.order = append(s.order[:i:i], s.order[i+1:]...)
			break
		}
	}
	s.gone = append(s.gone, Registered{ID: b.id, Dir: b.st.Base, Session: b.st.Session, Added: b.added, Gone: true})
}

// restore registers the boards of the registry that are still there; one
// whose directory went is kept as a tombstone without a word.
func (s *Server) restore() {
	for _, r := range ReadRegistry(s.registry) {
		st, err := CheckBoard(r.Dir, r.Session)
		if r.Gone || err != nil {
			s.mu.Lock()
			if validID(r.ID) && board.ReSession.MatchString(r.Session) && !s.taken(r.ID) {
				r.Gone = true
				s.gone = append(s.gone, r)
			}
			s.mu.Unlock()
			continue
		}
		s.add(st, r.ID, r.Added)
	}
	s.save()
}

// ReadRegistry is the boards kept in the registry file p.
func ReadRegistry(p string) []Registered {
	if p == "" {
		return nil
	}
	var f registryFile
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &f) != nil {
		return nil
	}
	return f.Boards
}

// WriteRegistry keeps bs in the registry file p (0600).
func WriteRegistry(p string, bs []Registered) error {
	b, err := json.MarshalIndent(registryFile{Boards: bs}, "", "  ")
	if err != nil {
		return err
	}
	return WritePrivate(p, append(b, '\n'))
}

// WritePrivate writes data to p readable by this user only (0600), by way
// of a new file beside it with a name of its own (made with O_EXCL and
// 0600, so a file left by someone else, or with a looser mode, is never
// written into, and two writers never touch each other's), whose mode is
// checked before it takes p's place.
func WritePrivate(p string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		if fi, err := os.Lstat(tmp); err != nil {
			werr = err
		} else if fi.Mode().Perm() != 0o600 || !fi.Mode().IsRegular() {
			werr = fmt.Errorf("%s: mode %v, not 0600", tmp, fi.Mode())
		}
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	return os.Rename(tmp, p)
}

// save writes the registry: one write at a time, each of the boards as
// they are when it starts, so the last write has the last change.
func (s *Server) save() {
	if s.registry == "" {
		return
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	bs := make([]Registered, 0, len(s.order)+len(s.gone))
	for _, b := range s.order {
		bs = append(bs, Registered{ID: b.id, Dir: b.st.Base, Session: b.st.Session, Added: b.added})
	}
	bs = append(bs, s.gone...)
	s.mu.Unlock()
	if err := WriteRegistry(s.registry, bs); err != nil {
		logError("pitboard: registry: %v\n", err)
	}
}

// logError says what went wrong to the log (stderr; tests watch it).
var logError = func(format string, a ...any) { fmt.Fprintf(os.Stderr, format, a...) }

// BoardRef is what register and forget take.
type BoardRef struct{ Dir, Session string }

// Registration is what register returns.
type Registration struct {
	ID    string
	Error string `json:",omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request, body []byte) {
	var ref BoardRef
	if err := json.Unmarshal(body, &ref); err != nil {
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	id, err := s.Register(ref.Dir, ref.Session)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJSON(w, Registration{Error: err.Error()})
		return
	}
	writeJSON(w, Registration{ID: id})
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request, body []byte) {
	var ref BoardRef
	if err := json.Unmarshal(body, &ref); err != nil {
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	writeJSON(w, Registration{ID: s.Forget(ref.Dir, ref.Session)})
}

// BoardRow is a board as the index lists it.
type BoardRow struct {
	ID, Dir, Session string
	Where            string // Dir, the home directory as ~
	Title            string
	Summary          []page.Word // 反応待ち 未読 AI 待機中 送信済 未送信, as the board's top right
	Message          string      `json:",omitempty"` // no report yet, …
	Gone             bool        `json:",omitempty"` // its directory is missing
}

// Index is what GET /api/boards returns.
type Index struct {
	Boards []BoardRow
	Words  map[string]string
}

func (s *Server) handleBoards(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.index())
}

// index is the boards as the index lists them now. A board whose
// directory is missing (the watcher has not dropped it yet) says so.
func (s *Server) index() Index {
	s.mu.Lock()
	bs := append([]*hosted(nil), s.order...)
	s.mu.Unlock()
	out := Index{Boards: []BoardRow{}, Words: map[string]string{"offline": page.MsgOffline, "none": MsgNoBoards}}
	for _, b := range bs {
		p := page.Open(b.st).Page(false)
		row := BoardRow{ID: b.id, Dir: b.st.Base, Where: tilde(b.st.Base), Session: b.st.Session, Summary: p.Summary, Message: p.Message}
		for _, t := range p.Things {
			if t.Kind == page.KindTitle {
				for _, sp := range t.Title {
					row.Title += sp.Text
				}
				break
			}
		}
		if _, err := os.Stat(b.st.Dir); err != nil {
			row.Gone, row.Message, row.Summary = true, MsgGone, nil
		}
		out.Boards = append(out.Boards, row)
	}
	return out
}

// MsgNoBoards is what the index says with no board registered.
const MsgNoBoards = "ボードはまだありません。報告のあるディレクトリで pitboard serve を実行すると、ここに並びます"

// tilde is p with the home directory as ~.
func tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return p
	}
	if rest, ok := strings.CutPrefix(p, home); ok && (rest == "" || rest[0] == filepath.Separator) {
		return "~" + rest
	}
	return p
}
