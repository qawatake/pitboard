package main

import (
	"os"
	"strings"
	"testing"
)

// Every letter the viewer does not keep for itself must answer an Ask, and
// every letter it keeps must be refused by check: when the viewer takes a
// new key, viewerKeys has to follow, or this fails.
func TestViewerKeysMatchTheViewer(t *testing.T) {
	for r := 'a'; r <= 'z'; r++ {
		k := string(r)
		other := "a"
		if k == "a" {
			other = "b"
		}
		src := "<Ask id=\"q\">q\n<Option key=\"" + k + "\">x</Option>\n<Option key=\"" + other + "\">y</Option>\n</Ask>\n"
		_, probs, _ := parseReport(src)
		if strings.Contains(viewerKeys, k) {
			if len(probs) != 1 || !strings.Contains(probs[0].Msg, "viewer key") {
				t.Errorf("%s: check must refuse a viewer key: %v", k, probs)
			}
			continue
		}
		if len(probs) != 0 {
			t.Errorf("%s: %v", k, probs)
			continue
		}
		st := testStore(t)
		os.WriteFile(st.reportPath(), []byte(src), 0o644)
		a := testApp(t, st)
		pv := a.top()
		a.render()
		a.handle(key{r: 'j'})
		a.handle(key{r: 'k'})
		a.render()
		a.handle(key{name: "enter"})
		a.render()
		if !pv.open["q"] {
			t.Fatalf("%s: the Ask did not open", k)
		}
		a.handle(key{r: r})
		if evs := st.Events(); len(evs) != 1 || evs[0].Value != k {
			t.Errorf("%s is not a viewer key, but pressing it did not answer: %+v (add it to viewerKeys?)", k, evs)
		}
	}
}

func TestHelpListsViewerKeys(t *testing.T) {
	if !strings.Contains(usage, "the viewer's own keys "+viewerKeyList()+")") || strings.Contains(usage, "{viewerKeys}") {
		t.Error("-h does not list viewerKeys")
	}
	if !strings.Contains(viewerKeys, "u") {
		t.Error("u (undo) is a viewer key")
	}
}
