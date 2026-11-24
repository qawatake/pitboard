# FIX27 の結果

1. `page.Report.Load` は `check` で落ちる版を読んでも `Shown` を進めない (前の良い版の有無によらない)。一覧・新しいタブ・新しく起こした端末も、送ったあとに落ちる版へ書き換わっても `AI の番`。開いていた画面の動き (FIX25) は同じ。テスト: `page.TestTurnFreshOnBrokenReport`、`web.TestTurnOnBrokenReportWeb` (一覧と新しいタブ)、`viewer.TestTurnFreshViewerOnBrokenReport`。直しを戻すと page と viewer と一覧のテストは落ちる
2. SPEC「誰の番か」に、送ったあとに消すのは書き換えでない (`AI の番`)・空にするのは書き換え (`あなたの番`) と書いた。どの画面でも同じ値になるので、コードは変えていない。3. SPEC に既知の限界の節が無いので何も書いていない
- `go vet ./... && go test -count=1 ./...` は通る。`playwright-cli` が無いので、ギャラリーのブラウザの手順は走らせていない
