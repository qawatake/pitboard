package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

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
	Dir      string // where its .pitboard/ is
	ImageMax int    // the largest image taken, in bytes
	// Say is what to tell the person now (a hidden line came back), once.
	Say string `json:",omitempty"`
	// Words is what the page says by itself, by name: offline (it cannot
	// reach the server: it cannot ask then, so it is given now), jumping (g
	// waits for a letter), noMarked (space has nowhere to go),
	// imageTooLarge, signClosed / signOpen (page.SignClosed, SignOpen: the
	// signs drawn as triangles).
	Words map[string]string
	// More and Tail are the keys of the bar after the line's own (page's
	// Bar, in the terminal's order).
	More, Tail []page.Hint
	// Comment is the keys on a comment of the comment panel
	// (page.CommentHints), Editor the only keys while writing
	// (page.EditorHints).
	Comment, Editor []page.Hint
	// Clears is when the message on the bottom line goes (page.MsgClears):
	// key, editor, panel, pointer.
	Clears []string
	// Gallery: the server serves the gallery's check, and the page may be
	// put in a state by ?ui= (Options.Gallery). Never on a plain serve.
	Gallery bool `json:",omitempty"`
	// Palette is the colors' meanings, for the keys list (page.Palette).
	Palette []PaletteWord
	// Drop is, for each key's Do, the order the bar leaves it out in when
	// there is no room (page.DropRank: lowest first; absent, never).
	Drop map[string]int
}

var words = map[string]string{
	"offline": page.MsgOffline, "jumping": page.MsgJumpLetter, "noMarked": page.MsgNoMarked,
	"imageTooLarge": board.ErrTooLarge.Error(),
	"signClosed":    page.SignClosed, "signOpen": page.SignOpen,
	"gone": MsgGone, "notSent": page.MsgNotSent, "go": page.WordGo, "close": page.WordClose, "noComments": MsgNoComments,
	"linkHidden": page.MsgLinkHidden, "back": page.WordBack,
}

// PaletteWord is a color of the palette, as the keys list says it.
type PaletteWord struct{ Name, Label string }

var palette = func() []PaletteWord {
	var out []PaletteWord
	for _, c := range page.Palette {
		out = append(out, PaletteWord{c.Name, c.Label})
	}
	return out
}()

// GalleryNow is the clock of a server for the gallery (Options.Gallery),
// and the time the gallery's fixtures are made at.
var GalleryNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))

