# FEAT12 の結果: `cmd/pitboard` と `internal/` に分けた

挙動は変えていない。vet / test / build は通る。git には触っていない (コミットしていないので、移動は「消えた + 増えた」に見える)。

## できた構造 (行数)

```
cmd/pitboard/                     (package main)
  main.go        279  main、run (コマンドの振り分け、-s)、settleArgs、runWait、exitError
  usage.go       404  -h の全文
  skill.go        26  SKILL.md と同じ文の定数 (REVIEW6 2-d の案 A)
  hook.go         94  pitboard hook
  main_test.go 225 / hook_test.go 240 / skill_test.go 37 / usage_test.go 17 / helpers_test.go 47
internal/report/                  報告の木と解析。ファイルも端末も触らない
  node.go        243  (旧 page.go) Node、Kind、HideKey、ByID、WalkAll、PlainOf
  parse.go       643  (旧 report.go) Parse、Problem、ViewerKeys
  node_test.go 61 / parse_test.go 428 (+ Fuzz)
internal/board/                   .pitboard/<session>/ の記録
  find.go        183  Store、Open、Sessions、FindSession、Root、ShortSession
  events.go      255  Event、追記、ロック、Seen、undo の解き方
  changes.go     325  Changes、Pending、deliverable、describe、Settle、Unsent、SentUnread
  state.go       157  Replay、State、Thought、Status (列挙)
  images.go      177  画像の保存と番号、ImageFileOf
  viewed.go       75  viewed ファイル (HashKey)
  wait.go        191  Store.Wait、Waiter
  lint.go         41  ProblemNote
  changes_test.go 350 / events_test.go 30 / find_test.go 61 / images_test.go 43 /
  state_test.go 16 / wait_test.go 136 / helpers_test.go 70
internal/viewer/                  画面。公開は Run だけ
  app.go         365  app、view、Run、load、loop、reload
  keys.go        520  handle / handleList / handlePane / react / hide / retract / undo
  cursor.go      216  カーソルの移動と止まる所
  input.go       406  エディタの入力、外部エディタ、エディタの描画
  editor.go      234  edit (そのまま)
  wake.go        168  wake、deliver、herdrPane (旧 send.go の残り)
  draw.go        385  draw、compose、line、titled
  rows.go        425  rows、right、markOf、contents、notice
  bottom.go      468  案内の行、? の一覧、知らせ
  pane.go        232  下の欄、コメントの一覧
  marks.go       348  ● の印 (waiting / unread)、viewKey
  clipboard.go   176  ctrl+v の画像、withImages
  words.go        53  画面の日本語 (知らせ、状態語)
  styled.go      205  (旧 text.go) 装飾付きの文字列
  term.go        547  raw モード、キー、幅
  *_test.go: bottom 319 / clipboard 87 / cursor 206 / draw 302 / editor 200 / helpers 121 /
             hide 212 / input 255 / marks 308 / pane 317 / rows 273 / styled 53 / term 153 /
             undo 190 / wake 299
```

依存 (`go list` で確かめた): `cmd/pitboard` → `viewer` `board` `report`、`viewer` → `board` `report`、`board` → `report`、`report` → なし。

## テストの関数の数

- 前: 136 (Test 135 + Fuzz 1、すべて直下の package main)
- 後: 138 = cmd 19 + report 18 + board 22 + viewer 79
- 増えた 2 つ:
  - `TestSkillMatchesSKILLmd` (cmd): skill.go の定数と `skills/pitboard/SKILL.md` が同じか。REVIEW6 は `TestSkillIsThin` に足す案だったが、失敗の意味が別なので分けた
  - `TestShortSession` (board): 旧 `TestWakeMessageIsShort` の後半 (短い id、それが session に解決されること) を board へ移した。viewer に残した前半は wake の文だけを確かめる。確かめていることは減っていない
- 中身を変えたテスト (確かめていることは同じ):
  - 状態の語を見ていた board のテスト (`TestChangesAndSettle` `TestChangesStopsAtTheLatestSend` `TestEdit*` `TestSettleStates`) は、語の代わりに `Status` の列挙と比べる。語への変換は viewer のテスト (`TestInputStateOnTheLine` など、画面の語を見るもの) が見る
  - `TestWaitWaitsForASend` (viewer) は board の `waitPoll` を短くしなくなった (公開しないため)。待つ間隔が 300ms になるだけで、期限 2 秒に収まる
  - `TestPasteImage` は `st.imagesDir()` の代わりに `filepath.Join(st.Dir, "images")` を読む

## 進め方

各段で `go vet ./... && go test ./... && go build ./...` を通した。

1. 同じ package main のまま、宣言単位でファイルを割った (`view.go` → app/keys/cursor/input/draw/rows/bottom/pane/words、`store.go` → find/events/changes/state、`send.go` → wake と changes、`marks.go` `images.go` の Store 側と画面側を分けた)。テストも関数単位で行き先の名前のファイルへ (`featN_test.go` `fixN_test.go` は無くなった)
2. 境界のずれを直した (下の「推測で決めた点」)
3. report → board → viewer の順に、型情報を使う小さな道具で「別パッケージから使われる名前だけ大文字にし、修飾子と import を付けて移す」をした
4. 残った main を `cmd/pitboard` へ移し、skill の埋め込みを定数に替えた
5. コメントの中の古い名前、公開した名前の doc コメントの頭を直した。README の `go install` と SPEC.md の組み込み skill の節を直した

