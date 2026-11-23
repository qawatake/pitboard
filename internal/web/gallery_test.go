package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/page"
	"github.com/qawatake/pitboard/internal/report"
)

// The gallery (/_gallery, docs/spec/web-states.md) draws every part and
// layer of the page from fixtures made here, from small reports and the
// person's acts on them: the data shapes are the code's, never written by
// hand. TestGalleryFixtures fails when the file checked in is not what the
// code makes now; go test ./internal/web -run TestGalleryFixtures -update
// writes it again.

var (
	update = flag.Bool("update", false, "write static/gallery/fixtures.json again")
	// boardsOut keeps the boards the fixtures are made from, one directory
	// per situation (DIR/<name>, its board -s gallery1): scripts/web-gallery.sh
	// serves them live and compares the live page with the gallery's.
	boardsOut = flag.String("boards", "", "keep the fixtures' boards in this directory")
)

const fixturesPath = "static/gallery/fixtures.json"

// Fixtures is static/gallery/fixtures.json.
type Fixtures struct {
	// Enums is every value a part or layer draws differently (page.Enums,
	// and the page's modes and connection states): each is covered by a
	// variant of the module that declares it.
	Enums      map[string][]string
	Situations map[string]*Situation
	// Index is the index page (/) in its situations: none, several, gone.
	Index map[string]Index
	// Refusals is page.Refusals: what each refusal says.
	Refusals map[string]string
	// Sent is what the bar says after a send.
	Sent string
	// Locked is the bar's offer on a line rewritten under the cursor.
	Locked *OfferJSON
}

// Situation is one data situation: the page as /api/page gives it, and
// what the bar offers on each of its lines.
type Situation struct {
	Note string
	Page PageJSON
	// Offers is, by line key, what works there when the person moved the
	// cursor to it, closed and open; Placed, when the page put it there.
	Offers map[string]OfferPair  `json:",omitempty"`
	Placed map[string]*OfferJSON `json:",omitempty"`
	// UndoMore is the bar's More for the tab that did the acts above (it
	// can take them back: u). Page is as a fresh tab sees it, as the live
	// page the gallery is compared with.
	UndoMore []page.Hint `json:",omitempty"`
}

// OfferPair is a line's offers, closed and open.
type OfferPair struct{ Closed, Open *OfferJSON }

// galleryNow is the time the fixtures are made at (a --gallery server's
// clock too).
var galleryNow = GalleryNow

type gboard struct {
	t  *testing.T
	st *board.Store
	b  *page.Board
}

