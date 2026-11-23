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

var update = flag.Bool("update", false, "write static/gallery/fixtures.json again")

const fixturesPath = "static/gallery/fixtures.json"

// Fixtures is static/gallery/fixtures.json.
type Fixtures struct {
	// Enums is every value a part or layer draws differently (page.Enums,
	// and the page's modes and connection states): each is covered by a
	// variant of the module that declares it.
	Enums      map[string][]string
	Situations map[string]*Situation
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
}

// OfferPair is a line's offers, closed and open.
type OfferPair struct{ Closed, Open *OfferJSON }

// galleryNow is the time the fixtures are made at.
var galleryNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))

type gboard struct {
	t  *testing.T
	st *board.Store
	b  *page.Board
}

func newGBoard(t *testing.T, src string, writeReport bool) *gboard {
	t.Helper()
	st := board.Open(t.TempDir(), "gallery1")
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
	t := &tab{b: g.b, seen: map[string]seenLine{}}
	out := &Situation{Note: note, Page: t.pageJSON(g.st, showHidden)}
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
	g := newGBoard(t, denseReport, true)
	g.react("merge", "y")
	g.react("pr", "n")
	g.react("unit", "b")
	g.comment("hook", "", "入れてよい。settings.json は手で直す")
	f.Situations["dense"] = g.situation("a whole report: every part, reactions, something to send and to take back", false, true)

	// comments: a comment in every state, on lines and on none.
	g = newGBoard(t, commentsReport, true)
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
	// 取消 未送信.
	g.undoReact("u1")
	f.Situations["comments"] = g.situation("comments in every state: unsent, sent, read, doing, hold, done, on a gone line, on none, with an image", false, false)

	// hidden: every kind hidden, shown with H.
	g = newGBoard(t, hiddenReport, true)
	for _, id := range []string{"n1", "h1", "h2", "h3", "隠したただの行", "h4"} {
		g.hide(id)
	}
	f.Situations["hidden"] = g.situation("hidden lines of every kind, shown (H)", true, false)

	// long: strings with no place to break.
	g = newGBoard(t, longReport, true)
	g.comment("url", "", "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJK https://example.com/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going")
	f.Situations["long"] = g.situation("long unbroken strings everywhere", false, false)

	f.Situations["problems"] = newGBoard(t, problemsReport, true).situation("a report with format errors", false, false)
	f.Situations["empty"] = newGBoard(t, "", true).situation("an empty report", false, false)
	f.Situations["noreport"] = newGBoard(t, "", false).situation("no report written yet", false, false)
	nb := &gboard{t: t, b: page.Open(nil), st: board.Open("/nowhere", "none")}
	f.Situations["nosession"] = nb.situation("no session yet", false, false)
	return f
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

// reCovers is a module's declaration of what it covers: a JSON object.
var reCovers = regexp.MustCompile(`(?s)export const covers = (\{.*?\});\n`)

// TestGalleryCovers: every value of every enum (page's, the modes, the
// connection states) is covered by the module of a part or layer that
// declares it (each module's variants are checked against what it
// declares by the gallery itself, scripts/web-gallery.sh).
func TestGalleryCovers(t *testing.T) {
	f := makeFixtures(t)
	have := map[string]map[string][]string{} // axis -> value -> modules
	for _, dir := range []string{"static/parts", "static/layers"} {
		es, err := staticFS.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range es {
			b, _ := staticFS.ReadFile(dir + "/" + e.Name())
			m := reCovers.FindSubmatch(b)
			if m == nil {
				if !strings.Contains(string(b), "export const variants") {
					continue // a helper, not a part
				}
				t.Errorf("%s/%s has variants but no covers", dir, e.Name())
				continue
			}
			var cov map[string][]string
			if err := json.Unmarshal(m[1], &cov); err != nil {
				t.Errorf("%s/%s: covers is not JSON: %v", dir, e.Name(), err)
				continue
			}
			for axis, vs := range cov {
				if _, ok := f.Enums[axis]; !ok {
					t.Errorf("%s/%s covers %q, which is no enum", dir, e.Name(), axis)
				}
				for _, v := range vs {
					if !slices.Contains(f.Enums[axis], v) {
						t.Errorf("%s/%s covers %s %q, which is no value", dir, e.Name(), axis, v)
					}
					if have[axis] == nil {
						have[axis] = map[string][]string{}
					}
					have[axis][v] = append(have[axis][v], e.Name())
				}
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
