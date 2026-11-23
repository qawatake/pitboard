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
		s.mu.Lock()
		bs := append([]*hosted(nil), s.order...)
		s.mu.Unlock()
		for _, b := range bs {
			if b.poll() {
				s.notify(b.id)
			}
		}
	}
}

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
			fmt.Fprint(w, "event: changed\ndata: 1\n\n")
		}
		fl.Flush()
	}
}
