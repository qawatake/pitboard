package viewer

import (
	"fmt"
	"github.com/qawatake/pitboard/internal/page"
	"os"
	"path/filepath"
	"strings"
)

// bottomLines is everything under the scrolling part: the editor while
// writing, or the key hints. n is how many rows the
// scrolling part has (the list of keys scrolls only if it does not fit).
func (a *app) bottomLines(n int) []string {
	switch {
	case a.help:
		hs := []hint{{k: "q", d: "閉じる"}}
		if n > a.t.h-1 {
			hs = []hint{{k: "j k ↑↓", d: "スクロール"}, {k: "q", d: "閉じる"}}
		}
		return a.packKeys(hs, nil, false)
	case a.in != nil:
		return a.editorLines(true)
	}
	if a.jumping && !a.onComments() {
		return a.jumpLines(a.jumpTo)
	}
	return a.bottom()
}

// msgRows is a message on the bottom lines: on one line with right (the
// count of hidden lines) when it fits, else wrapped, never cut. A message
// in two parts (what happened, and why with how to fix it) is split at
// "\n" when it does not fit on one line.
func (a *app) msgRows(msg string, right text) []string {
	// A refusal (why a key did nothing) needs the person; any other
	// message is the normal color.
	st := ""
	for _, r := range page.Refusals {
		if strings.TrimSpace(msg) == r {
			st = cNeed
		}
	}
	one := func(s string) text { return plainText(strings.Join(strings.Fields(s), " "), st) }
	t := one(msg)
	if t.width() <= a.room(0, right) {
		return []string{a.compose(false, 0, nil, t, right)}
	}
	var out []string
	for _, part := range strings.Split(msg, "\n") {
		for _, l := range one(part).wrap(a.room(0, nil)) {
			out = append(out, a.compose(false, 0, nil, l, nil))
		}
	}
	// The report gives way, down to one line of it.
	return out[:min(len(out), max(a.t.h-1, 1))]
}

