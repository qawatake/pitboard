package board

import (
	"sync"
	"testing"
)

func TestAppendNumbers(t *testing.T) {
	st := testStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Append(Event{Op: "thought", Text: "x"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	evs := st.Events()
	if len(evs) != 20 {
		t.Fatalf("events = %d", len(evs))
	}
	for i, e := range evs {
		if e.N != i+1 {
			t.Fatalf("event %d has n=%d", i, e.N)
		}
	}
}
