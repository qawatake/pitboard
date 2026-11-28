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
	if len(usage) > 15000 {
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

// What every session needs stays in -h, in substance.
func TestHelpKeepsWhatEverySessionNeeds(t *testing.T) {
	for _, w := range []string{
		// Editing report.md safely: unique old_string, and what is not unique.
		"old_string unique", `include the
    row's id (<Item id="x")`, "</Item>, </Fold>", `<Option key="a">, repeated in every Ask`,
		"Run 'pitboard check' after every rewrite",
		// The commands that end with the problem line (not path or open).
		"'changes', 'settle', 'wait' and the\n    hooks end with \"report.md has N problem(s)",
		// Notices.
		"not for ordinary questions", "a new at alone does not show a hidden Notice",
		// Settling.
		"'settle 9 fixed 5 bugs' settles 9", "two states for one thought are an error",
		"doing 186 (12m):",
		// Answers and links.
		"One answer Item may cover several thoughts", "drop an\n    answer the person replied to",
		"never make a section of links",
		// Lines of 'changes' every session sees.
		`'on fold "TITLE"' on a Fold without`, "unhide: they want to see it",
	} {
		if !strings.Contains(usage, w) {
			t.Errorf("-h lacks %q", w)
		}
	}
	if strings.Contains(usage, "Every other command") {
		t.Error("-h says every other command ends with the problem line (path and open do not)")
	}
	// Every topic in the list says when to read it.
	for _, tp := range topics {
		if !strings.Contains(tp.summary, "read when") {
			t.Errorf("topic %s does not say when to read it: %q", tp.name, tp.summary)
		}
	}
}
