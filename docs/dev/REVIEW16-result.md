# REVIEW16 の結果: 1631c32 (FIX22・FEAT29・FEAT30・FIX23・FIX24)

要約:
- a (x) と FIX23 は端末・web とも約束どおり。FIX22 の `Hid` の決まりは残っていない (`Hid` `hidTo` `"hid"` `hiding` は 0 件)。`go vet` と `go test -count=1 ./...` は通る。3 つの変異はすべてテストで落ちた
- b (FEAT29) は約束どおり動く。良い版を残す。消えた行の警告では出ない。同じ行なら記録し、違う行なら断る。空・消えた後は誤りを出す。wait・changes・settle は注記を出す。ただし c と食い違う: 壊れた版を書いた時点で `あなたの番` に変わる (中)
- c (FEAT30・FIX24) は端末・web ともその場で切り替わる。色は `AI の番` が `\e[2m`/quiet、`あなたの番` が色なし/normal。`送信済 N` は SPEC・-h・SKILL・web から消えた (`送信済` はコメントの状態の語としてだけ残る)。報告が無い・空のときは web に番の語が出ない (低)

やり方: worktree を 1631c32 に detach し、`.rv/pb` をビルドした。スクラッチのボード (`.rv/s1/.pitboard/rv1,rv3,rv4,rv5`)、`XDG_STATE_HOME=.rv/state`、`serve --port 0 --no-open` を使い、web は HTTP API (`/s/<id>/api/page` `act` `send`)、端末は Python の pty で操作した。最初の `send` 1 回は PATH にある本物の `herdr agent list` を呼んだ (読むだけ。AI はいないと返った)。それ以降は `PITBOARD_SEND=true` と `PATH=/usr/bin:/bin` にした。終わりに `serve --stop` を実行し、`ps -ax | grep .rv/pb` で 0 件を確かめた。変異はすべて元に戻した (`git status` は `.rv/` だけ。`.rv/` は消した)。

## 指摘

### 中
- 中 | FEAT29×FEAT30: 壊れた書き換えで `あなたの番` に変わる | 送る → `AI の番(quiet)` になる → report.md を check で落ちる版 (16 字を超える id) に書き換える → web の Summary は `あなたの番(normal)` + `Fixing=AI が報告を直しています`。端末も同じ (`AITurn` は `Report.Mtime` を見ており、`Report.Load` は壊れた版でも Mtime を進める。internal/page/report.go:46 と board.go:82) | 期待: 人に見えているのは送る前の良い版のままで、AI は直している途中なので `AI の番` のまま。実際: 人の番に見え、答えが来たのに中身が変わらないように見える。Fixing の間は「書き換わった」と数えない (良い版を読んだ時の mtime を使う) のが筋。SPEC の「誰の番か」もこの場合を書いていない

### 低
- 低 | web: 報告が無い・空のとき番の語が無い | 送る → report.md を消す (または空にする) → web の Summary は null (page.go:216-226 で `Summary` を詰める前に return する)。一覧 (boards.go) も同じ。端末の右上には出る (`sendMark` はいつも `Board.Summary`) | SPEC:142 は「どちらか 1 つを必ず出す」と書いている。画面どうしも SPEC とも食い違う
- 低 | FEAT30: 時計に頼る端の場合 | (1) 書き換えた後に mtime を 1 時間戻す (`rsync -t`、mtime を保つエディタ) と、`AI の番` に戻る (実測)。(2) 秒の精度で書かれた古い `send` は秒に切り捨てられる。同じ秒の中で送る前に書いた報告でも「後」と数え、`あなたの番` になる (コードで確認)。(3) 送るのはサーバーまたは viewer の時計、mtime はファイルシステムの時計。網の上のファイルシステムやずれた時計では食い違う (未確認) | 実害は小さい。SPEC に一言あるとよい
- 低 | FEAT30: `settle --done` だけでは番が戻らない | 送る → 報告は書き換えずに `settle N --done` (チャットで答えた) → `AI の番` のまま。次に書き換えるまで続く (実測。`--hold` も同じ) | 決めた決まりどおりで、SKILL/-h もそう書いている。チャットだけで答える AI にはボードが嘘をつく。決まりを変えるかを決めてほしい
- 低 | web: 隠した行への遅れて届く x の言葉 | `Board.Hide` の拒否は効く: 隠した b に古い token で x (person) → `Changed: true`「報告が書き換わりました。もう一度押してください」。報告は変わっていないので、言葉が事実と違う。`Cur.How=app` で届くと「…もう一度押すと表示に戻ります」と返すが、もう一度押しても同じ拒否になる (コードで確認)。キューがあるので普通は届かない | 拒否のときの言葉を別にする (例:「隠した行です」)
- 低 | 手順: ビルダーの「通った」は cache を含むおそれ | FEAT29/30・FIX23/24 の依頼文は `go test ./...` (`-count=1` なし)。e2e は TestMain で `go build` するので、cache は internal の変更を見ない。guard を変異させても `e2e (cached) ok` と出た。`-count=1` にすると TestHide などが落ちた | 依頼文も `scripts/check.sh` と同じく `-count=1` にする

## 確かめたこと (抜き書き)
- a: 端末 (pty) で `j k x x` → 1 行だけ隠れ「いま移った行です。もう一度押すと隠れます」が出る。もう一度 `x` → b が隠れる。0.6 秒後の `x` → c が隠れる。`u` `u` → 1 つずつ戻る。web で app/50ms の x → 断る、同じ x → 隠れる、app/500ms → 隠れる、undo → charlie、bravo の順に戻る。変異: 設定中の drop を外すと guard・viewer・web・e2e が落ちた。Hide の「隠し中は戻さない」を外すと `TestHideRowsInARowWeb` が落ちた
- b: 壊れた版で開くと誤りを出す → 直すと消える → 再び壊すと端末は `\e[2m` で 1 行出し、誤りは出さない。空・消えた後の壊れた版は誤りを出す。閉じていない `<Item` (ReadErr 側) も Fixing になる。答えた Ask を消しても Fixing にならない (`check` は終了コード 0)。`wait` は始めに stderr、終わりに stdout へ注記を出す (2 回出るのは前からの仕様)。settle は成功しても失敗しても注記を出す
- c: 送る・`--doing`・`--hold`・2 回送る・送った直後 (<1ms) の書き換え (APFS は ns なので `あなたの番`)。端末・web とも書き換え・settle でその場で切り替わる
- d: SPEC:31/101、-h、SKILL、skill.go はコードと合っている。FIX22 の決まりを書いた所は無い。食い違いは上の「番の語が無い」と、SPEC が Fixing 中の番を書いていないことだけ

## 確かめていないこと
- `playwright-cli` が無いので、ブラウザでの描き方は見ていない (静かな 1 行の位置と色、`.normal` の色、キーを押し続けたときのキューの実際のタイミング、ギャラリーの手順)。web は HTTP API で、端末は生のエスケープ列で見た
- HFS+/FAT/網の上のファイルシステム (mtime の精度が粗い) と、時計のずれ
