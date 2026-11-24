# REVIEW19 の結果: 93ce157 (FIX28)

要約: report.md が `check` で落ちる・無いときは、どの画面も `AI の番`。直せば全部そろって戻る。REVIEW18 の 中・低は直った。`go vet ./...` と `go test -count=1 ./...` は通る

やり方: worktree を 93ce157 に detach し `.rv/pb` をビルド。先に `.rv/s1/.pitboard` を作り、そこを cwd にした。`PITBOARD_SEND=true`・`PATH=/usr/bin:/bin`・`XDG_STATE_HOME=.rv/state` (絶対パス)・`serve --port 0 --no-open`。web は `/s/rv1/api/{page,act,send}` (開いたままのタブ A と毎回新しいタブ) と `/api/boards`、端末は `go test` の viewer テスト。変異 2 つを試して戻した: `AITurn` の `Broken()` 判定を外す → page/viewer/web の 9 テストが落ちる。`Broken()` から `Missing` を外す → `TestTurnBrokenOrMissing`・`TestTurnWithoutReportWeb`・ギャラリーが落ちる。最後に `serve --stop`、`ps` で `.rv/pb` 0 件、`.rv/` を消し、`git status` は空

## a. 壊れた・無い報告 (実測。タブ A / 新しいタブ / 一覧で、どの場合も 3 つとも同じ語)
- 一度も送らずに壊す: `AI の番` → 直す: `あなたの番`。報告がまだ無い新しいボード: `AI の番`
- 送る (`AI の番`) → 良い版で答える (`あなたの番`) → `changes` → 壊す: `AI の番` → 直す: `あなたの番`
- 送る → 壊す (答えないまま): `AI の番` → 直す: `あなたの番`
- 送る → 全部 `settle --done` (`あなたの番`) → 壊す: `AI の番` → 直す: `あなたの番`
- 答えたあとに消す: `AI の番`。空にする: `あなたの番`
- 端末 (開いていた viewer と新しい viewer): `TestTurnAnsweredThenBrokenViewer` が答えたあとに壊す場合を見る。消した場合の viewer は、同じ `page.Board.AITurn` を通る `page` のテストだけが見ている

## b. 回帰
- FEAT29: 壊れた版の間、タブ A は `Fixing=AI が報告を直しています` で良い版を出し、右上は `Tone=quiet`。新しいタブは誤りの一覧を出す (前と同じ)。FEAT31: 1 行に 2 件を `--done` → `DoneFold=対応済 2 件`。FIX26: `--doing` の間は書き換えても `AI の番`、`--done` で `あなたの番`。x: 2 回目は「この行はもう隠れています」で `hide` は 1 件のまま

## c. 文書
- SPEC「誰の番か」(0)、`-h`、`--skill`/SKILL.md はコードと合う。`Report.Shown` も「書き換えと数えない」も、もう出てこない (guard.go の `Touch.Shown` は別物)

## 指摘
- 低 | viewer で「報告を消した」場合の番を見るテストが無い (web と page にはある) | 判定は共通の `AITurn` なので実害は小さい
- 確かめていないこと: `playwright-cli` が無いのでブラウザの描画。端末の pty での実機操作

VERDICT: LGTM
