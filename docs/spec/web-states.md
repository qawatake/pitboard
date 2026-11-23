# ブラウザの状態の一覧 (ギャラリー)

`pitboard serve` のページは、部品 (`internal/web/static/parts/*.js`) と層 (`layers/*.js`) が自分を描く。描くのは `render.js` だけで、`(page の JSON, uiState)` から DOM を作る (fetch・タイマー・リスナー・大域の状態を持たない)。生きたページ (`main.js`) もギャラリーも同じ `render.js` で描く。一覧のページ `/` は `layers/home.js` が描き、`home.js` (生きたページ) とギャラリーが使う。描いたあとに配置を測る段 (`render.fit`: バーのキーを落とす・`g` の題を縮める・狭い幅で右端の語を下へ・高すぎるお知らせの固定をやめる) も共通。

## 決まり

**部品や層は自分の変種 (variant) を持つ。新しい enum の値・新しいモード・新しい見た目の違いを足したら、それを持つモジュールに変種を足し、どの変種が見せるかを宣言する。**

- 変種は生きたページと同じ道で描く: 層とページ全体は `render()`、部品は `renderPart()` (`render()` と同じ `things()` と同じ置き場。報告のいちばん外の行なら、いちばん外として描く: `Fold` は見出し)。そのあと `fit()`。変種に「どう見せるか」を渡す道は無い
- 各モジュールは描く関数の隣に、静的なデータとして書く:
  - `export const variants = [{name: '…', fixture, pick, open, ui, w, h, …}]`。`name` は文字列そのまま (`go test` が読む)
  - `export const covers = {軸: {値: [変種の name]}}`: データの型のどの値を、どの変種が見せるか
  - `export const differences = {名前: [変種の name]}`: enum でない見た目の違い (`rights-below` `unpinned` `under-band` `under-bar` `at-bottom` `focus-bar` `loose-editor` `editor-empty` `edit-comment` `panel-scrolled` `help-scrolled` `image-large` `message` `sent` `index-none` `index-several` `index-gone`)
- 部品に固有でない軸 (カーソルが乗っている・幅 1280 / 390・明暗) は一覧に書かない。ギャラリーがすべての変種にかける。変種が自分の幅や高さを決めるときは `w` `h`
- データは手で書かない: `internal/web/gallery_test.go` が小さな報告と操作の列から `page` の JSON を作り、`static/gallery/fixtures.json` に置く (dense・comments・hidden・long・problems・empty・noreport・nosession、一覧のページの none・several・gone、断りの文、送ったあとの文)。ページは新しいタブが見るとおり (`u` は無い)。操作したタブのバーは `UndoMore`。コードが変われば `TestGalleryFixtures` が落ちる。作り直しは `go test ./internal/web -run TestGalleryFixtures -update`
- 漏れは 2 段で見つける:
  - `go test` (`TestGalleryCovers`): `page.Enums()` (Kind・Style・Tone・Mark・状態・コメントの状態・settle・Actions・キーの Do・断り) と `web.Modes`・`web.Conns` のどの値も、どこかのモジュールの `covers` にあり、名前を挙げた変種が実際にあること。決まった見た目の違いがどれもどこかの `differences` にあり、`gallery.js` の `DIFFS` がそれを DOM から見分けられること。`state.js` の `MODES` `CONNS` が Go と同じこと
  - ギャラリー (スクリプト): 名前を挙げた変種が本当にそれを見せていること。見るのは描いた DOM だけ (クラス・属性・配置・フォーカス。変種の `ui` やデータは見ない)
- ページ全体の組み合わせ (部品の間の空き、帯と本文の重なり、フォーカス) は `gallery/pages.js`

## 生きたページと同じか (fidelity)

`scripts/web-gallery.sh` は、fixture を作った報告と操作の列を本物のボードとして置き (`go test … -boards DIR`)、`pitboard serve --foreground --gallery` で出す。`--gallery` のサーバーでだけ、ページは `?ui=<JSON>` で状態に入り、時計は fixture と同じ (`web.GalleryNow`)。ふつうの `serve` は `?ui=` を読まない。すべてのページ全体 (`pages/*`) といくつかの変種を、生きたページを同じ状態にして DOM を比べる (id・キー・トークン・画像の場所・カーソルは除く)。部品は、その行の並びが生きたページの中に順に見つかること。違えば失敗。

## 見る・撮る・確かめる

- `/_gallery` (同じ鍵の cookie で入る)。`?w=390&theme=dark&cur=1&only=item` などで絞る
- `scripts/web-gallery.sh [OUT]`: スクラッチのサーバー (自分の状態ディレクトリ・`--no-open`・空きポート) と Playwright 同梱の headless Chromium (`playwright-cli`) で、すべての変種を `OUT/<module>/NN-name.png` に、部品ごとの最初の変種をカーソル付きで (`-cur`)、モジュールごとの 390 暗いの 1 枚を `sheet-390-dark.png` に撮り、`OUT/index.html` に並べ、`OUT/report.json` に変種ごとに DOM が見せたものを書く。変種が例外を出した・何も描かなかった・箱より広いものがある (`Code` も)・宣言したものを見せていない・生きたページと違う、のどれかがあれば 0 でない終了コード
- `scripts/check.sh [OUT]`: `go vet ./...`・`go test ./...`・`web-gallery.sh`。**ブラウザのページを変えたら、これが通るまで終わりではない**
- キーでの移り変わり (フォーカス・守り) は `page` の Go のテストと、少しの `playwright-cli` の確認で見る。これを二つ目の一覧には育てない
