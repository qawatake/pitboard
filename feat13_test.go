package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExeWatchNoticesChange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pitboard")
	os.WriteFile(p, []byte("old"), 0o755)
	w := watchFile(p)
	w.check()
	if w.stale() {
		t.Fatal("stale before any change")
	}
	// Same size, later mtime.
	os.WriteFile(p, []byte("new"), 0o755)
	os.Chtimes(p, time.Now(), time.Now().Add(time.Hour))
	w.check()
	if !w.stale() {
		t.Fatal("a changed mtime was not noticed")
	}
	// Size alone.
	os.WriteFile(p, []byte("old"), 0o755)
	w = watchFile(p)
	fi, _ := os.Stat(p)
	os.WriteFile(p, []byte("longer"), 0o755)
	os.Chtimes(p, fi.ModTime(), fi.ModTime())
	w.check()
	if !w.stale() {
		t.Fatal("a changed size was not noticed")
	}
}

func TestExeWatchWaitsWhileMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pitboard")
	os.WriteFile(p, []byte("old"), 0o755)
	w := watchFile(p)
	fi, _ := os.Stat(p)
	os.Remove(p)
	w.check()
	if w.stale() {
		t.Fatal("stale while the file is only missing")
	}
	// Put back as it was: nothing to say.
	os.WriteFile(p, []byte("old"), 0o755)
	os.Chtimes(p, fi.ModTime(), fi.ModTime())
	w.check()
	if w.stale() {
		t.Fatal("stale after the same file came back")
	}
	// Removed and a different one put back.
	os.Remove(p)
	w.check()
	os.WriteFile(p, []byte("new build"), 0o755)
	w.check()
	if !w.stale() {
		t.Fatal("a replaced file was not noticed")
	}
	// It stays, even if the file goes away again.
	os.Remove(p)
	w.check()
	if !w.stale() {
		t.Fatal("the notice went away")
	}
	// An unknown executable never says anything.
	var none *exeWatch
	none.check()
	if none.stale() || watchFile(filepath.Join(t.TempDir(), "x")).stale() {
		t.Fatal("stale without a file")
	}
}

func TestNewBuildLineOnTop(t *testing.T) {
	st := testStore(t)
	report := "<Title>題</Title>\n\n<Section title=\"s\">\n"
	for i := range 60 {
		report += "<Item id=\"i" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "\">項目</Item>\n"
	}
	os.WriteFile(st.reportPath(), []byte(report+"</Section>\n"), 0o644)
	a := testApp(t, st)
	lines, _ := screen(t, a)
	for _, l := range lines {
		if strings.Contains(l, msgNewBuild) {
			t.Fatal("shown without a change")
		}
	}
	a.exe = &exeWatch{path: "x", changed: true}
	a.top().readErr = "報告を読めませんでした"
	for _, scroll := range []bool{false, true} {
		if scroll {
			for range 50 {
				a.handle(key{r: 'j'})
			}
		}
		lines, raw := screen(t, a)
		if len(lines) != a.t.h {
			t.Fatalf("%d lines on a %d-line screen", len(lines), a.t.h)
		}
		if !strings.Contains(lines[0], msgNewBuild) {
			t.Fatalf("scroll %v: top line %q", scroll, lines[0])
		}
		if first := strings.SplitN(raw, "\r\n", 2)[0]; !strings.Contains(first, yellow) {
			t.Error("not yellow")
		}
		if !scroll {
			if i := strings.Index(strings.Join(lines, "\n"), "報告を読めませんでした"); i < 0 {
				t.Error("the read error is gone")
			}
		}
	}
	if a.t.h != 30 {
		t.Fatalf("height left at %d", a.t.h)
	}
}
