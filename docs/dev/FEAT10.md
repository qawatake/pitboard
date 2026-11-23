# pitboard: 起こす文を短く、画像を貼れるようにする

仕様: `docs/spec/SPEC.md`。FEAT9 が終わってから取りかかること。終わったら仕様書・`-h`・`?` の一覧も合わせる。

## 1. AI を起こす文を短くする (コンテキストの節約)

いま端末へ打ち込む文は
`pitboard: the person sent 4 item(s) from the board. Run "pitboard -s 38d12f95-0b22-4d64-88fb-17ed69d428cb changes" and act on them.`
で、送るたびに AI の会話に残る。さらに AI は `changes` を呼ぶので、1 回の送信で「文 + ツール呼び出し + 結果」の 3 つが積もる。

- 打ち込む文を短くする: `pitboard: 4 sent. Run: pitboard -s 38d12f95 changes`
  - セッション id は、その板のディレクトリの中で一意になる最短の先頭 (最低 8 字)
  - 意味は `-h` に書く (「`pitboard: N sent.` で始まる行が来たら、書かれたコマンドを実行して対応する」)。skill の description の「`pitboard:` で始まる行」はそのまま
- `changes` の出力も詰める:
  - `hide:` / `unhide:` は 1 行にまとめる: `hide (5): 題1 / 題2 / …` (題は 20 字で切る)
  - 同じ行への `y` を押し直した・答えを選び直したときは、最後のものだけ出す
  - 思いつきの紐づけ先の題は 30 字で切る
- `settle` を 1 回の呼び出しで複数書けるようにする: `pitboard -s S settle 50 "…" 51 "…" 59 "…"` (番号と文の組を並べる。今までの 1 組の形もそのまま動く)
- `pitboard wait` (FEAT7) の出力は、この詰めた `changes` と同じ。こちらは起こす文もツール呼び出しも要らないので、いちばん安い。`-h` にそう書く

## 2. 思いつきに画像を貼る

ユーザーの要望: Claude Code の入力欄のように、画像を貼ると `[Image #12]` と入り、AI がその画像を見られるようにしたい。

- エディタで `ctrl+v`: クリップボードに画像があれば、`.pitboard/<session>/images/<通し番号>.png` に保存し、カーソルの位置に `[Image #N]` を入れる。
  N はそのセッションの画像の通し番号 (1 から。追記だけ。消しても再利用しない)
- 画像の取り出しは外部コマンドで行う (依存を増やさない):
  - macOS: `osascript` で、クリップボードの PNG (`«class PNGf»`) をファイルへ書き出す。PNG が無ければ TIFF (`«class TIFF»`) を取り出して `sips` で PNG に直す
  - Linux: `wl-paste --type image/png`、無ければ `xclip -selection clipboard -t image/png -o`
  - どれも無い・画像が無いときは、下の行に「クリップボードに画像がありません」と出して何も入れない
  - 3 秒で打ち切る。20 MiB を超える画像は保存せず、その旨を出す
- 端末からの貼り付け (bracketed paste) で入ってきた文が、存在する画像ファイル (`.png` `.jpg` `.jpeg` `.gif` `.webp`) のパス 1 つだけだったとき (ファイルを端末へドラッグするとこうなる) も、
  同じように `images/` へコピーして `[Image #N]` を入れる
- `[Image #N]` は文の中のただの文字。消せば、その画像は使われない (ファイルは残す)
- 画面では、`[Image #N]` を薄い色にする (エディタの中、思いつきの欄、一覧)
- AI への届き方: `changes` / `wait` の出力で、その思いつきの行の下に、文に残っている画像のパスを 1 行ずつ添える:
  `  image 12: /abs/path/.pitboard/<session>/images/12.png` (AI はこのパスを Read で開いて見る)。`-h` に書く
- 保存するファイルの権限は 0600、`images/` は 0700
- 単体テスト: 通し番号、文に残っている画像だけがパスつきで届くこと、パス 1 つの貼り付け、大きすぎる画像。クリップボードの取り出しは、外部コマンドを差し替えられる形にして代役で試す

## 終わったら

vet / test / build。2 は pty 越しに、パスの貼り付けの経路で確かめる (クリップボードには触らない)。
**実物の `herdr agent prompt` と `herdr pane send-*` は実行しない**。`~/.claude/` には書かない。git は commit も push もしない。
報告は `docs/dev/FEAT10-result.md` に: 項目ごとに 1 行と、推測で決めた点だけ。画面への返事は「書きました」の 1 行。
