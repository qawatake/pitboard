# WEBFIX2 の結果

vet / test (`-count=1`、e2e 込み) / build 通過。確認はスクラッチのボード・一時的な `XDG_STATE_HOME`・`--no-open`・`playwright-cli` (Playwright 同梱の `chromium_headless_shell-1248`) だけ。自分のサーバーとブラウザはすべて止め、`ps` と `playwright-cli list` で残っていないことを確かめた。作業中、私が起動していないサーバー (pid 68065、`~/src/tries/2026-11-23-cureent/pitboard/pitboard serve --foreground`) が動いていた。触っていない。

git について: 一度だけ誤って `git rm --cached internal/web/static/app.js` を実行し、すぐ `git reset -- internal/web/static/app.js` で index を元に戻した。いまの index は HEAD と同じ (作業ツリーで `app.js` を消しただけ)。ほかに git は使っていない。

ギャラリーの画像 (135 枚、問題 0): `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/255e557b-582b-407d-930d-aee14e3a218b/scratchpad/gallery/` (並べたもの: 同じ所の `index.html`)。

## A. 描き方は純粋な関数、ギャラリーは部品ごと

- `app.js` を分けた: `render.js` (`(page, uiState)` から DOM。fetch・タイマー・リスナー・大域の状態なし。`memo` は前に描いた行を覚えるだけで、新しい `memo` なら同じものを一から描く)、`state.js` (uiState の移り変わり。純粋)、`keys.js` (キー → 意図)、`net.js`、`main.js` (配線: フォーカス・スクロール・タイマー・保存)、`dom.js`
- 部品 `parts/{notice,section,item,ask,link,fold,text,code,comment}.js` (+ 骨組みの `row.js`)、層 `layers/{bar,goto,editor,help,comments,image,banners}.js`。それぞれが描く関数の隣に `variants` と `covers` を持つ。画像の表示はブリーフの層の一覧に無かったので `layers/image.js` を足した
- 描いたあとに測る段は `render.fit` 1 つ (バーのキーを落とす・狭い幅で右端の語を下へ・高すぎるお知らせの固定をやめる)。生きたページもギャラリーも呼ぶ
- fixture は Go から: `internal/web/gallery_test.go` が報告と操作の列から 8 つの場面 (dense, comments, hidden, long, problems, empty, noreport, nosession) と、各行のバーの申し出 (人が動かした・開いた・置かれた) を作り `static/gallery/fixtures.json` に置く。`TestGalleryFixtures` は違えば落ちる (`-update` で書き直す)
- 漏れはデータの型から: `page.Enums()` (Kind・Style・Tone・Mark・状態・コメントの状態・settle・Actions・キーの Do)、`web.Modes`、`web.Conns`。`TestGalleryCovers` はどの値もどこかのモジュールの `covers` にあること、`state.js` の `MODES` `CONNS` が Go と同じことを確かめる。ギャラリーは各モジュールの変種が `covers` を本当に見せているか (描いたデータとバーのボタンの `data-act`) を 1280 幅で確かめる
- `/_gallery` (同じ鍵)。幅 1280/390・明暗・カーソルは全変種にかかる切り替え。ページ全体の組み合わせ 5 つ
- `scripts/web-gallery.sh [OUT]`: スクラッチのサーバーと headless shell で、変種ごと・モジュールごとの 390 暗い 1 枚・`responsive` の変種の 4 通りを撮り、`index.html` を作る。例外・何も描かない・Code 以外の横のはみ出し・covers の不足で 0 以外で終わる。実行ビットは付けられなかった (chmod が許可されなかった): `sh scripts/web-gallery.sh` で動く。`chmod +x` をお願いしたい
- 気づいたこと: Playwright はソケットを `$TMPDIR` の下に作り、macOS では 104 字を超えると `EADDRINUSE` で開けない。スクリプトは長い `$TMPDIR` のときだけ Playwright に `/tmp` を渡す (自分の一時ファイルは `$GALLERY_TMP`)
- `docs/spec/web-states.md` を書いた。SPEC のブラウザの節を今回の決まりに合わせた

