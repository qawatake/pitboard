package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// keysScript runs script (an ES module) with static/keys.js and state.js
// beside it, under node; the test is skipped without node.
func keysScript(t *testing.T, script string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	dir := t.TempDir()
	for _, f := range []string{"keys.js", "state.js"} {
		b, err := staticFS.ReadFile("static/" + f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "test.js"), []byte(script), 0o644)
	out, err := exec.Command(node, filepath.Join(dir, "test.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// l opens the comment panel and closes it again (the terminal's l goes to
// its comments screen and back); while the page is not connected it does
// nothing.
func TestLTogglesThePanel(t *testing.T) {
	out := keysScript(t, `
import { intentOf } from './keys.js';
const page = { Things: [], Comments: [] };
const ui = (o) => ({ mode: 'normal', panel: false, cur: '', conn: 'live', ...o });
const ev = { key: 'l', target: {} };
const of = (u) => JSON.stringify(intentOf(ev, u, page));
console.log([
  of(ui({})),
  of(ui({ panel: true, mode: 'comments', cur: '#l:1' })),
  of(ui({ conn: 'gone' })),
  JSON.stringify(intentOf({ key: 'l', target: { tagName: 'TEXTAREA' } }, ui({ mode: 'editor' }), page)),
].join('\n'));
`)
	want := strings.Join([]string{`{"do":"openPanel"}`, `{"do":"closePanel"}`, `{"do":"none"}`, `null`}, "\n")
	if out != want {
		t.Errorf("intents:\n%s\nwant:\n%s", out, want)
	}
}