// helpRows is the list of every key.
func (a *app) helpRows() []row {
	rows := []row{{}, {line: a.compose(false, 0, nil, plainText("キー", bold+cStructure), nil)}, {}}
	for _, k := range [][2]string{
		{"j k", "上下の行へ動く (ctrl+n / ctrl+p も)。止まる行がその先になければ半画面スクロール。カーソルの行が画面の外にあれば、まずそこまで戻す"},
		{"ctrl+d u", "画面だけを半画面ずつ下 / 上へ (カーソルは動かない。pgdn pgup・alt+↓ ↑ も)"},
		{"ctrl+e y", "画面だけを 1 行ずつ下 / 上へ (↓ ↑ も。マウスのホイールやトラックパッドも、端末が ↓ ↑ に変えて送る)"},
		{"", "報告のいちばん上からさらに k で、上に固定されたお知らせへ"},
		{"", "日本語入力のままでも、ctrl+n ctrl+p と全角の英字 (ｊ ｙ など) が効く"},
		{"g", "節へ飛ぶ: お知らせ・節・いちばん外の折りたたみに a b c … が付く。その字で、そこの最初の行へ (見出しを画面の上に)。esc でやめる"},
		{"g 1 … 9", "カーソルの行の中のリンク (別の行を指すもの) を、出てくる順に 1 から。その数字で、指す行へ (折りたたみは開く)"},
		{"g -", "戻る: g で飛ぶ前の行へ (飛ぶたびに覚え、新しい順に戻る)"},
		{"", "開いた中身が画面より高ければ、j k は次 (前) の行へ移る前に、間を半画面ずつスクロールする (カーソルは動かず、画面の外に出ることもある。そのとき x r c と答えのキーは、まずカーソルの行を画面に戻す)。k は j で読み下った中身を同じように読み上る"},
		{"space", "カーソルより下で、いちばん近い印のある行 (黄の ●・人待ちの項目・緑の ●) へ。下に無ければ上から。折りたたみは開く"},
		{"r", "読んだ: カーソルの行の緑の ● (右上の「未読」) を消す (カーソルを当てる・開くだけでは消えない。答え・コメント・x でも消える)"},
		{"A", "いま報告にある行をすべて読んだことにする (緑の ● を消す)"},
		{"enter", "開く・閉じる (コメントの画面では、そのコメントの行へ)"},
		{"u", "元に戻す: 直前の操作を取り消す (答え・コメントを置く / 直す / 取り消す・隠す / 表示に戻す・r・A。新しい順に 1 つずつ。送ったあとなら、取り消しを次に送る)"},
		{"a b …", "問いに答える (選択肢のキー。問いを開いているときだけ。閉じていれば開く)"},
		{"c", "この行にコメントを書く"},
		{"m", "宛先なしでコメントを書く (どこにカーソルがあっても)"},
		{"s", "AI に送る: 未送信のものをまとめて送る (AI が pitboard wait で待っていればそれが起こす。いなければ AI の端末に 1 行打ち込んで起こす)"},
		{"x", "この行を隠す (隠した行の上では表示に戻す)"},
		{"H", "隠した行を表示する / しない"},
		{"l", "コメントの画面 / 報告へ (押すたびに切り替わる。tab も)。報告のカーソルと開閉はそのまま"},
		{"o (コメント)", "コメントの画面で、そのコメントを開く・閉じる (全文と AI の対応)"},
		{"x (コメント)", "コメントの画面で、そのコメントを取り消す"},
		{"e (コメント)", "コメントの画面で、そのコメントを直す (enter で置き換える。esc でやめる)"},
		{"?", "このキーの一覧"},
		{"q", "終了 (コメントの画面からは報告へ。ctrl+c はどこでも終了)"},
		{"", ""},
		{"書いているとき", ""},
		{"enter", "pitboard に置く (s で送るまで AI には届かない)"},
		{"shift+enter", "改行 (対応していない端末では ctrl+j。alt+enter も)"},
		{"← → ctrl+b f", "1 字左 / 右"},
		{"↑ ↓ ctrl+p n", "上 / 下の行へ (折り返した見た目の行で)"},
		{"ctrl+a e", "行頭 / 行末 (home / end も)"},
		{"alt+b f", "語単位で左 / 右 (alt+← → も)"},
		{"backspace", "左の 1 字を消す (ctrl+h も)"},
		{"ctrl+d", "右の 1 字を消す (delete も)"},
		{"ctrl+w", "左の 1 語を消す (alt+backspace も)"},
		{"ctrl+k / u", "カーソルから行末まで / 行頭からカーソルまで消す"},
		{"pgup pgdn", "書きながら画面を半画面ずつ上 / 下へ (alt+↑ ↓ も。1 行目の ↑・最後の行の ↓ は 1 行ずつ。入力欄は報告の中で一緒に動き、字を打つと戻る)"},
		{"ctrl+v", "クリップボードの画像を貼る ([Image #N] が入る。画像ファイルのパスを貼っても同じ)"},
		{"ctrl+g", "$VISUAL / $EDITOR / vim で続きを書く"},
		{"esc", "やめる"},
	} {
		if k[0] == "" && k[1] == "" {
			rows = append(rows, row{})
			continue
		}
		// The description wraps in its own column, to the right of the key.
		const col = 14
		if a.textWidth()-markW-col < 16 {
			// Too narrow for a column of descriptions: the key on its own
			// line, the description under it.
			if k[0] != "" {
				rows = append(rows, a.para(0, plainText(k[0], bold))...)
			}
			if k[1] != "" {
				rows = append(rows, a.para(2, plainText(k[1], dim))...)
			}
			continue
		}
		label := plainText(k[0], bold)
		label = append(label, plainText(strings.Repeat(" ", max(col-label.width(), 1)), "")...)
		for i, l := range plainText(k[1], dim).wrap(a.room(col, nil)) {
			if i == 0 {
				rows = append(rows, row{line: a.compose(false, 0, nil, append(label, l...), nil)})
			} else {
				rows = append(rows, row{line: a.compose(false, col, nil, l, nil)})
			}
		}
	}
	return rows
}

// hint is one key in the hints at the bottom. A hint with a short form is
// never left out: when the line is narrow, the short form is shown instead.
type hint struct {
	k, d  string
	alt   string // a shorter label tried before hints are left out
	short string // the short form: these keys alone, without the label
	// shortK is a shorter way to write the key itself (S-enter), tried
	// last.
	shortK string
	// shortD is a word the short form keeps while it fits (then it goes
	// too).
	shortD string
	rank   int // when it is left out on a narrow line (see rankX)
}

