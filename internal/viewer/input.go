package viewer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// ---- the editor ----

func (a *app) handleInput(k key) {
	e, w := &a.in.ed, a.editWidth()
	switch k.name {
	case "esc":
		a.in = nil
	case "left", "ctrl+b":
		e.Left()
	case "right", "ctrl+f":
		e.Right()
	case "up", "ctrl+p":
		e.Up(w)
	case "down", "ctrl+n":
		e.Down(w)
	case "home", "ctrl+a":
		e.Home()
	case "end", "ctrl+e":
		e.End()
	case "alt+left", "alt+b":
		e.WordLeft()
	case "alt+right", "alt+f":
		e.WordRight()
	case "backspace":
		e.Backspace()
	case "delete", "ctrl+d":
		e.Delete()
	case "ctrl+w", "alt+backspace":
		e.KillWordLeft()
	case "ctrl+k":
		e.KillToEnd()
	case "ctrl+u":
		e.KillToStart()
	case "pgup", "pgdn":
		// The report above still scrolls while writing.
		pv, d := a.top(), max(a.t.h/3, 1)
		if k.name == "pgup" {
			pv.top = max(pv.top-d, 0)
		} else {
			pv.top += d
		}
	case "ctrl+g":
		a.external()
	case "newline":
		e.Insert('\n')
	case "enter":
		a.write(e.String())
	case "ctrl+s":
		// Nothing: sending is s, once written.
	case "ctrl+v":
		a.pasteImage()
	case "paste-start":
		a.in.paste = e.pos
	case "paste-end":
		if start := a.in.paste; start >= 0 {
			a.in.paste = -1
			a.pastedImage(start)
		}
	case "":
		if k.r != 0 {
			e.Insert(k.r)
		}
	}
}

// editWidth is the width the editor's text is laid out in.
func (a *app) editWidth() int {
	if a.in != nil {
		return max(a.textWidth()-markW-a.in.ind, 8)
	}
	return a.textWidth() - markW
}

// write records what is in the editor as a thought and closes the editor
// (empty text closes it without recording). It reports false when the
// thought could not be recorded; the editor then stays open.
func (a *app) write(text string) bool {
	var err error
	if n := a.in.edit; n != 0 {
		_, err = a.b.Edit(n, text)
	} else {
		_, err = a.b.Comment(a.in.id, a.in.title, text)
	}
	if err != nil {
		a.msg = err.Error()
		return false
	}
	a.in = nil // empty text closes it too (to take a thought back, x)
	return true
}

// thoughtN is the thought numbered n on the board, or nil.
func (a *app) thoughtN(n int) *board.Thought { return a.b.Thought(n) }

// rewrite opens the editor on thought n's text (e in the pane or the list).
func (a *app) rewrite(n int) {
	t := a.thoughtN(n)
	if t == nil || !a.canWrite() {
		return
	}
	a.in = &input{edit: n, label: fmt.Sprintf("コメント %d を直す", n), ed: newEdit(strings.TrimSpace(t.Text)), paste: -1}
}

const scissors = "# ------------------------ >8 ------------------------"

func editorCommand() []string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if f := strings.Fields(os.Getenv(v)); len(f) > 0 {
			return f
		}
	}
	return []string{"vim"}
}

// external continues the text in the person's editor. What is above the
// scissors line is sent; leaving it empty sends nothing.
func (a *app) external() {
	f, err := os.CreateTemp("", "pitboard-*.md")
	if err != nil {
		a.msg = err.Error()
		return
	}
	defer os.Remove(f.Name())
	fmt.Fprintf(f, "%s\n\n%s\n# %s: この線より上を pitboard に置きます。空のまま閉じると何もしません。\n", a.in.ed.String(), scissors, a.in.label)
	f.Close()
	cmd := editorCommand()
	if err := a.t.run(exec.Command(cmd[0], append(cmd[1:], f.Name())...)); err != nil {
		a.msg = cmd[0] + ": " + err.Error()
		return
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		a.msg = err.Error()
		return
	}
	text, _, _ := strings.Cut(string(b), scissors)
	if strings.TrimSpace(text) == "" {
		a.msg = "空だったので書きませんでした"
		a.in = nil
		return
	}
	a.write(text)
}