func newGBoard(t *testing.T, name, src string, writeReport bool) *gboard {
	t.Helper()
	root := t.TempDir()
	if *boardsOut != "" {
		root = filepath.Join(*boardsOut, name)
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
	}
	st := board.Open(root, "gallery1")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if writeReport {
		if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &gboard{t: t, st: st, b: page.Open(st)}
}

func (g *gboard) node(id string) *report.Node {
	g.t.Helper()
	if n := report.ByID(g.b.Report.Nodes, id); n != nil {
		return n
	}
	var found *report.Node
	report.WalkAll(g.b.Report.Nodes, func(n *report.Node) {
		if found == nil && page.Stops(n) && n.Title() == id {
			found = n
		}
	})
	if found == nil {
		g.t.Fatalf("no line %q", id)
	}
	return found
}

func (g *gboard) must(_ string, err error) {
	g.t.Helper()
	if err != nil {
		g.t.Fatal(err)
	}
}

func (g *gboard) react(id, v string) { g.must(g.b.React(page.SeenOf(g.node(id)), v)) }
func (g *gboard) hide(id string) {
	g.t.Helper()
	_, _, err := g.b.Hide(page.SeenOf(g.node(id)), false)
	g.must("", err)
}

// comment writes text on line id ("" none) and says its number.
func (g *gboard) comment(id, title, text string) int {
	g.t.Helper()
	if id != "" && title == "" {
		title = g.node(id).Title()
	}
	g.must(g.b.Comment(id, title, text))
	ts := g.b.State.Thoughts
	return ts[len(ts)-1].N
}

// undoReact takes back the reaction on id (its record, as u does).
func (g *gboard) undoReact(id string) {
	g.t.Helper()
	if _, err := g.st.Append(board.Event{Op: "undo", Undo: g.b.State.React[id].N}); err != nil {
		g.t.Fatal(err)
	}
	g.b.Load()
}

func (g *gboard) send() {
	g.t.Helper()
	if _, err := g.st.Append(board.Event{Op: "send"}); err != nil {
		g.t.Fatal(err)
	}
	g.b.Load()
}

// read is the AI reading what was sent ('changes').
func (g *gboard) read() {
	g.t.Helper()
	if _, err := g.st.Changes(); err != nil {
		g.t.Fatal(err)
	}
	g.b.Load()
}

func (g *gboard) settle(ss ...board.Settlement) {
	g.t.Helper()
	if err := g.st.SettleAll(ss); err != nil {
		g.t.Fatal(err)
	}
	g.b.Load()
}

func (g *gboard) image() int {
	g.t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	n, err := g.b.SaveImage(buf.Bytes())
	if err != nil {
		g.t.Fatal(err)
	}
	return n
}

// situation is the board's page now, as a tab is given it, made the same
// on every machine (time, directory), with the offers of its lines.
func (g *gboard) situation(note string, showHidden, placed bool) *Situation {
	var fresh *page.Board
	if g.st != nil && g.b.Store != nil {
		fresh = page.Open(g.st)
	} else {
		fresh = g.b
	}
	t := &tab{b: fresh, seen: map[string]seenLine{}}
	out := &Situation{Note: note, Page: t.pageJSON(g.st, showHidden)}
	if g.b.CanWrite() && len(g.b.Hist) > 0 {
		out.UndoMore, _ = g.b.Bar(showHidden)
	}
	out.Page.Dir = "~/dev/sample"
	var walk func(ts []*page.Thing)
	walk = func(ts []*page.Thing) {
		for _, x := range ts {
			if !x.Right.At.IsZero() {
				x.Right.AtText = page.AtWord(x.Right.At, galleryNow)
			}
			if x.Stop && x.Node != nil {
				if out.Offers == nil {
					out.Offers = map[string]OfferPair{}
				}
				of := func(tc page.Touch) *OfferJSON {
					o := g.b.Offer(x.Node, tc)
					return &OfferJSON{Hints: o.Hints, Options: o.Options}
				}
				out.Offers[x.Key] = OfferPair{Closed: of(page.Touch{}), Open: of(page.Touch{Open: true})}
				if placed {
					if out.Placed == nil {
						out.Placed = map[string]*OfferJSON{}
					}
					out.Placed[x.Key] = of(page.Touch{Placed: true})
				}
			}
			walk(x.Kids)
		}
	}
	walk(out.Page.Notices)
	walk(out.Page.Things)
	placeholdTokens(out.Page.Page)
	return out
}

// placeholdTokens replaces each line's Token (a content hash: it would
// read as a secret to a scanner) with a short placeholder from the line's
// id, or its place: the gallery only needs them to match one another.
// How real tokens are made does not change.
func placeholdTokens(p *page.Page) {
	n := 0
	var walk func(ts []*page.Thing)
	walk = func(ts []*page.Thing) {
		for _, t := range ts {
			if t.Token != "" {
				n++
				if t.Target != "" {
					t.Token = "tok-" + t.Target
				} else {
					t.Token = "tok-" + strconv.Itoa(n)
				}
			}
			walk(t.Kids)
			walk(t.Body)
			walk(t.Subs)
		}
	}
	walk(p.Notices)
	walk(p.Things)
}

// reSecretLike is what a secret scanner would take for a key: a run of 16
// or more hex digits, or a base64-looking run (24 or more, letters and
// digits mixed).
var (
	reHexRun    = regexp.MustCompile(`[0-9a-f]{16,}`)
	reBase64Run = regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}={0,2}`)
	reDigit     = regexp.MustCompile(`[0-9]`)
	reLetter    = regexp.MustCompile(`[A-Za-z]`)
)

// secretLike is the first run in b that looks like a secret ("" none).
func secretLike(b []byte) string {
	if m := reHexRun.Find(b); m != nil {
		return string(m)
	}
	for _, m := range reBase64Run.FindAll(b, -1) {
		if reDigit.Match(m) && reLetter.Match(m) && !bytes.Contains(m, []byte("/")) {
			return string(m)
		}
	}
	return ""
}

const denseReport = `<Notice id="restart" at="2026-11-24 09:12">pitboard を開き直してください
  新しい版を入れました。q で閉じて、もう一度 pitboard を動かします。
</Notice>
<Notice id="ci" at="2026-11-20 18:30">CI が赤です</Notice>
<Title>pitboard を作る</Title>

inbox を広げた pitboard を作る。いまは **報告の書き方** を決めている。` + "`report.md`" + ` は [仕様](https://example.com/spec) のとおり。

<Section title="あなたへ">
  <Ask id="unit">ページの単位
    <Option key="a">セッションごと</Option>
    <Option key="b">プロジェクトごと</Option>
  </Ask>
  <Ask id="where">データの置き場所
    <Option key="a">cwd</Option>
    <Option key="b">XDG の状態ディレクトリ (長い選択肢の文は折り返し、選んでいてもいなくても同じ列にそろう)</Option>
  </Ask>
  <Item id="hook" ask="y">hook を入れるか決める
    入れると、書いたことが AI の作業中にも届く。` + "`~/.claude/settings.json`" + ` を触る。
  </Item>
  <Item id="merge" ask="y">WEB4 をこのまま merge してよいか</Item>
  <Item id="reply" look="y">> 左の印の列をどこに置くか
    ページの左の余白に置く。題の文の左端は見出しとそろえる。
  </Item>
  <Item id="parts" status="あなた待ち" wait="you" note="上の問いの答え">部品の一覧を決める</Item>
</Section>

<Section title="進行中">
  <Item id="impl" state="doing">最小版を実装する</Item>
  <Item id="web" state="doing" status="レビュー中">ブラウザ版を部品に分ける
    - 部品ごとにモジュールに分ける
      - 入れ子の行
    - ギャラリーを作る
    1. まず描く
    2. 次に測る

    **手順**
    段落の 2 つ目。
  </Item>
  <Item id="next1" note="上が済んでから">次にやること</Item>
  <Item id="old" state="done">viewer を 1 列にする</Item>
  <Item id="olddiff" state="done">差分を見る
    <Code lang="diff">
    @@ -1,3 +1,3 @@
    -  .row.cur { outline: 1px solid var(--accent); }
    +  .row.cur > .rowbtn > .gut::before { content: ">"; }
    </Code>
  </Item>
</Section>

<Section title="見せたいもの">
  <Link id="spec" path="docs/spec/SPEC.md">最小版の仕様</Link>
  <Link id="pr" url="https://example.com/pull/1" ask="y">プルリクエスト</Link>
  <Code>
  go test ./...
  </Code>
  - ただの行
  - もう 1 つのただの行
</Section>

<Section title="あとで">
</Section>

<Fold title="決まったこと (3)">
  <Item id="tui" state="done">viewer を 1 列にする</Item>
  - id の要らない、ただの行
  <Fold title="中の折りたたみ">
    <Item id="deep">入れ子の項目
      中身
    </Item>
  </Fold>
</Fold>
`

const commentsReport = `<Title>コメント</Title>

<Section title="行に付いたコメント">
  <Item id="c1">未送信のコメントのある行
    中身
  </Item>
  <Item id="c2">送った行</Item>
  <Item id="c3">AI が読んだ行</Item>
  <Item id="c4">対応中の行</Item>
  <Item id="c5">保留の行</Item>
  <Item id="c6">対応済の行</Item>
  <Item id="c7">題を書き換えた行</Item>
  <Item id="u1" ask="y">送った y を取り消した行</Item>
  <Fold id="f1" title="コメントのある折りたたみ">
    <Item id="c8">中の行</Item>
  </Fold>
</Section>
`

const hiddenReport = `<Notice id="n1">隠したお知らせ</Notice>
<Notice id="n2">見えているお知らせ</Notice>
<Title>隠した行</Title>

<Section title="節">
  <Item id="h1">隠した項目</Item>
  <Item id="v1">見えている項目</Item>
  <Link id="h2" path="a/b.md">隠したリンク</Link>
  <Ask id="h3">隠した問い
    <Option key="a">はい</Option>
    <Option key="b">いいえ</Option>
  </Ask>
  - 隠したただの行
  - 見えているただの行
  <Fold id="h4" title="隠した折りたたみ">
    - 中
  </Fold>
</Section>
`

const longReport = `<Notice id="long">とても長いお知らせの題がここに入ります。とても長いお知らせの題がここに入ります。ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ
  https://example.com/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going/for/a/while/until/it/overflows/the/box
</Notice>
<Title>長い文字列 ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ</Title>

<Section title="長いもの ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ">
  <Item id="url" status="とても長い状態の語です">切れ目の無い URL を持つ項目の題は、長くても折り返して右端の語と並ぶ。題は長い ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ
    https://example.com/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going/for/a/while/until/it/overflows/the/box
    ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJK
    - 箇条の中の長い列 ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ
    <Code>
    const veryLongLine = "a code line that is much longer than the box it sits in, and scrolls inside its own box only, never clipped";
    </Code>
  </Item>
  <Ask id="lask">長い選択肢
    <Option key="a">ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ</Option>
    <Option key="b">短い</Option>
  </Ask>
  <Link id="longlink" url="https://example.com/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going/for/a/while/until/it/overflows">長いリンク</Link>
  - ただの行にも長い列 ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ
</Section>
`

const problemsReport = `<Title>誤りのある報告</Title>

<Section title="節">
  <Item>id の無い項目</Item>
  <Item id="ok" state="later">知らない状態</Item>
  <Bogus>知らない部品</Bogus>
</Section>
`

// makeFixtures makes every situation.
func makeFixtures(t *testing.T) *Fixtures {
	prevLocal := time.Local
	time.Local = galleryNow.Location()
	t.Cleanup(func() { time.Local = prevLocal })
	t.Setenv("PITBOARD_SEND", "")
	f := &Fixtures{Situations: map[string]*Situation{}, Locked: &OfferJSON{Locked: true, Message: page.MsgMoved}}
	f.Enums = page.Enums()
	f.Enums["mode"] = Modes
	f.Enums["conn"] = Conns

	// dense: one of each part, answered and not, a reaction of each kind,
	// something to take back and to send.
	g := newGBoard(t, "dense", denseReport, true)
	g.react("merge", "y")
	g.react("pr", "n")
	g.react("unit", "b")
	g.comment("hook", "", "入れてよい。settings.json は手で直す")
	f.Situations["dense"] = g.situation("a whole report: every part, reactions, something to send and to take back", false, true)

	// comments: a comment in every state, on lines and on none.
	g = newGBoard(t, "comments", commentsReport, true)
	g.react("u1", "y")
	c3 := g.comment("c3", "", "読んでください")
	c4 := g.comment("c4", "", "直してほしい")
	c5 := g.comment("c5", "", "あとでよい")
	c6 := g.comment("c6", "", "これで合っている？")
	f8 := g.comment("f1", "", "折りたたみへのコメント")
	g.send()
	g.read()
	g.settle(board.Settlement{N: c4, State: "doing", Text: "直しています"},
		board.Settlement{N: c5, State: "hold", Text: "次の版で"},
		board.Settlement{N: c6, State: "done", Text: "合っています。\n2 行目の返事"},
		board.Settlement{N: f8, State: "done", Text: "対応しました"})
	_ = c3
	g.comment("c2", "", "送ったコメント")
	g.send()
	n := g.image()
	g.comment("c1", "", "1 行目\n2 行目\n3 行目 [Image #"+itoa(n)+"]")
	g.comment("c1", "", "同じ行への 2 つ目")
	g.comment("c7", "前の題", "題が変わる前に書いた")
	g.comment("", "", "宛先なしのコメント")
	g.comment("gone", "消えた行の題", "報告から消えた行へのコメント")
	// The y on u1 went with the sends above; taken back now, not sent:
	// 取り消し 未送信.
	g.undoReact("u1")
	f.Situations["comments"] = g.situation("comments in every state: unsent, sent, read, doing, hold, done, on a gone line, on none, with an image", false, false)

	// hidden: every kind hidden, shown with H.
	g = newGBoard(t, "hidden", hiddenReport, true)
	for _, id := range []string{"n1", "h1", "h2", "h3", "隠したただの行", "h4"} {
		g.hide(id)
	}
	f.Situations["hidden"] = g.situation("hidden lines of every kind, shown (H)", true, false)

	// long: strings with no place to break.
	g = newGBoard(t, "long", longReport, true)
	g.comment("url", "", "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJK https://example.com/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going")
	f.Situations["long"] = g.situation("long unbroken strings everywhere", false, false)

	f.Situations["problems"] = newGBoard(t, "problems", problemsReport, true).situation("a report with format errors", false, false)
	f.Situations["empty"] = newGBoard(t, "empty", "", true).situation("an empty report", false, false)
	f.Situations["noreport"] = newGBoard(t, "noreport", "", false).situation("no report written yet", false, false)
	nb := &gboard{t: t, b: page.Open(nil), st: board.Open("/nowhere", "none")}
	f.Situations["nosession"] = nb.situation("no session yet", false, false)

	f.Refusals = page.Refusals
	f.Sent = "送りました"
	f.Index = indexFixtures(t)
	return f
}

// indexFixtures is the index page with no board, several, and one whose
// directory went.
func indexFixtures(t *testing.T) map[string]Index {
	t.Helper()
	out := map[string]Index{}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	mk := func(name, src string, act func(g *gboard)) *board.Store {
		root := filepath.Join(home, "dev", name)
		st := board.Open(root, "s"+strconv.Itoa(len(name)))
		if err := os.MkdirAll(st.Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if src != "" {
			if err := os.WriteFile(st.ReportPath(), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if act != nil {
			act(&gboard{t: t, st: st, b: page.Open(st)})
		}
		return st
	}
	index := func(sts ...*board.Store) Index {
		s, err := New(Options{Port: 0})
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range sts {
			if _, err := s.Register(st.Base, st.Session); err != nil {
				t.Fatal(err)
			}
		}
		// A board registered whose directory then went (before the
		// watcher drops it).
		for _, st := range sts {
			if strings.HasSuffix(st.Base, "moved-away") {
				if err := os.RemoveAll(st.Base); err != nil {
					t.Fatal(err)
				}
			}
		}
		ix := s.index()
		for i := range ix.Boards {
			ix.Boards[i].ID, ix.Boards[i].Dir = "board"+strconv.Itoa(i+1), ""
		}
		return ix
	}
	out["none"] = index()
	a := mk("pitboard", denseReport, func(g *gboard) { g.react("merge", "y"); g.comment("hook", "", "入れてよい") })
	b := mk("notes", "", nil)
	c := mk("a-project-with-a-rather-long-directory-name-that-wraps", "<Title>長い場所</Title>\n<Item id=\"x\" look=\"y\">読む</Item>\n", nil)
	out["several"] = index(a, b, c)
	d := mk("moved-away", "<Title>動かしたプロジェクト</Title>\n", nil)
	out["gone"] = index(a, d)
	return out
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestGalleryFixtures(t *testing.T) {
	f := makeFixtures(t)
	got, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	// Nothing in it may look like a secret (the commit's scanner rejects
	// it, and no allowlist is kept for it).
	if m := secretLike(got); m != "" {
		t.Fatalf("the fixtures hold a run that looks like a secret: %q", m)
	}
	if *update {
		if err := os.MkdirAll(filepath.Dir(fixturesPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixturesPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(fixturesPath)
	if err != nil {
		t.Fatalf("%v (go test ./internal/web -run TestGalleryFixtures -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not what the code makes now: go test ./internal/web -run TestGalleryFixtures -update", fixturesPath)
	}
}

// A module's declarations, as static data: covers = {axis: {value:
// [variant names]}}, differences = {name: [variant names]}, and its
// variants' names (each written name: '...').
var (
	reCovers      = regexp.MustCompile(`(?s)export const covers = (\{.*?\});\n`)
	reDifferences = regexp.MustCompile(`(?s)export const differences = (\{.*?\});\n`)
	reVariants    = regexp.MustCompile(`(?s)export const variants = \[(.*)\n\];`)
	reName        = regexp.MustCompile(`name: '([^']*)'`)
	reDiffs       = regexp.MustCompile(`(?s)export const DIFFS = \{(.*?)\n\};`)
	reDiffKey     = regexp.MustCompile(`(?m)^  '([a-z-]+)': `)
)

