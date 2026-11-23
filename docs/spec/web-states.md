# ブラウザの状態の一覧 (ギャラリー)

`pitboard serve` のページは、部品 (`internal/web/static/parts/*.js`) と層 (`layers/*.js`) が自分を描く。描くのは `render.js` だけで、`(page の JSON, uiState)` から DOM を作る (fetch・タイマー・リスナー・大域の状態を持たない)。生きたページ (`main.js`) もギャラリーも同じ `render.js` で描く。描いたあとに配置を測る段 (`render.fit`: バーのキーを落とす・狭い幅で右端の語を下へ・高すぎるお知らせの固定をやめる) も共通。

## 決まり

**部品や層は自分の変種 (variant) を持つ。新しい enum の値・新しいモード・新しい見た目の違いを足したら、それを持つモジュールに変種を足す。**

- 各モジュールは描く関数の隣に `export const variants = [{name, fixture, pick, open, ui, …}]` と `export const covers = {kind: [...], mark: [...], …}` を書く。変種は「その部品だけの意味のある違い」1 つ
- 部品に固有でない軸 (カーソルが乗っている・幅 1280 / 390・明暗) は一覧に書かない。ギャラリーがすべての変種にかける (上の切り替え。撮影は既定で 1280 明るい、モジュールごとに 390 暗いの 1 枚、`responsive` / `themed` の変種だけ全部の組み合わせ)
- データは手で書かない: `internal/web/gallery_test.go` が小さな報告と操作の列から `page` の JSON を作り、`static/gallery/fixtures.json` に置く (dense・comments・hidden・long・problems・empty・noreport・nosession)。コードが変われば `TestGalleryFixtures` が落ちる。作り直しは `go test ./internal/web -run TestGalleryFixtures -update`
- 漏れはデータの型から見つける: `TestGalleryCovers` は `page.Enums()` (Kind・Style・Tone・Mark・Item の状態・コメントの状態・AI の settle・Actions・キーの Do) と `web.Modes`・`web.Conns` のどの値も、どこかのモジュールの `covers` にあることを確かめる (`state.js` の `MODES` `CONNS` が Go と同じことも)。ギャラリー自身は、各モジュールの変種が `covers` に書いたものを本当に見せているかを、描いたデータとバーのボタンから確かめる
- ページ全体の組み合わせ (部品の間の空き) は `gallery.js` の `PAGES` に 5 つ

## 見る・撮る

- `/_gallery` (同じ鍵の cookie で入る)。`?w=390&theme=dark&cur=1&only=item` などで絞る
- `scripts/web-gallery.sh [OUT]`: スクラッチのサーバー (自分の状態ディレクトリ・`--no-open`・空きポート) と Playwright 同梱の headless Chromium (`playwright-cli`) で、すべての変種を `OUT/<module>/NN-name.png` に、モジュールごとの 390 暗いの 1 枚を `sheet-390-dark.png` に撮り、`OUT/index.html` に並べる。変種が例外を出した・何も描かなかった・Code 以外に箱より広いものがある・`covers` のものを見せていない、のどれかがあれば 0 でない終了コード。クリックで状態に入ることはしない
- キーでの移り変わり (フォーカス・守り) は `page` の Go のテストと、少しの `playwright-cli` の確認で見る。これを二つ目の一覧には育てない