// editorLines is the editor and its hints, as drawEditing puts them among
// the lines: a label saying what is being written (when it shows one), the
// text, growing up to a third of the screen, and the hints. pad puts a
// blank line above it and under the hints (left out when the screen is too
// low for them).
func (a *app) editorLines(pad bool) []string {
	e, w := &a.in.ed, a.editWidth()
	rs := e.rows(w)
	crow, ccol := e.Cursor(w)
	// Taller than the editor: scrolled to keep the cursor's row in view.
	limit := max(a.t.h/3, 3)
	top := &a.in.etop
	if crow < *top {
		*top = crow
	}
	if crow >= *top+limit {
		*top = crow - limit + 1
	}
	*top = max(min(*top, len(rs)-limit), 0)
	hints := a.editorHints()
	if a.msg != "" {
		// What a key in the editor did (ctrl+v that found no image...):
		// in place of the hints until the next key.
		hints = a.msgRows(a.msg, nil)
	}
	// If the line being written about changed or went away meanwhile, say so
	// (sending still works: what was written is never lost).
	var changed text
	if a.in.id != "" {
		if n := report.ByID(a.rep.Nodes, a.in.id); n == nil || n.Title() != a.in.title {
			changed = plainText("この行は書き換わりました", magenta)
		}
	}
	ind := a.in.ind
	// Between two rules, like a prompt: the label (if shown) in the upper
	// one, and ❯ before the text.
	var label text
	if a.in.showLabel && a.in.label != "" {
		label = plainText(a.in.label, cyan+bold)
	}
	var out []string
	if pad {
		out = append(out, "")
	}
	out = append(out, a.rule(label, changed))
	for i := *top; i < len(rs) && i < *top+limit; i++ {
		if i == crow {
			// The input position, past the gutter and the mark field (the
			// terminal's cursor is put there).
			a.inAt = [2]int{len(out), lgutter + ind + markW + ccol}
		}
		var mark text
		if i == 0 {
			mark = plainText("❯", "")
		}
		l := withImages(string(e.text[rs[i].start:rs[i].end]), "")
		out = append(out, a.compose(false, ind, mark, l, nil))
	}
	out = append(out, a.rule(nil, nil))
	out = append(out, hints...)
	if pad {
		out = append(out, "")
	}
	return out
}

// rule is a faint line across the text's width, around the editor: with
// label in it near the left end, and right near the right end.
func (a *app) rule(label, right text) string {
	w := a.textWidth()
	if len(right) > 0 {
		right = right.trunc(max(w/2, 4))
	}
	var left, tail text
	if len(label) > 0 {
		left = append(plainText("── ", dim), label.trunc(max(w-right.width()-10, 4))...)
		left = append(left, plainText(" ", "")...)
	}
	if len(right) > 0 {
		tail = append(append(plainText(" ", ""), right...), plainText(" ──", dim)...)
	}
	line := append(left, plainText(strings.Repeat("─", max(w-left.width()-tail.width(), 0)), dim)...)
	return strings.Repeat(" ", lgutter) + append(line, tail...).String()
}

// editorAt says where the editor goes among rows: at (the row it is put
// before), head (the first row of what it is about, kept in view when it
// fits), and ind (the indentation of that line's title).
func (a *app) editorAt(rows []row, ends map[string]int, key string) (at, head, ind int) {
	at, head = -1, -1
	if e, ok := ends[key]; ok {
		at = e
		for i, r := range rows {
			if r.key == key {
				head = i
				break
			}
		}
	}
	if head < 0 {
		head = at
	}
	return at, head, ind
}

