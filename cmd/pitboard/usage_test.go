package main

import (
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/report"
)

func TestHelpListsViewerKeys(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"help", "format"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "the viewer's own keys "+report.ViewerKeyList()+")") || strings.Contains(out.String(), "{viewerKeys}") {
		t.Error("help format does not list viewerKeys")
	}
	if !strings.Contains(report.ViewerKeys, "u") {
		t.Error("u (undo) is a viewer key")
	}
}

// -h is what every session reads in full: it stays small, starts with its
// contents and ends with the topics, each of which 'pitboard help' prints.
func TestHelpIsThinAndEndsWithTopics(t *testing.T) {
	if len(usage) > 14000 {
		t.Errorf("-h is %d bytes; keep what only some sessions need in a topic", len(usage))
	}
	if !strings.Contains(usage[:400], "Contents:") {
		t.Error("-h does not start with its contents")
	}
	if !strings.HasSuffix(usage, topicList()) {
		t.Error("-h does not end with the topic list")
	}
	var out strings.Builder
	if err := run([]string{"help"}, &out); err != nil || out.String() != topicList() {
		t.Errorf("help: %v %q", err, out.String())
	}
	for _, tp := range topics {
		if !strings.Contains(usage, "  "+tp.name+" ") {
			t.Errorf("-h does not list %s", tp.name)
		}
		out.Reset()
		err := run([]string{"help", tp.name}, &out)
		if err != nil || !strings.HasPrefix(out.String(), "pitboard help "+tp.name+":") || strings.Contains(out.String(), "{stopHookNote}") {
			t.Errorf("help %s: %v %q", tp.name, err, out.String())
		}
	}
	if err := run([]string{"help", "nope"}, &out); err == nil || !strings.Contains(err.Error(), "format changes send hooks browser") {
		t.Errorf("help nope: %v", err)
	}
}

// What moved out of -h is in its topic.
func TestHelpTopicsHoldWhatMovedOut(t *testing.T) {
	want := map[string][]string{
		"format":  {"Option    key", `wait="you"`, "Fold      title (required), id (optional; keeps it open"},
		"changes": {"retract thought N", "edit thought N", "unretract thought N", "→ now gone"},
		"send":    {"PITBOARD_SEND", "herdr agent prompt", "tmux send-keys", "The Stop hook now waits"},
		"hooks":   {"pitboard hook post-tool", `"asyncRewake": true, "timeout": 86400`},
		"browser": {"--rotate", "7482", "serve --forget"},
	}
	for name, ws := range want {
		var out strings.Builder
		if err := run([]string{"help", name}, &out); err != nil {
			t.Fatal(err)
		}
		for _, w := range ws {
			if !strings.Contains(out.String(), w) {
				t.Errorf("help %s lacks %q", name, w)
			}
		}
	}
	for _, w := range []string{"herdr agent prompt", "asyncRewake", "--rotate", "unretract thought N"} {
		if strings.Contains(usage, w) {
			t.Errorf("-h still has %q (it belongs in a topic)", w)
		}
	}
}
