# REVIEW17 の結果: 89aa881 (FEAT31・FIX25・FIX26)

要約:
- a: REVIEW16 の指摘は 6 件とも直った。ただし 中 (Fixing の間の番) が直ったのは、開いていたページと端末だけ。一覧と、後から開いた画面は `あなたの番` を出す (中)
- b (FEAT31) と c (FIX26 の番の決まり) は、端末 (pty) と web (HTTP API) の両方で約束どおりに動き、その場で切り替わる。端の場合が 1 つある: 送った直しを undo すると、すぐ `あなたの番` になる (低)。d: SPEC・-h・SKILL はコードと合っている (`--skill` は SKILL.md と同じバイト列)
- `go vet` と `go test -count=1 ./...` は通る。4 つの変異 (FIX26 の `!carried`、`SettledState==""`→`!="done"`、Fixing 中も `Shown` を進める、`EnterStep` の畳み) はすべてテストで落ちた。web の JS (`S.toggle`) は node で直接試した

やり方: worktree を 89aa881 に detach し、`.rv/pb` をビルドした。初めから `PITBOARD_SEND=true` と `PATH=/usr/bin:/bin` (本物の herdr は無し)、`XDG_STATE_HOME=.rv/state`、`serve --port 0 --no-open` で動かした。web は `/s/<id>/api/{page,act,send}` と `/api/boards`、端末は Python の pty と、e2e の画面の真似 (使い捨ての e2e テスト。消した) で見た。最後に `serve --stop` を実行し、`ps` で `.rv/pb` が 0 件であることを確かめた。変異は元に戻した (`git status` は `.rv/` だけ。その後 `.rv/` も消した)。注意: `.rv/s1/.pitboard` を作る前に `path` を 1 回呼んだので、main checkout の `.pitboard/` に空の `rv1/` ができた。すぐ `rmdir` で消した (中は空。ほかには触っていない)

## a. REVIEW16 の指摘
- 中 Fixing で番が人に移る → 直った (ページのタブ・端末とも `AI の番` が続き、`AI が報告を直しています` が出る)。ただし下の 中 が残っている
- 低 報告が無い・空で番の語が無い → 直った (ページ・一覧とも語を出す)
- 低 遅れた x の言葉 → 直った (web: person・app とも「この行はもう隠れています」。2 回目も同じ。記録は増えない。`ShowHidden` のときは表示に戻す)
- 低 時計 → SPEC に 1 文足した (コードは前のまま) / 低 `settle --done` だけでは番が戻らない → FIX26 で直った / 低 `-count=1` → FIX25・FIX26 の依頼文は直った。FEAT31 の依頼文は `go test ./...` のまま (今回 `-count=1` で通ることは確かめた)

## 指摘
### 中
- 中 | Fixing の間、一覧と後から開いた画面が `あなたの番` を出す | 送る → `AI の番` → report.md を check で落ちる版にする (16 字を超える id)。開いていたタブと端末は `AI の番` + Fixing。ところが `/api/boards` の一覧は `あなたの番`。新しいタブ (別の `X-Pitboard-Tab`) と、新しく開いた端末の viewer は、誤りの一覧と `あなたの番` を出す (実測) | 原因: 一覧は毎回 `page.Open` (internal/web/boards.go:389)、新しい画面も `page.Open` から始まるので、前の良い版を知らない。そのため `Report.Shown` は壊れた版の mtime になる。直し方の案: 「check で落ちる版は書き換えと数えない」を、前の良い版を持っているかどうかによらない決まりにする。または一覧は `hosted.watch` の Board を使う

### 低
- 低 | 送った直しを undo すると、すぐ `あなたの番` | コメント 10 を直す → 送る → `changes` → `settle 10 --done` → web で undo (直しを取り消す) → 送る。AI には `edit thought 10: fourth` が届くが、番はすぐ `あなたの番` になる。コメントは前の `対応: handled edit` のまま `対応済` で、FEAT31 で畳まれる (実測) | 原因: `Replay` は undo した edit を飛ばすので、edit と違って settle を消さない (board/state.go:79-84)。FIX26 で、この穴が番の語にも出るようになった
- 低 | 報告を消すと `AI の番`、空にすると `あなたの番` | 送った後に report.md を消すと、`Shown` はゼロになり `AI の番` が続く。空のファイルなら `あなたの番` (実測)。SPEC は書いていない

## b・c で確かめたこと (抜き書き)
- b: 開くと、まだのもの (対応中・既読・未送信。書いた順) の後ろに `✎ 対応済 2 件` が出る (端末では薄い)。j で畳んだ行を飛ばし、次の行に移る。新しく `--done` が来ると畳んだまま `3 件` になる。enter を押すたびに 閉→開(畳む)→出す→閉→開(畳む) と回る。案内は畳んでいる間 `enter 開く`、出している間 `閉じる`。`対応済` の無い行は前と同じ。Ask は畳んだまま `y` で答えが記録される。web の `Things[].Comments` の順と `DoneFold` も同じ。`S.toggle` も同じ順に回り、`facts().Folded` はそれに合う。クリックの `showDone` も効く。一覧 (`Page.Comments`) の順は前と同じ
- c: 2 件送る → 1 件 `--done` では `AI の番` → もう 1 件を `--hold` で `あなたの番` → `--doing` で `AI の番` → `--done` で `あなたの番`。回答だけを送ると、`changes` の後も `AI の番` で、書き換えると `あなたの番`。取り消しだけの送りも同じ。何も無いのに `s` を押すと「未送信のものはありません」で、記録は増えず番も変わらない。ページ・タブ 2 つ・一覧・端末は、どれもその場で切り替わった

## 確かめていないこと
- `playwright-cli` が無いので、ブラウザでの描き方は見ていない (畳んだ 1 行の薄さ・クリック・`dfoldbtn` の位置、ギャラリーの新しい違い 2 つ、`.normal`/`quiet` の色)。web は HTTP API と node での `state.js` だけで見た
- 端末で遅れた x の拒否の言葉 (カーソルが隠れた行に乗らないので、届かせる手が無い)。時計のずれと、mtime の粗いファイルシステム

VERDICT: NOT LGTM (中: Fixing の間、一覧と後から開いた画面が `あなたの番` を出す)
