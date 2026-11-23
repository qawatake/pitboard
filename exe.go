package main

import (
	"os"
	"path/filepath"
	"time"
)

const msgNewBuild = "新しい版があります。q で終えて、開き直してください。"

// exeWatch notices when the viewer's own executable is replaced, so the
// person learns to reopen it. The viewer never restarts itself: that would
// lose what is being written.
type exeWatch struct {
	path    string // empty: unknown, never checked
	mtime   time.Time
	size    int64
	changed bool // once set, it stays
}

// newExeWatch remembers the running executable (symlinks resolved).
// Whatever fails leaves a watch that never says anything.
func newExeWatch() *exeWatch {
	p, err := os.Executable()
	if err != nil {
		return &exeWatch{}
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return watchFile(p)
}

func watchFile(p string) *exeWatch {
	fi, err := os.Stat(p)
	if err != nil {
		return &exeWatch{}
	}
	return &exeWatch{path: p, mtime: fi.ModTime(), size: fi.Size()}
}

// check looks at the file again. A file that is not there (being replaced)
// is compared the next time it is seen.
func (w *exeWatch) check() {
	if w == nil || w.path == "" || w.changed {
		return
	}
	fi, err := os.Stat(w.path)
	if err != nil {
		return
	}
	if !fi.ModTime().Equal(w.mtime) || fi.Size() != w.size {
		w.changed = true
	}
}

func (w *exeWatch) stale() bool { return w != nil && w.changed }
