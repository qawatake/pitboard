package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Two marks keep the person from losing track of what wants them on a long
// report, in the gutter left of a line's mark (where the cursor's > goes;
// the > wins):
//
//	● yellow  waiting: an Ask not answered yet, or an Item / Link with
//	          ask="y" without a y / n on what it says now
//	● green   new: the person has not looked at what the line says now
//
// "Looked at" is the cursor resting on the line for settleDelay, or reacting
// to it, opening it, hiding it. Being on screen is not enough. What a line
// says is its viewKey; the keys looked at are kept, hashed, in
// .pitboard/<session>/viewed (one per line, never sent to the AI).

func (s *Store) viewedPath() string { return filepath.Join(s.dir, "viewed") }

// Viewed is the hashes in the viewed file. A file that cannot be read is
// empty, and a line that is not a hash is skipped.
func (s *Store) Viewed() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(s.viewedPath())
	if err != nil {
		return out
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); isHash(l) {
			out[l] = true
		}
	}
	return out
}

// AddViewed appends hashes to the viewed file.
func (s *Store) AddViewed(hs []string) error {
	if len(hs) == 0 {
		return nil
	}
	f, err := os.OpenFile(s.viewedPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(hs, "\n") + "\n")
	return err
}

func isHash(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func hashKey(k string) string {
	h := sha256.Sum256([]byte(k))
	return hex.EncodeToString(h[:16])
}

// viewKey is what a line says, for "new": what hiding goes by (hideKey),
// plus what opening it shows (its note and contents, an Ask's options) and a
// Link's target. "" for lines that are never new (Fold, text, headings).
func viewKey(n *node) string {
	if n.kind == kDetails {
		return ""
	}
	k := hideKey(n)
	if k == "" {
		return ""
	}
	if n.kind == kPlain {
		return k
	}
	var b strings.Builder
	b.WriteString(k)
	b.WriteString("|note=" + n.reason)
	b.WriteString("|" + n.path + n.url)
	var body func(ns []*node)
	body = func(ns []*node) {
		for _, c := range ns {
			b.WriteString("|" + c.label + ":" + c.text + strings.Join(c.code, "\n"))
		}
	}
	body(n.body)
	if n.kind == kQuestion {
		body(n.kids)
	}
	return b.String()
}

// waiting says whether n wants the person's reaction and has none on what
// it says now.
func (a *app) waiting(n *node) bool {
	if n.id == "" {
		return false
	}
	e, ok := a.state.react[n.id]
	answered := ok && e.Line == n.sig()
	switch {
	case n.kind == kQuestion:
		return !answered
	case n.ask == "y" && (n.kind == kTodo || n.kind == kDoing || n.kind == kLink):
		return !answered || (e.Value != "y" && e.Value != "n")
	}
	return false
}

// isNew says whether the person has not looked at what n says now.
func (a *app) isNew(n *node) bool {
	k := viewKey(n)
	return k != "" && a.st != nil && !a.viewed[hashKey(k)]
}

// markViewed records that the person looked at n.
func (a *app) markViewed(ns ...*node) {
	if !a.canWrite() {
		return
	}
	var hs []string
	for _, n := range ns {
		if n == nil {
			continue
		}
		k := viewKey(n)
		if k == "" {
			continue
		}
		h := hashKey(k)
		if !a.viewed[h] {
			a.viewed[h] = true
			hs = append(hs, h)
		}
	}
	if err := a.st.AddViewed(hs); err == nil {
		a.viewedMod = modTime(a.st.viewedPath())
	}
}

const (
	markNone = iota
	markNew
	markWaiting
)

// markOfLine is the mark n gets in the gutter. A Fold gets the strongest
// mark of what it holds, open or not. Hidden lines get none.
func (a *app) markOfLine(pv *view, n *node) int {
	if pv.isHidden(n) {
		return markNone
	}
	if n.kind == kDetails {
		m := markNone
		var rec func(ns []*node)
		rec = func(ns []*node) {
			for _, c := range ns {
				if m == markWaiting || pv.isHidden(c) {
					continue
				}
				m = max(m, a.markOfLine(pv, c))
				if c.kind == kSection {
					rec(c.kids)
				}
			}
		}
		rec(n.kids)
		return m
	}
	switch {
	case a.waiting(n):
		return markWaiting
	case a.isNew(n):
		return markNew
	}
	return markNone
}

// gutterMark puts the mark in the gutter of a drawn line that is not the
// cursor's.
func gutterMark(line string, m int) string {
	if m == markNone || strings.HasPrefix(line, ">") || !strings.HasPrefix(line, strings.Repeat(" ", margin)) {
		return line
	}
	color := green
	if m == markWaiting {
		color = yellow
	}
	return plainText("●", color).String() + line[1:]
}

// target is a line marked in the report, in the order of the page, with
// the Folds around it.
type marked struct {
	n     *node
	m     int
	folds []*node
}

// allLines is every line the cursor can stop at once Folds are open, in the
// order of the page, with its mark: what space walks through. Hidden lines
// are left out (unless H shows them), and so is what is inside an Ask.
func (a *app) allLines(pv *view) []marked {
	var out []marked
	var rec func(ns []*node, folds []*node)
	rec = func(ns []*node, folds []*node) {
		for _, n := range ns {
			if !pv.showHidden && (pv.isHidden(n) || (n.kind == kSection && pv.allHidden(n.kids))) {
				continue
			}
			if pv.navigable(n) && n.key != "" {
				m := markNone
				if n.kind != kDetails {
					m = a.markOfLine(pv, n)
				}
				out = append(out, marked{n: n, m: m, folds: folds})
			}
			switch n.kind {
			case kSection:
				rec(n.kids, folds)
			case kDetails:
				rec(n.kids, append(append([]*node{}, folds...), n))
			}
		}
	}
	rec(pv.nodes, nil)
	return out
}

// counts is how many lines are waiting, and how many are new (and not
// waiting), in the whole report.
func (a *app) counts(pv *view) (waiting, fresh int) {
	if pv == nil || a.st == nil {
		return 0, 0
	}
	for _, l := range a.allLines(pv) {
		switch l.m {
		case markWaiting:
			waiting++
		case markNew:
			fresh++
		}
	}
	return
}

// jump moves the cursor to the next waiting line, or if none, the next new
// one: downwards from the cursor, then from the top. Folds on the way are
// opened. The person moved the cursor, but the line still waits settleDelay
// before x y n and options work on it.
func (a *app) jump(pv *view) {
	ls := a.allLines(pv)
	at := -1
	for i, l := range ls {
		if l.n.key == pv.cur {
			at = i
		}
	}
	for _, want := range []int{markWaiting, markNew} {
		for d := 1; d <= len(ls); d++ {
			l := ls[(at+d+len(ls))%len(ls)]
			if l.m != want {
				continue
			}
			for _, f := range l.folds {
				pv.open[f.key] = true
			}
			pv.cur, pv.follow, pv.lock, pv.placed = l.n.key, true, false, false
			pv.auto, pv.autoAt = l.n.key, time.Time{}
			return
		}
	}
	a.msg = "反応待ちも新着もありません"
}

// viewAll marks every line now in the report as looked at (A).
func (a *app) viewAll(pv *view) {
	if !a.canWrite() {
		return
	}
	var ns []*node
	walkAll(pv.nodes, func(n *node) { ns = append(ns, n) })
	a.markViewed(ns...)
	a.msg = "いまある行をすべて見たことにしました"
}

// dwell marks the cursor's line as looked at once it has rested there for
// settleDelay (in the frames before; a line merely on screen is not).
func (a *app) dwell(pv *view) {
	n := pv.nodeAt(pv.cur)
	if n == nil || !pv.curShown || pv.placed || pv.lock || a.inPane || a.list || a.help {
		pv.dwellKey = ""
		return
	}
	k := pv.cur + "\x00" + viewKey(n)
	switch {
	case pv.dwellKey != k:
		pv.dwellKey, pv.dwellAt = k, clock()
	case clock().Sub(pv.dwellAt) >= settleDelay && a.isNew(n):
		a.markViewed(n)
	}
}
