package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// TabHeader carries the browser tab's id (kept in its sessionStorage):
// each tab has its own undo history.
const TabHeader = "X-Pitboard-Tab"

// maxJSON is the largest JSON body taken, but for images.
const maxJSON = 1 << 20

// PageJSON is what GET /api/page returns.
type PageJSON struct {
	*page.Page
	Session  string // the session's short id ("" none)
	NoBoard  bool   // there is no .pitboard/ at all (MsgNoBoard)
	Base     string // where boards are looked for, when there is none
	ImageMax int    // the largest image taken, in bytes
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	t := s.tabOf(r.Header.Get(TabHeader))
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fresh()
	p := t.b.Page(r.URL.Query().Get("hidden") == "1")
	t.remember(p)
	out := PageJSON{Page: p, ImageMax: board.ImageMax}
	if st := t.b.Store; st != nil {
		out.Session = board.ShortSession(st)
	} else {
		if fi, err := os.Stat(s.base + "/.pitboard"); err != nil || !fi.IsDir() {
			out.NoBoard, out.Page.Message = true, page.MsgNoBoard
		}
		out.Base = s.base
	}
	writeJSON(w, out)
}

// remember keeps the lines of p by token, as the tab was shown them: an
// action carries the token, and is done on the line as the person saw it.
func (t *tab) remember(p *page.Page) {
	if len(t.seen) > 20000 {
		t.seen = map[string]seenLine{}
	}
	var walk func(ts []*page.Thing)
	walk = func(ts []*page.Thing) {
		for _, x := range ts {
			if x.Stop && x.Node != nil {
				t.seen[x.Token] = seenLine{seen: x.Seen, target: x.Target, title: x.Node.Title()}
			}
			walk(x.Kids)
		}
	}
	walk(p.Notices)
	walk(p.Things)
}

// post reads a JSON body into f's request.
func (s *Server) post(w http.ResponseWriter, r *http.Request, f func(w http.ResponseWriter, r *http.Request, body []byte)) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		http.Error(w, "pitboard: JSON で送ってください", http.StatusUnsupportedMediaType)
		return
	}
	limit := int64(maxJSON)
	if r.URL.Path == "/api/image" {
		limit = int64(board.ImageMax)*4/3 + 4096 // base64
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, Result{Message: board.ErrTooLarge.Error()})
			return
		}
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	f(w, r, body)
}

// Act is what POST /api/act takes.
type Act struct {
	Action string // react hide comment edit retract read readall undo
	Target string // the line's Key ("" for none)
	Seen   string // the line's Token, as shown
	Value  string // react: y, n or an option key
	Text   string // comment, edit
	N      int    // edit, retract: the comment's number
	// ShowHidden: the page shows hidden lines (the words after hiding
	// differ).
	ShowHidden bool
}

// Result is what an action returns: Message is the same notice the
// terminal shows. Changed: the line no longer says what the person saw;
// nothing was recorded, and the page should be fetched again.
type Result struct {
	OK      bool
	Message string
	Changed bool `json:",omitempty"`
	Hidden  bool `json:",omitempty"` // hide: the line is hidden now
	Image   int  `json:",omitempty"` // image: its number
}

func (s *Server) handleAct(w http.ResponseWriter, r *http.Request, body []byte) {
	var a Act
	if err := json.Unmarshal(body, &a); err != nil {
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	t := s.tabOf(r.Header.Get(TabHeader))
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fresh()
	writeJSON(w, t.act(a))
}

// line is the line a tab was shown with token tok, if it is the line key.
func (t *tab) line(key, tok string) (seenLine, bool) {
	l, ok := t.seen[tok]
	return l, ok && key != "" && l.seen.Key == key
}

func (t *tab) act(a Act) Result {
	b := t.b
	res := func(msg string, err error) Result {
		switch {
		case errors.Is(err, page.ErrChanged):
			return Result{Message: page.MsgChanged, Changed: true}
		case err != nil:
			return Result{Message: err.Error()}
		}
		return Result{OK: true, Message: msg}
	}
	changed := Result{Message: page.MsgChanged, Changed: true}
	switch a.Action {
	case "react":
		l, ok := t.line(a.Target, a.Seen)
		if !ok {
			return changed
		}
		return res(b.React(l.seen, a.Value))
	case "hide":
		l, ok := t.line(a.Target, a.Seen)
		if !ok {
			return changed
		}
		msg, hidden, err := b.Hide(l.seen)
		if err == nil && hidden && !a.ShowHidden {
			msg += " (H で表示)"
		}
		out := res(msg, err)
		out.Hidden = hidden
		return out
	case "comment":
		target, title := "", ""
		if a.Target != "" {
			l, ok := t.line(a.Target, a.Seen)
			if !ok || l.target == "" {
				return changed
			}
			target, title = l.target, l.title
		}
		return res(b.Comment(target, title, a.Text))
	case "edit":
		return res(b.Edit(a.N, a.Text))
	case "retract":
		msg, err := b.Retract(a.N)
		if err == nil {
			msg += " (u で戻す)"
		}
		return res(msg, err)
	case "read":
		l, ok := t.line(a.Target, a.Seen)
		if !ok {
			return changed
		}
		// Only on the line as it is now: a line rewritten since is the
		// person's to look at again.
		var now *report.Node
		report.WalkAll(b.Report.Nodes, func(n *report.Node) {
			if now == nil && n.Key == l.seen.Key {
				now = n
			}
		})
		if now == nil || page.SeenOf(now) != l.seen {
			return changed
		}
		return res(b.Read(l.seen.Key, "r (読んだ)"))
	case "readall":
		return res(b.ReadAll("A (すべて読んだことにする)"))
	case "undo":
		return res(b.Undo())
	}
	return Result{Message: fmt.Sprintf("知らない操作です: %q", a.Action)}
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request, body []byte) {
	t := s.tabOf(r.Header.Get(TabHeader))
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fresh()
	msg, err := t.b.Send(nil)
	if err != nil {
		writeJSON(w, Result{Message: err.Error()})
		return
	}
	writeJSON(w, Result{OK: true, Message: msg})
}

// ImageUpload is what POST /api/image takes: the image, base64.
type ImageUpload struct{ Data string }

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request, body []byte) {
	var u ImageUpload
	if err := json.Unmarshal(body, &u); err != nil {
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	data, err := base64.StdEncoding.DecodeString(u.Data)
	if err != nil {
		writeJSON(w, Result{Message: page.ErrNotImage.Error()})
		return
	}
	t := s.tabOf(r.Header.Get(TabHeader))
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fresh()
	n, err := t.b.SaveImage(data)
	if err != nil {
		writeJSON(w, Result{Message: err.Error()})
		return
	}
	writeJSON(w, Result{OK: true, Image: n, Message: fmt.Sprintf("[Image #%d] を貼りました", n)})
}

// handleImageGet serves the session's image by number: no path ever comes
// from the browser, and only what is an image is served.
func (s *Server) handleImageGet(w http.ResponseWriter, r *http.Request, num string) {
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 || strconv.Itoa(n) != num {
		http.NotFound(w, r)
		return
	}
	st, _ := s.store()
	b := &page.Board{Store: st} // only its images: the report is not read
	p := b.ImageFile(n)
	if p == "" {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) > board.ImageMax {
		http.NotFound(w, r)
		return
	}
	ct := http.DetectContentType(data)
	if !page.IsImageType(ct) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}
