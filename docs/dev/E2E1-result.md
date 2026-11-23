# E2E1 の結果

PR: https://github.com/qawatake/pitboard/pull/1 (ブランチ `e2e-tests`、`origin/main` の `798f94a` に rebase 済み。merge はしていない)。
`go test ./... -count=3` を繰り返し通過 (e2e は 1 回 約 8 秒、`-parallel 32` でも通る)。非テストのコードは触っていない (README に実行方法 1 行のみ)。

## 形

- `e2e/` (package `e2e_test`)。`TestMain` で `./cmd/pitboard` を 1 回ビルドし、CLI と pty だけで動かす。`internal/...` は import しない。標準ライブラリのみ
- pty は syscall の ioctl で darwin / linux (build tag)。ほかの OS は viewer のテストを skip。linux / freebsd / windows でコンパイルは確認、実行は darwin のみ (手元に Linux が無い)
- 画面のモデル (`screen_test.go`): カーソル移動・行/画面の消去・SGR の色・OSC 8・全角 2 桁。`v.Keys` `v.Type` `v.Paste` `v.WaitFor` `v.Line` `v.ColorAt` `v.MoveTo` `v.Sync` (? を開閉して鍵の処理を待つ)。失敗時は画面を出す
- 揺れ対策: 待ちはすべて期限つきの polling (sleep で同期しない)。0.4 秒の守りは「記録が出るまで押し直す」で通す。日付は過去の固定値、`open` の経過時間は単位だけ比べる。HOME・PATH (stty ps sh などのリンクだけ。herdr・osascript なし)・TZ をテストごとに分離

## 数 (トップレベル / サブテスト込み: 54 / 185)

- a. CLI (path・check の全種類・changes・settle・open・wait・hook・セッション・-h・--skill): 14 / 91
- b. 描き方 (部品・文・折り返し・幅 40/80/120・CJK): 6 / 29
- c. 反応 (y n・答え・c m e x H r A u s・守り・コメントの状態・画像): 16 / 37
- d. 移動 (j k・高い中身・space・g・欄・tab・?): 7 / 12
- e. 印と数: 2 / 2
- f. 再読み込み・起動時の状態: 4 / 9
- g. 頑丈さ (20x5・resize・3000 行・壊れた events 行・SIGTERM): 5 / 5

## spec と動作のずれ

- h. SPEC.md 内の食い違い: 「コメント」は AI の対応の文を「行の下と欄では状態の語だけ」とし、「画面」は「行の下のコメントに AI の文があれば `対応: …`」とする。動作は後者 (行を開くと `対応: やりました`)。テストは後者で固定し、両方の文を注記した。どちらに揃えるか決めてほしい
- i. 作業中に main が 2 度動いた (FIX13/14 で `re` 廃止・`g` の字・Notice の縮め・`at` の 0 埋め、FIX15 で placed の `x` `r`・効かない理由の知らせ)。どれも spec の変更なので、テストを新しい spec に合わせた。rebase 前に見つけた「幅 20 で Notice の題が `…` で切れ、`at` が 1 字ずつ折り返り、20x5 で入力の行が見えない」は FIX14 の spec と動作で解消済み (今は spec どおり)

## skip

- なし (pty の無い OS での viewer テストだけ自動で skip)