// MsgNoComments is what the comment panel says with no comment.
const MsgNoComments = "コメントはまだありません。m で書けます"

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func (b *hosted) handlePage(w http.ResponseWriter, r *http.Request) {
	t := b.tabOf(r.Header.Get(TabHeader))
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fresh()
	out := t.pageJSON(b.st, r.URL.Query().Get("hidden") == "1")
	out.Gallery = b.gallery
	t.say = ""
	body, err := json.Marshal(out)
	if err != nil {
		http.Error(w, "pitboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// The same page as last time is not sent again: the browser keeps
	// what it drew.
	sum := sha256.Sum256(body)
	tag := `"` + hex.EncodeToString(sum[:12]) + `"`
	w.Header().Set("ETag", tag)
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
	// The person's page shows these rows now (see board.Store.RememberShown).
	if !b.gallery && b.st != nil {
		b.st.RememberShown()
	}
}

// pageJSON is the page as the tab is to be shown it now (and remembered
// by token, for its actions).
func (t *tab) pageJSON(st *board.Store, showHidden bool) PageJSON {
	p := t.b.Page(showHidden)
	t.remember(p)
	out := PageJSON{Page: p, ImageMax: board.ImageMax, Say: t.say, Words: words, Clears: clears}
	out.More, out.Tail = t.b.Bar(showHidden)
	out.Drop = page.DropRanks()
	out.Palette = palette
	out.Comment = page.CommentHints(p.CanWrite)
	out.Editor = page.EditorHints("shift+enter")
	out.Session, out.Dir = board.ShortSession(st), st.Base
	return out
}

// clears is the events that take the message off the bottom line
// (page.MsgClears), by the names the page uses.
var clears = func() []string {
	var out []string
	for e, n := range map[page.MsgEvent]string{page.OnKey: "key", page.OnEditor: "editor", page.OnPanel: "panel", page.OnPointer: "pointer", page.OnReload: "reload"} {
		if page.MsgClears(e) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}()

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
				if _, ok := t.seen[x.Token]; !ok {
					t.seen[x.Token] = seenLine{seen: x.Seen, target: x.Target, title: x.Node.Title(), node: x.Node}
				}
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
	limit, tooBigMsg := int64(maxJSON), page.MsgTooLong
	if strings.HasSuffix(r.URL.Path, "/api/image") {
		limit, tooBigMsg = int64(board.ImageMax)*4/3+4096, board.ErrTooLarge.Error() // base64
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, Result{Message: tooBigMsg})
			return
		}
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	f(w, r, body)
}

// Act is what POST /api/act takes.
type Act struct {
	// Action is key (a key pressed on the cursor's line, or its button),
	// hint (what works there now), comment, edit, retract, readall, undo.
	// react, hide and read are key with Value, x and r.
	Action string
	Key    string // key: y n x r c or an option's key
	Target string // the line's Key ("" for none)
	Seen   string // the line's Token, as shown
	Value  string // react: y, n or an option key
	Text   string // comment, edit
	N      int    // edit, retract: the comment's number
	// ShowHidden: the page shows hidden lines (the words after hiding
	// differ).
	ShowHidden bool
	// Cur is how the cursor came to the line (key, hint).
	Cur Cur
}

// Cur is the facts the page knows of the cursor's line; the rules are
// page's guard's (page.Touch).
type Cur struct {
	// Start: the person has not moved the cursor since the page put it on
	// a line by itself (at start, or when lines appeared on a page that had
	// none); the page may have moved it on since (app).
	Start bool
	// How the line became the cursor's: person (the person moved it there:
	// keys, Tab, click), app (the page moved it there after the person's
	// act took their line away, or for space / g), start (the page put it
	// on its first line).
	How string
	// Ago is how long ago it became the cursor's line, in ms.
	Ago int
	// Sel is the line's Token when it became the cursor's ("": Seen).
	Sel string
	// Open says whether the line is open (an Ask's options shown).
	Open bool
	// Folded: open, its done comments folded (page.Touch.Folded).
	Folded bool
}

// Result is what an action returns: Message is the same notice the
// terminal shows. Changed: the line no longer says what the person saw;
// nothing was recorded, and the page should be fetched again.
type Result struct {
	OK      bool
	Message string
	Changed bool   `json:",omitempty"`
	Hidden  bool   `json:",omitempty"` // hide: the line is hidden now
	Image   int    `json:",omitempty"` // image: its number
	Do      string `json:",omitempty"` // key: what it did (page.Do)
	// Offer is, for hint, what works on the line now.
	Offer *OfferJSON `json:",omitempty"`
}

// OfferJSON is page.Offer for the browser.
type OfferJSON struct {
	Hints   []page.Hint
	Options []string
	Recheck int  // ms; 0 none
	Locked  bool // the line was rewritten: nothing works until the person moves
	// Message is what the bar says instead of keys (Locked: MsgMoved, as
	// the terminal's hint line).
	Message string `json:",omitempty"`
}

func (b *hosted) handleAct(w http.ResponseWriter, r *http.Request, body []byte) {
	var a Act
	if err := json.Unmarshal(body, &a); err != nil {
		http.Error(w, "pitboard: 読めませんでした", http.StatusBadRequest)
		return
	}
	t := b.tabOf(r.Header.Get(TabHeader))
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

// now is the line key in the report as it is now (nil: gone).
func (t *tab) now(key string) *report.Node {
	var now *report.Node
	report.WalkAll(t.b.Report.Nodes, func(n *report.Node) {
		if now == nil && key != "" && n.Key == key && page.Stops(n) {
			now = n
		}
	})
	return now
}

// touch is a's facts as page's guard takes them.
func (t *tab) touch(a Act, now *report.Node) page.Touch {
	c := a.Cur
	tc := page.Touch{Placed: c.Start, Open: c.Open, Folded: c.Open && c.Folded, Again: t.again}
	if c.How == "app" {
		tc.Auto, tc.Shown = true, time.Duration(max(c.Ago, 0))*time.Millisecond
	}
	sel := c.Sel
	if sel == "" {
		sel = a.Seen
	}
	// Rewritten (or gone) since the cursor came to it: locked, as the
	// terminal locks its cursor when a reload changes its line.
	l, ok := t.seen[sel]
	tc.Locked = !ok || l.seen.Key != a.Target || page.Rewritten(l.node, now)
	return tc
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
		a.Action, a.Key = "key", a.Value
	case "hide":
		a.Action, a.Key = "key", "x"
	case "read":
		a.Action, a.Key = "key", "r"
	}
	switch a.Action {
	case "hint":
		now := t.now(a.Target)
		if _, ok := t.line(a.Target, a.Seen); !ok || now == nil {
			return Result{OK: true, Offer: &OfferJSON{Locked: true, Message: page.MsgMoved}}
		}
		tc := t.touch(a, now)
		o := b.Offer(now, tc)
		out := &OfferJSON{Hints: o.Hints, Options: o.Options, Recheck: int(o.Recheck.Milliseconds()), Locked: tc.Locked}
		if tc.Locked {
			out.Message = page.MsgMoved
		}
		return Result{OK: true, Offer: out}
	case "key":
		l, ok := t.line(a.Target, a.Seen)
		now := t.now(a.Target)
		if !ok || now == nil {
			return changed
		}
		v := b.Allow(now, a.Key, t.touch(a, now))
		t.again = v.Again
		switch v.Do {
		case page.DoNothing:
			return Result{Message: v.Msg}
		case page.DoOpen, page.DoComment:
			return Result{OK: true, Do: string(v.Do)}
		case page.DoReact:
			out := res(b.React(l.seen, v.Value))
			out.Do = string(v.Do)
			return out
		case page.DoHide:
			msg, hidden, err := b.Hide(l.seen, a.ShowHidden)
			out := res(msg, err)
			out.Hidden, out.Do = hidden, string(v.Do)
			return out
		case page.DoRead:
			// Only on the line as it is now: a line rewritten since is the
			// person's to look at again.
			if page.SeenOf(now) != l.seen {
				return changed
			}
			out := res(b.Read(l.seen.Key, "r (読んだ)"))
			out.Do = string(v.Do)
			return out
		}
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
		// A comment the page just moved the cursor to (after x) waits, as
		// a line does: x pressed twice takes back only the first.
		if (page.Touch{Auto: a.Cur.How == "app", Shown: time.Duration(max(a.Cur.Ago, 0)) * time.Millisecond}).Settling() {
			return Result{}
		}
		return res(b.Retract(a.N))
	case "readall":
		return res(b.ReadAll("A (すべて読んだことにする)"))
	case "undo":
		return res(b.Undo())
	}
	return Result{Message: fmt.Sprintf("知らない操作です: %q", a.Action)}
}

func (b *hosted) handleSend(w http.ResponseWriter, r *http.Request, body []byte) {
	t := b.tabOf(r.Header.Get(TabHeader))
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

func (b *hosted) handleImage(w http.ResponseWriter, r *http.Request, body []byte) {
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
	t := b.tabOf(r.Header.Get(TabHeader))
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
// from the browser, only a regular file (not a link) in the session's
// images/ is read, and only what is an image is served.
func (h *hosted) handleImageGet(w http.ResponseWriter, r *http.Request, num string) {
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 || strconv.Itoa(n) != num {
		http.NotFound(w, r)
		return
	}
	b := &page.Board{Store: h.st} // only its images: the report is not read
	p := b.ImageFile(n)
	if p == "" || !inImages(h.st, p) {
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

// inImages says whether p is a regular file right in st's images/, that
// directory and the file both not links.
func inImages(st *board.Store, p string) bool {
	dir := filepath.Join(st.Dir, "images")
	if filepath.Dir(p) != dir {
		return false
	}
	for _, q := range []string{st.Dir, dir, p} {
		fi, err := os.Lstat(q)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	fi, _ := os.Lstat(p)
	return fi.Mode().IsRegular()
}
