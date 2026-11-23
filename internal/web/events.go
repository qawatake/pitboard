package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/qawatake/pitboard/internal/page"
)

// The page is told when the board changes (the report, the record, what
// was looked at, a waiting AI): one watcher polls at the viewer's pace and
// every event stream gets "changed"; the page then fetches /api/page.

// keepAlive is how often a stream says it is still there.
const keepAlive = 15 * time.Second

func (s *Server) watchLoop() {
	tick := time.NewTicker(PollInterval)
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-tick.C:
		}
		s.watchOnce()
	}
}

// watchOnce is one poll of every board.
func (s *Server) watchOnce() {
	s.mu.Lock()
	bs := append([]*hosted(nil), s.order...)
	s.mu.Unlock()
	for _, b := range bs {
		if !b.there() {
			// Its directory went (moved, deleted): after dropAfter
			// polls in a row it is dropped, and its pages are told so.
			// One miss may be a move and back, or a slow disk.
			if b.missed++; b.missed >= dropAfter {
				s.drop(b)
			}
			continue
		}
		b.missed = 0
		if b.poll() {
			s.notify(b.id)
		}
	}
}

// dropAfter is how many polls in a row must miss a board's directory
// before the board is dropped.
const dropAfter = 3

// poll says whether the board changed since it was last polled.
func (b *hosted) poll() bool {
	switch {
	case b.watch == nil:
		b.watch = page.Open(b.st)
		return false
	case b.watch.NoSession:
		if fi, err := statDir(b.st.Dir); err == nil && fi {
			b.watch = page.Open(b.st)
			return true
		}
		return false
	}
	return b.watch.Changed()
}

// there says whether the board's session directory is still there.
func (b *hosted) there() bool {
	ok, err := statDir(b.st.Dir)
	return err == nil && ok
}

// drop stops showing b (its directory went): the registry keeps only its
// tombstone, and its event streams get "gone" and end.
func (s *Server) drop(b *hosted) {
	s.mu.Lock()
	if s.boards[b.id] != b {
		s.mu.Unlock()
		return
	}
	s.bury(b)
	s.mu.Unlock()
	s.save()
	s.notify(b.id) // its streams, and the index's
}

// MsgGone is what a page of a board that is no longer shown says
// (forgotten, or its directory went): on its banner, and as the answer to
// its calls. Registered again, the board has the same id, and its page
// works again.
const MsgGone = "この pitboard は見つからなくなりました。そのディレクトリで pitboard serve をもう一度動かすと戻ります"

// notify tells the event streams of board id, and the index's, that it
// changed (id "": the index's only).
func (s *Server) notify(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c, of := range s.subs {
		if of != id && of != "" {
			continue
		}
		select {
		case c <- struct{}{}:
		default: // one waiting already says it
		}
	}
}

// handleEvents is the event stream of board id ("" the index).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, id string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "pitboard: 流せません", http.StatusInternalServerError)
		return
	}
	c := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[c] = id
	s.lastSub = time.Now()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, c)
		s.lastSub = time.Now()
		s.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")
	fl.Flush()
	ka := time.NewTicker(keepAlive)
	defer ka.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case <-ka.C:
			fmt.Fprint(w, ": still here\n\n")
		case <-c:
			if id != "" && s.board(id) == nil {
				fmt.Fprintf(w, "event: gone\ndata: %s\n\n", MsgGone)
				fl.Flush()
				return
			}
			fmt.Fprint(w, "event: changed\ndata: 1\n\n")
		}
		fl.Flush()
	}
}

// Modes is every mode of the board's page (its uiState's mode; state.js
// MODES is the same list, and each has a variant in the layer that owns
// it, see docs/spec/web-states.md): normal, goto (g waits for a letter),
// editor, help (the keys), comments (the comment panel), image (an image
// shown large).
var Modes = []string{"normal", "goto", "editor", "help", "comments", "image"}

// Conns is every state of the page's connection: live, offline (the
// server cannot be reached, or no longer takes the page), gone (its board
// is no longer shown: MsgGone).
var Conns = []string{"live", "offline", "gone"}