func keyHints(hs []hint) text {
	var t text
	for i, h := range hs {
		if i > 0 {
			t = append(t, plainText("  ", "")...)
		}
		t = append(t, plainText(h.k, bold)...)
		if h.d != "" {
			t = append(t, plainText(" "+h.d, dim)...)
		}
	}
	return t
}

// bottom is the bottom line(s): a message, or the keys that work on this
// line, and on the right how many lines are hidden.
func (a *app) bottom() []string {
	var right text
	switch {
	case a.st == nil || a.b.NoSession:
		// Waiting for a board: say where the viewer is looking.
		// A long path keeps its end (the directory's own name).
		p, room := homeTilde(a.base), a.textWidth()-markW-len("? キー")-4
		if width(p) > room {
			p = "…" + tail(p, max(room-1, 4))
		}
		right = plainText(p, dim)
	case !a.onComments():
		right = a.hiddenCount()
	}
	say := func(msg string) []string { return a.msgRows(msg, right) }
	switch {
	case a.msg != "":
		return say(a.msg)
	case !a.onComments() && a.top().lock:
		return say(msgMoved)
	}
	var keys []hint
	if a.onComments() {
		if t := a.listCur(); t != nil && a.lcurShown {
			if a.canGo(t) {
				keys = append(keys, hint{k: "enter", d: page.WordGo, rank: rankEnter})
			}
			if a.lopen[t.N] {
				keys = append(keys, hint{k: "o", d: page.WordClose, rank: rankEnter})
			} else {
				keys = append(keys, hint{k: "o", d: "開く", rank: rankEnter})
			}
			keys = append(keys, commentHints(a.canWrite())...)
		}
		keys = append(keys, hint{k: "l", d: "報告へ"})
		return a.oneLine(keys, nil)
	}
	pv := a.top()
	n := pv.nodeAt(pv.cur)
	if n == nil || !pv.curShown {
		if pv.Err == "" && !pv.Missing {
			keys = append(keys, hint{k: "j k", d: "スクロール", rank: rankEnter})
		}
		// Where the cursor's line went (reacting brings it back first): it
		// outranks every key.
		var where text
		switch pv.offScreen() {
		case -1:
			where = plainText("↑ カーソルは上", dim)
		case 1:
			where = plainText("↓ カーソルは下", dim)
		}
		return a.oneLineKeep(keys, where, right)
	}
	// The keys for this line, in the order shown (page's Offer: never a
	// key its guard refuses); an open Ask leads with its options.
	for _, h := range a.b.Offer(n, pv.touch()).Hints {
		x := hint{k: h.Key, d: h.Word, rank: page.DropRank(h.Do)}
		switch h.Do {
		case "wait":
			// Narrow, it keeps a short word while it fits (j k alone does
			// not say what it is for).
			x.short, x.shortD = h.Key, wordWaitNarrow
		case "hide":
			if pv.placed {
				// Before the person moves, x is what works on the line (a
				// notice read and done with): kept over g, space and enter.
				x.rank = rankAnswer
			}
		}
		keys = append(keys, x)
	}
	// m q H work everywhere: the list of keys (?) tells them; m is also
	// shown, since writing works wherever the cursor is.
	return a.oneLine(keys, right)
}

// wordWaitNarrow is the word of "j k" (move first) on a narrow line, in
// place of page's longer one (the terminal's alone: a browser's line wraps).
const wordWaitNarrow = "動かす"

// commentHints is the keys on a comment (page's CommentHints): e to edit,
// x to take it back.
func commentHints(canWrite bool) []hint {
	var out []hint
	for _, h := range page.CommentHints(canWrite) {
		x := hint{k: h.Key, d: h.Word, rank: rankAnswer}
		if h.Do == "retract" {
			x.rank = rankX
		}
		out = append(out, x)
	}
	return out
}

func concat(hs ...[]hint) []hint {
	var out []hint
	for _, h := range hs {
		out = append(out, h...)
	}
	return out
}