// drawEditing draws the screen while the editor is open. The editor and
// its hints go among the lines, right under what they are about: the line
// a comment is written on (c), the cursor's line (m), the comment being
// rewritten (e, in the pane or the list). They are kept in the upper two
// thirds of the screen, scrolling what is above if need be, without moving
// the page's own scroll position: closing the editor puts back the screen
// as it was. The last line stays empty.
func (a *app) drawEditing() {
	// The notices pinned above the page; a comment on one (or m with the
	// cursor on one) is written right under it, among them.
	var pins pinning
	key := a.in.id
	if n := report.ByID(a.rep.Nodes, key); n != nil && n.Key != "" {
		key = n.Key // a Fold without an id: its comments go by its title
	}
	if key == "" {
		key = a.in.at
	}
	pinned := false
	if !a.list {
		pv := a.top()
		pv.settleCursor()
		must := ""
		if !a.inPane && a.in.edit == 0 {
			must = key
		}
		pins = a.pin(pv, a.t.h, must)
		_, pinned = pins.ends[key]
		pinned = pinned && key != "" && must != ""
	}
	// The counts at the top right, on a line of their own (see draw).
	mark := a.sendMark()
	head := 0
	if len(mark) > 0 {
		head = 1
	}
	var banner []string
	bannerAt := -1 // where the editor's input starts among the banner lines
	if pinned {
		a.in.ind, a.in.showLabel = 0, a.in.id == ""
		e := pins.ends[key]
		for _, r := range pins.rows[:e] {
			banner = append(banner, r.line)
		}
		rest := pins.rows[e:]
		block := a.editorLines(true)
		if len(rest) > 0 && rest[0].line == "" {
			block = block[:len(block)-1] // the blank line under the notices is its own
		}
		if len(pins.rows)+len(block)+head+2 > a.t.h {
			// Too low a screen: the blank lines go first, so that the
			// input and the hints stay.
			block = a.editorLines(false)
		}
		bannerAt = len(banner)
		banner = append(banner, block...)
		for _, r := range rest {
			banner = append(banner, r.line)
		}
	} else {
		for _, r := range pins.rows {
			if len(banner) < max(a.t.h-6, 0) {
				banner = append(banner, r.line)
			}
		}
	}
	h := max(a.t.h-len(banner)-head-1, 1)
	var rows []row
	top, at, headRow, ind := 0, -1, -1, 0
	switch {
	case pinned:
		pv := a.top()
		rows, top = a.rows(pv), pv.top
	case a.list:
		rows, top = a.listRows(), a.ltop
		n := a.in.edit
		if n == 0 {
			n = a.lcur
		}
		if e, ok := a.lends[n]; ok {
			at = e
			for i, r := range rows {
				if r.isCur {
					headRow = i
				}
			}
		}
	case a.inPane:
		// The pane, in full, under the page.
		pv := a.top()
		rows, top = a.rows(pv), pv.top
		rows = append(rows, row{}, row{line: a.compose(false, 0, nil, plainText("コメント", bold+cStructure), nil)})
		prows, ends := a.paneAll(a.paneOrder())
		n := a.in.edit
		if n == 0 {
			n = a.pcur
		}
		if e, ok := ends[n]; ok {
			at = len(rows) + e
			for i, r := range prows {
				if r.key == fmt.Sprint(n) {
					headRow = len(rows) + i
				}
			}
		}
		rows = append(rows, prows...)
	default:
		pv := a.top()
		rows, top = a.rows(pv), pv.top
		if n := pv.nodeAt(key); n != nil {
			ind = indentOf(n)
		}
		at, headRow, _ = a.editorAt(rows, pv.ends, key)
	}
	var block []string
	if !pinned {
		if at < 0 {
			// Nothing to go under: halfway down what is on screen.
			at = min(len(rows), top+h/2)
		}
		if headRow < 0 || headRow > at {
			headRow = at
		}
		a.in.ind = ind
		// c needs no label (what it is about is right above it), unless
		// that line is too tall to be seen with it.
		a.in.showLabel = a.in.id == "" || a.in.edit != 0
		limit := max(h*2/3, 1)
		place := func() {
			block = a.editorLines(true)
			if len(block) > limit {
				// Too low a screen: the blank lines go first.
				block = a.editorLines(false)
			}
			end := at + len(block)
			if end > top+limit {
				top = end - limit
			}
			if headRow < top && end-headRow <= limit {
				top = headRow
			}
			if at < top {
				top = at
			}
			top = max(top, 0)
		}
		place()
		if !a.in.showLabel && headRow < top {
			a.in.showLabel = true
			place()
		}
	} else {
		at = len(rows)
		top = max(min(top, len(rows)-h), 0)
	}
	all := make([]string, 0, len(rows)+len(block))
	for _, r := range rows[:at] {
		all = append(all, r.line)
	}
	all = append(all, block...)
	for _, r := range rows[at:] {
		all = append(all, r.line)
	}
	// The page starts with a blank line; under the counts' line it is not
	// needed.
	skip := 0
	if head > 0 && top == 0 && len(all) > 0 && all[0] == "" && at > 0 {
		skip = 1
	}
	lines := make([]string, 0, h+head)
	if head > 0 {
		lines = append(lines, a.overlayRight("", mark))
	}
	for i := top + skip; i < len(all) && len(lines) < h+head; i++ {
		lines = append(lines, all[i])
	}
	for len(lines) < h+head {
		lines = append(lines, "")
	}
	var cur *[2]int
	// The terminal's own cursor goes where the person writes, so an input
	// method shows what is being composed there.
	if pinned {
		cur = &[2]int{bannerAt + a.inAt[0] + 1, a.inAt[1] + 1}
	} else if r := at + a.inAt[0] - top - skip; r >= 0 && r < h {
		cur = &[2]int{len(banner) + head + r + 1, a.inAt[1] + 1}
	}
	a.t.draw(append(append(banner, lines...), ""), cur)
}

// editorHints is the editor's one line of hints. When narrow, they are
// left out from the end: ctrl+g, then the word of ctrl+j (its key stays:
// how to write a new line is not found otherwise), then ctrl+j; how to
// write, send and stop always stay.
func (a *app) editorHints() []string {
	nl := "ctrl+j"
	if a.kbd {
		nl = "shift+enter" // the terminal tells it from enter
	}
	var all []hint
	for _, h := range page.EditorHints(nl) {
		x := hint{k: h.Key, d: h.Word}
		if h.Do == "newline" {
			x.rank, x.short = 3, h.Key
		}
		all = append(all, x)
	}
	all = append(all, hint{k: "ctrl+g", d: "エディタ", rank: 1})
	var ks []hint
	for lvl := 0; lvl <= 3; lvl++ {
		ks = ks[:0]
		for _, k := range all {
			if k.rank == 0 || k.rank > lvl {
				if lvl >= 2 && k.short != "" {
					k.k, k.d = k.short, ""
				}
				ks = append(ks, k)
			}
		}
		if keyHints(ks).width() <= a.room(a.in.ind, nil) {
			break
		}
	}
	return []string{a.compose(false, a.in.ind, nil, keyHints(ks), nil)}
}