// requiredDifferences is every visual difference that is no enum value
// and must be shown by some variant (WEBFIX3): a module declares it, the
// gallery checks the drawn DOM shows it.
var requiredDifferences = []string{
	"rights-below", "unpinned", "under-band", "under-bar", "at-bottom", "focus-bar",
	"loose-editor", "editor-empty", "edit-comment", "panel-scrolled", "help-scrolled",
	"image-large", "message", "sent", "index-none", "index-several", "index-gone",
}

// TestGalleryCovers: every value of every enum (page's, the modes, the
// connection states) and every required visual difference is declared by
// a module, each naming the variants that show it, and each named variant
// exists. Whether a named variant really shows it is the gallery's check
// on the drawn DOM (scripts/web-gallery.sh, part of scripts/check.sh).
func TestGalleryCovers(t *testing.T) {
	f := makeFixtures(t)
	have := map[string]map[string][]string{} // axis -> value -> modules
	diffs := map[string][]string{}
	var files []string
	for _, dir := range []string{"static/parts", "static/layers"} {
		es, err := staticFS.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range es {
			files = append(files, dir+"/"+e.Name())
		}
	}
	files = append(files, "static/gallery/pages.js")
	for _, file := range files {
		b, _ := staticFS.ReadFile(file)
		vm := reVariants.FindSubmatch(b)
		if vm == nil {
			continue // a helper, not a module
		}
		names := map[string]bool{}
		for _, n := range reName.FindAllSubmatch(vm[1], -1) {
			names[string(n[1])] = true
		}
		m := reCovers.FindSubmatch(b)
		if m == nil {
			t.Errorf("%s has variants but no covers", file)
			continue
		}
		var cov map[string]map[string][]string
		if err := json.Unmarshal(m[1], &cov); err != nil {
			t.Errorf("%s: covers is not {axis: {value: [variant names]}}: %v", file, err)
			continue
		}
		named := func(what string, vs []string) {
			if len(vs) == 0 {
				t.Errorf("%s: %s names no variant", file, what)
			}
			for _, v := range vs {
				if !names[v] {
					t.Errorf("%s: %s names %q, which is no variant of it", file, what, v)
				}
			}
		}
		for axis, vals := range cov {
			if _, ok := f.Enums[axis]; !ok {
				t.Errorf("%s covers %q, which is no enum", file, axis)
			}
			for v, vs := range vals {
				if !slices.Contains(f.Enums[axis], v) {
					t.Errorf("%s covers %s %q, which is no value", file, axis, v)
				}
				named(axis+" "+v, vs)
				if have[axis] == nil {
					have[axis] = map[string][]string{}
				}
				have[axis][v] = append(have[axis][v], file)
			}
		}
		if dm := reDifferences.FindSubmatch(b); dm != nil {
			var ds map[string][]string
			if err := json.Unmarshal(dm[1], &ds); err != nil {
				t.Errorf("%s: differences is not {name: [variant names]}: %v", file, err)
				continue
			}
			for d, vs := range ds {
				named("difference "+d, vs)
				diffs[d] = append(diffs[d], file)
			}
		}
	}
	for axis, vs := range f.Enums {
		for _, v := range vs {
			if len(have[axis][v]) == 0 {
				t.Errorf("no part or layer has a variant for %s %q", axis, v)
			}
		}
	}
	// Every difference is declared somewhere, and the gallery can tell
	// from the DOM whether a variant shows it.
	g, _ := staticFS.ReadFile("static/gallery/gallery.js")
	known := map[string]bool{}
	if m := reDiffs.FindSubmatch(g); m != nil {
		for _, k := range reDiffKey.FindAllSubmatch(m[1], -1) {
			known[string(k[1])] = true
		}
	}
	for _, d := range requiredDifferences {
		if len(diffs[d]) == 0 {
			t.Errorf("no module declares the difference %q", d)
		}
	}
	for d, files := range diffs {
		if !known[d] {
			t.Errorf("%v declare the difference %q, which gallery.js's DIFFS cannot see", files, d)
		}
	}
	// state.js's modes are Modes.
	b, _ := staticFS.ReadFile("static/state.js")
	m := regexp.MustCompile(`export const MODES = (\[[^\]]*\]);`).FindSubmatch(b)
	var modes []string
	if m == nil || json.Unmarshal(m[1], &modes) != nil || !slices.Equal(modes, Modes) {
		t.Errorf("state.js MODES is not Modes %q", Modes)
	}
	m = regexp.MustCompile(`export const CONNS = (\[[^\]]*\]);`).FindSubmatch(b)
	var conns []string
	if m == nil || json.Unmarshal(m[1], &conns) != nil || !slices.Equal(conns, Conns) {
		t.Errorf("state.js CONNS is not Conns %q", Conns)
	}
}

func TestSecretLike(t *testing.T) {
	// The samples are made here, so that this file holds none itself.
	hex := strings.Repeat("3f2a9c0d", 4)
	b64 := base64.StdEncoding.EncodeToString([]byte("not a key, but looks like one 123456"))
	for s, want := range map[string]bool{
		`"Token": "` + hex + `"`:                                    true,
		`"k": "` + b64 + `"`:                                        true,
		`"Token": "tok-hook"`:                                       false,
		strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 2):             false,
		"https://example.com/a/very/long/path/that/has/no/spaces/1": false,
	} {
		if got := secretLike([]byte(s)) != ""; got != want {
			t.Errorf("%s: %v", s, got)
		}
	}
}