// The order in which hints are left out when the line is narrow: lowest
// first (u, then x and space, g, enter, r and an answer, c). A hint with rank 0 (one with a short
// form, m, ?) is kept.
// They are page's (DropRank), the same on every screen.
const (
	rankU      = page.RankUndo
	rankX      = page.RankHide
	rankG      = page.RankJump
	rankEnter  = page.RankEnter
	rankAnswer = page.RankAnswer
	rankC      = page.RankComment
)

// oneLine is the hints: always one line, the keys for this line, then
// "m 書く" and "? キー". When the line is narrow, right goes first, then
// keys by rank (x, enter, r and an answer, c); "m 書く", "? キー" and a hint with a
// short form stay to the end.
func (a *app) oneLine(keys []hint, right text) []string {
	return a.oneLineKeep(keys, nil, right)
}

// oneLineKeep is oneLine with keep on the right (before right) outranking
// every key: the keys, "m 書く" and "? キー" included, are left out before
// it is.
func (a *app) oneLineKeep(keys []hint, keep, right text) []string {
	// The keys after the line's own, and the ones kept to the end, are
	// page's (Bar): the same on every screen.
	more, tailHints := a.b.Bar(a.top().showHidden)
	var tail []hint
	for _, h := range tailHints {
		x := hint{k: h.Key, d: h.Word}
		if h.Do == "write" {
			// m goes before g and enter on a narrow line (page.DropRank);
			// "? キー" stays to the end: it shows them all.
			x.rank = page.DropRank(h.Do)
		}
		tail = append(tail, x)
	}
	if !a.onComments() && a.top().taller() {
		// The page is taller than the screen: how to move the view alone.
		keys = append(append([]hint{}, keys...), hint{k: "^d ^u", d: "スクロール", rank: rankU})
	}
	q := tail[len(tail)-1] // "? キー" (Bar ends with it)
	if !a.onComments() && a.st != nil && !a.b.NoSession && len(a.b.State.Thoughts) > 0 {
		// The comments screen, with how many the AI has not finished (the
		// browser's コメント N): before "? キー"; on a narrow line it goes with
		// m, before "? キー" does.
		l := hint{k: "l", d: fmt.Sprintf("コメント %d", a.openCount()), rank: page.RankPage}
		tail = append(append(tail[:len(tail)-1:len(tail)-1], l), q)
	}
	for _, h := range more {
		switch h.Do {
		case "undo":
			// Something to take back: dropped first when narrow.
			keys = append(append([]hint{}, keys...), hint{k: h.Key, d: h.Word, rank: page.DropRank(h.Do)})
		case "next":
			if !a.onComments() {
				// Where to go next: before "m 書く", dropped first when narrow.
				keys = append(append([]hint{}, keys...), hint{k: h.Key, d: h.Word, rank: page.DropRank(h.Do)})
			}
		case "jump":
			if !a.onComments() && len(a.jumpTargets(a.top())) > 0 {
				keys = append(append([]hint{}, keys...), hint{k: h.Key, d: h.Word, rank: page.DropRank(h.Do)})
			}
		}
	}
	shorten := func(ks []hint, word bool) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.short != "" {
				k.k, k.d = k.short, ""
				if word {
					k.d = k.shortD
				}
			}
			out[i] = k
		}
		return out
	}
	alt := func(ks []hint) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.alt != "" {
				k.d = k.alt
			}
			out[i] = k
		}
		return out
	}
	fits := func(ks []hint, r text) bool { return keyHints(ks).width() <= a.room(0, r) }
	for lvl := 0; lvl <= rankC; lvl++ {
		var ks []hint
		for _, k := range keys {
			if k.short != "" || k.rank == 0 || k.rank > lvl {
				ks = append(ks, k)
			}
		}
		var tl []hint
		for _, k := range tail {
			if k.rank == 0 || k.rank > lvl {
				tl = append(tl, k)
			}
		}
		cands := [][]hint{ks, alt(ks)}
		if lvl == rankC {
			// A hint with a short form keeps its words until every key
			// that can go has gone.
			cands = append(cands, shorten(alt(ks), true), shorten(alt(ks), false))
		}
		for _, cand := range cands {
			cand = append(cand, tl...)
			for _, r := range []text{join(keep, right), keep} { // right goes before any key
				if fits(cand, r) {
					return []string{a.compose(false, 0, nil, keyHints(cand), r)}
				}
			}
		}
	}
	if len(keep) > 0 {
		// keep stays: the keys that work everywhere go, the last ("? キー")
		// last.
		for i := range tail {
			if cand := tail[i+1:]; len(cand) > 0 && fits(cand, keep) {
				return []string{a.compose(false, 0, nil, keyHints(cand), keep)}
			}
		}
		// Alone: as the body, where it has the most room (cut only on the
		// narrowest screens, the arrow first).
		return []string{a.compose(false, 0, nil, keep, nil)}
	}
	// Too narrow for all: what the person must know to react (the short
	// form of a kept hint) comes first, then "? キー".
	var must []hint
	for _, k := range shorten(keys, false) {
		if k.short != "" {
			must = append(must, k)
		}
	}
	for _, cand := range [][]hint{append(append([]hint{}, must...), q), must} {
		if len(must) > 0 && fits(cand, nil) {
			return []string{a.compose(false, 0, nil, keyHints(cand), nil)}
		}
	}
	return []string{a.compose(false, 0, nil, keyHints([]hint{q}), nil)}
}

