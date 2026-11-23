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
	gen := -1
	for {
		select {
		case <-s.done:
			return
		case <-tick.C:
		}
		st, g := s.store()
		if g != gen || s.watch == nil {
			if gen != -1 {
				s.notify()
			}
			s.watch, gen = page.Open(st), g
			continue
		}
		if s.watch.NoSession && st != nil {
			if fi, err := statDir(st.Dir); err == nil && fi {
				s.watch = page.Open(st)
				s.notify()
			}
			continue
		}
		if s.watch.Changed() {
			s.notify()
		}
	}
}

// notify tells every event stream that the board changed.
func (s *Server) notify() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.subs {
		select {
		case c <- struct{}{}:
		default: // one waiting already says it
		}
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "pitboard: 流せません", http.StatusInternalServerError)
		return
	}
	c := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[c] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, c)
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