## B. 目録の前後 (1280 幅・明るい表示。測りは `getBoundingClientRect`、字の左端は Range の 1 字目)

| # | いま | 測ったもの |
|---|---|---|
| 1 | 直った | 長い URL・英字の列 (題・段落・箇条・コメント・リンク先) は折り返す。はみ出す要素 1280: 0 件 / 390: 0 件。Code は `overflow-x: auto` (scrollWidth 1068 / clientWidth 680、中でスクロール)。ギャラリーは全変種でこれを確かめる |
| 2 | 直った | 帯は地の色 (rgb 255) + 下に 1px の線 + 間。開いたお知らせが画面の 3 割を超えれば固定をやめて頭に置く (`fit`) |
| 3 | 直った | Tab でバーへ: `:focus-visible` true、outline 1px solid (フォーカスの色) |
| 4 | 直った (WEB4 のまま) | h1・見出し・段落・題・最上位 Fold・お知らせ・バー: すべて 316px |
| 5 | 直った (WEB4 のまま) | 題 316 → 中身 340 (+1 段) |
| 6 | 直った | 状態語は `nowrap` (7 行のコメントでも「未送信」は 1 行) |
| 7 | 直った (WEB4 のまま) | 開いた直後のフォーカスは行のボタンで outline none |
| 8 | 直った (WEB4 のまま) | 入れ子の `>` 260 = 外の `>` 260。入れ子の題 340、その中 364 |
| 9 | 直った | バー・知らせ・「コメント N」は本文と同じ左端 316 と右端 1020。1280 でバーは 1 行 (入らないキーは `page.DropRank` の順で落とす。端末も同じ関数に変えた)。390 で 2 行 |
| 10 | 直った | リンク先 316 = 題 316 |
| 11 | 直った | 選択肢の文は選んでも選ばなくても同じ列 (● はキーの左にぶら下がる)。ギャラリー ask/02・04 |
| 12 | 直った | 返事と画像はコメントの文の列 (✎ の右)。画像は文のすぐ下、返事の前。返事の改行も保つ |
| 13 | 直った | `g` の間も題 316 のまま (字は `>` の列 260)。バーに `esc やめる a 題 b 題 …` |
| 14 | 直った | 7 行で高さ 198 = scrollHeight 198 (伸びる)。上限は画面の 4 割、その先は中でスクロール |
| 15 | 直った | scrollTop 0、フォーカスは見出し「キー」、説明はふつうの色 (rgb 29,29,31)、「黄の ●」は黄。z: キー 110 > バー 30 > 帯 10、シートの上の点を取ると `.help`、ページはスクロールしない。390 は画面いっぱい |
| 16 | 直った | 隠したお知らせ: 薄い色 (rgb 95,99,104)、印 ⌀ |
| 17 | 直った | 欄のコメントは 7 行のまま (改行を保つ)。AI の返事も |
| 18 | 直った | 消える決まりを `page.MsgClears` に 1 つ (次のキー・エディタ・欄・ポインタ。読み直しでは消えない)。端末も使う。`y` の文 → `c` で消え、入力中のバーは `enter 置く  esc やめる  shift+enter 改行` だけ (`page.EditorHints`)。置いた文 → 次の `j` で消える |
| 19 | 直った | 帯は 316 に左寄せ (`text-align: start`)。バーは 0.55 に薄く、ボタンはすべて disabled、「s 送る」も太字でなくなる (実際にサーバーを止めて測った) |
| 20 | 直った | 390: 右端の語が 4 割に入らない並びは、その並びの全行で次の行に右寄せ (`rights-below`)。お知らせの時刻は 1 行目のまま (差 2px)。題の前は 62px |
| 21 | 直った | 間の token: `--gap-row` .25rem (行の間)、`--gap-open` .6rem (開いた中身のあと)、`--gap-head` 1.6rem (節・最上位 Fold の上)、`--gap-under-head` .4rem、`--gap-para` .3rem、`--gap-blank` .9rem (報告の空行)。重なりは `--z-band` 10 / `--z-banner` 20 / `--z-foot` 30 / `--z-veil` 90 / `--z-panel` 100 / `--z-dialog` 110 / `--z-image` 120 |
| 22 | 直った | 番号は 2.2em の列、文はその右にぶら下がる (2 行目も 763px)。`x 取り消す` / `u 元に戻す` (語は `page` の `WordRetract` `WordUndo`。端末の一覧・欄も同じ語が重なっていたので直した) |
| 23 | 変えない (指示どおり) | |
| 24 | 直った | 印は同じ箱 (左 294、中心 302.4 で ? ▸ · ✓ → すべて同じ) |
| h1 | もとから同じ | 端末: 空の節の「なし」と Fold の間、Fold と次の節の間、どちらも空行 1 つ (テスト `TestFoldAfterEmptySectionGap`)。web: 最上位 Fold は見出しと同じ `--gap-head` |
| h2 | 直った | 端末の Code の折り返しは、その行の字下げと diff の印のあとの列に続ける (テスト `TestCodeWrapHangs`) |
| h3 | 変えない (指示どおり) | |
| h4 | 直った | web もお知らせが先、数がその下。時刻は両方 `page.AtWord` (1280: `09:12`、別の日 `11/20 18:30`) |
| c | 直った | 下の C |