// packKeys lays the key hints out on one line beside right, or on two. Hints
// are never cut: when even two lines are too few, the keys that work
// everywhere (they come last) are left out first, the keys for this line
// stay, and "? キー" at the end shows them all.
func (a *app) packKeys(keys []hint, right text, more bool) []string {
	// lay fills the first line, then the second, in order. right goes on
	// line rl (1 or 2).
	lay := func(ks []hint, rl int) ([]string, bool) {
		var rights [2]text
		rights[rl-1] = right
		var ls [2][]hint
		li := 0
		for _, k := range ks {
			for li < 2 && keyHints(append(ls[li], k)).width() > a.room(0, rights[li]) {
				li++
			}
			if li == 2 {
				return nil, false
			}
			ls[li] = append(ls[li], k)
		}
		if len(ls[0]) == 0 && len(ks) > 0 {
			return nil, false // the first line would hold nothing but right
		}
		out := []string{a.compose(false, 0, nil, keyHints(ls[0]), rights[0])}
		if len(ls[1]) > 0 || len(rights[1]) > 0 {
			out = append(out, a.compose(false, 0, nil, keyHints(ls[1]), rights[1]))
		}
		return out, true
	}
	try := func(ks []hint) ([]string, bool) {
		if out, ok := lay(ks, 1); ok {
			return out, true
		}
		if len(right) > 0 {
			return lay(ks, 2)
		}
		return nil, false
	}
	shorten := func(ks []hint) []hint {
		out := make([]hint, len(ks))
		for i, k := range ks {
			if k.short != "" {
				k.k, k.d = k.short, ""
			}
			out[i] = k
		}
		return out
	}
	if out, ok := try(keys); ok {
		return out
	}
	if more {
		// Keys are dropped from the end, so the ones for this line go last;
		// a hint with a short form is never dropped.
		for n := len(keys); n >= 0; n-- {
			var ks []hint
			for i, k := range keys {
				if i < n || k.short != "" {
					ks = append(ks, k)
				}
			}
			ks = append(ks, hint{k: "?", d: "キー"})
			if out, ok := try(ks); ok {
				return out
			}
			if out, ok := try(shorten(ks)); ok {
				return out
			}
		}
	}
	// Too narrow for anything: cut, as a last resort.
	return []string{a.compose(false, 0, nil, keyHints(shorten(keys)), right)}
}

// homeTilde writes a path under the home directory with ~.
func homeTilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return p
}

// hiddenCount is what the hints say on the right about hidden lines.
func (a *app) hiddenCount() text {
	pv := a.top()
	if pv == nil {
		return nil
	}
	c := pv.hiddenCount()
	switch {
	case c == 0:
		return nil
	case pv.showHidden:
		return plainText(fmt.Sprintf("隠した行 %d (表示中)", c), dim)
	}
	return plainText(fmt.Sprintf("隠した行 %d", c), dim)
}