## 確かめたこと

- vet / test / build、`go build ./cmd/pitboard`
- スクラッチパッドの空のディレクトリで、作った実行ファイルを pty 越しに動かした (PATH から herdr を外し、`PITBOARD_SEND` はファイルへの追記)
  - `path`、`check`
  - `wait --timeout 20s` を裏で動かしたまま画面を開き、`j` `k` `c` (書いて enter)、`y`、`s` → 画面に「送りました」「AI 待機中」、`wait` が thought と y を出して終わった
  - `wait` なしで `c` → `s` → `PITBOARD_SEND` に `pitboard: 1 sent. Run: pitboard -s s1 changes` が渡った。`changes` がその thought を出した
  - `settle --done 1 TEXT --hold 4 TEXT` が 2 件の settle を書いた (はじめ私が番号を 3 と取り違え、「no thought 3」で何も書かずに終わることも見えた)
- どの repo の `.pitboard/` も触っていない。herdr は実行していない

## 推測で決めた点

- **公開名**: `parseReport` → `report.Parse`、`newStore` → `board.Open`、`replayEvents` → `board.Replay`、`boardRoot` → `board.Root`、`runViewer` → `viewer.Run`、`kTitle` などの種類 → `report.KindTitle` など、`node.id` / `url` → `ID` / `URL`。ほかは頭を大文字にしただけ。`thought.sstate` は埋め込んだ `Event.State` とぶつかるので `SettledState`
- **Store のフィールド** (`Base` `Session` `Dir`) は公開フィールドにした (メソッドにはしていない)
- **状態語**: board は `Status` の列挙 (`StatusUnsent` `StatusSent` `StatusRead` `StatusDoing` `StatusHold` `StatusDone`) を返し、viewer の `words.go` の `statusWord` と `app.statusOf` が語と色にする。色の規則 (未送信だけ黄色、`sentWord` を使っていた所は常に dim) はそのまま
- **`runViewer` が返していた `exitError`** (cmd の型) は viewer から使えないので、`viewer.Run` は `(exit int, err error)` を返し、cmd が `exitError` にする
- **小さな重複**: `report` の `joinLines` が viewer の `runeWidth` を使っていたので、report に `wide` (幅 2 の範囲だけ同じ表) を持たせた。`modTime` (board と viewer) と `isDir` (board と viewer) も各パッケージに 1 つずつ持つ。汎用の小関数を公開するより、重複の方が境界がきれいと判断した
- `contains` は report に残し、board と viewer は `slices.Contains` にした
- `imageFileOf` (貼った文が画像ファイルのパスか) は対応する拡張子を知っている board に置いた (`board.ImageFileOf`)。クリップボードの取得 (`clipboardImage` など) は viewer
- `viewKey` (unread の鍵になる「行が言っていること」) は viewer、`HashKey` と viewed ファイルの読み書きは board。● の印の説明のコメントは viewer の `marks.go` へ移し、board の `viewed.go` には短い説明を書いた
- REVIEW6 2-f に従い、`term.go` の文字列用 `wrap` / `trunc` を `wrapPlain` / `truncPlain` にした (styled の `text.wrap` / `text.trunc` と区別)
- テストの共有の関数 (`testStore` `sendAll` など) は、REVIEW6 の勧めどおり各パッケージの `helpers_test.go` に同じものを持つ
- テストの組み分けは REVIEW6 2-e の表に沿い、表に無い後のテスト (FEAT10 以降) は確かめている物で決めた。`hide_test` `pane_test` `undo_test` `marks_test` `input_test` `rows_test` などを足した
- `parse.go` は 643 行で 600 行の目安を少し超えるが、旧 `report.go` そのままで割っていない (割る依頼は `view.go` だった)

## 見つけた別件 (直していない)

- board に画面の日本語が残っている: `ErrNoImage` (「クリップボードに画像がありません」)、`errTooLarge`、「画像の番号を決められませんでした」。いずれも viewer だけが画面に出す。列挙や英語の error にして viewer で語にするのが筋だが、今回は動かすだけにした
- `ErrNoImage` は「クリップボードに画像がない」の意味だが、board の `SaveImage` も空のデータに対して返している
- `report.Node.Part()` は Item / Ask / Link / Fold / Notice 以外の種類で `fmt.Sprint(n.Kind)` (数字) を返す。いまの呼び出し元では起きない組み合わせだが、気づいたので書いておく
- `go install github.com/qawatake/pitboard@latest` (古いパス) は、この版からは「not a main package」で失敗する。タグを切るならリリースノートに 1 行要る (REVIEW6 2-c)
- `docs/dev/` の扱い (REVIEW6 2-e) は指示どおり手を付けていない