## C. 裏のサーバー

- 同時の `serve`: 状態ディレクトリの `serve.lock` を flock で取ってから探す・起動する。4 つ同時でも同じ URL (`TestServeConcurrent`。ロックを外すと落ちることを確かめた)
- 置き場が消えたボード: 見張りが外し (registry からも)、そのタブの SSE に `gone` を送る。呼び出しは 404 で「この pitboard は見つからなくなりました」。ページは帯にその文を出し、試し続けない (実機でセッションのディレクトリを動かして確かめた。`TestBoardGone`)。SSE が無くなればアイドルで止まる
- `serve.json` `boards.json`: `.tmp` を消してから O_EXCL・0600 で作り、権限を確かめてから置き換える (`web.WritePrivate`。古い 0644 の tmp があっても 0600: `TestStateFileMode` `TestWritePrivateMode`)
- `Start` が空の `serve.json` は、その pid が生きていてもサーバーと見なさない (`--stop` が他のプロセスを止めない: `TestServeStateNeedsStart`)
- `pitboard serve -h` は serve の使い方 (`TestServeHelp`)

## 推測で決めたこと

- コメント欄は WEB6 の右下の浮いた欄のまま (ブリーフの「85vh・画面いっぱい」は `?` と画像に。欄は 7 割で、390 では帯とバーの間いっぱい)。開いている間はページを薄く覆ってスクロールを止め、バーだけは欄の上に出す (390 でも `esc 閉じる` と「コメント N」を押せるように)
- 390 で右端の語を下に出すのは「同じ親の行の並び」単位。右端の語が無い行は 1 行のまま
- 帯の線は、お知らせがあるときだけ (数だけのときは線を引かない)
- 「(書いたときは: …)」はコメント欄では行の題 (`About`) に含まれるので、もう 1 行は出さない (前は 2 回出ていた)
- バーに入らない `u` `space` が、1280 でもカーソルの行によっては落ちる (端末と同じ順)。ギャラリーにはそれが見える変種 (箇条の行) を足した

## 確かめていないこと

- 本物の Chrome・Firefox・Safari (`field-sizing` が無いブラウザでは、入力欄は行数に合わせた `rows` で伸びる)、スクリーンリーダー、IME
- 5000 行の速さ: カーソルだけの移動は前と同じく 2 行とバーだけを描き直す (`render(…, 'cursor')`) が、測り直していない
