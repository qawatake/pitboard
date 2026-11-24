# FIX25 の結果 (項目 1-4。5 は未着手)

1. `Report.Shown` (画面に出ている版の mtime) を足し、Fixing の間は進めない。`AITurn` はこれと送った時刻を比べる。SPEC「誰の番か」に 1 文を足した。テスト: `page.TestTurnWhileFixing`
2. `Page` は報告が無い・空でも `Summary` (番の語) を詰める (一覧も同じ値を使う)。ギャラリーの fixtures.json を -update した。テスト: `page.TestTurnWithoutReport`、`web.TestTurnWithoutReportWeb`
3. `page.MsgAlreadyHidden`「この行はもう隠れています」と `ErrAlreadyHidden` を足した。`Board.Hide` はこれを返し、web の key x は Allow の前に同じ語で断る (person / app とも)。端末は `failed` がそのまま出す (隠れた行にはカーソルが乗らないので普段は届かない)。SPEC:31 の下に 1 文。テスト: `page.TestHideAlreadyHidden`、`web.TestLateHideOnHiddenWeb`
4. SPEC「誰の番か」に、mtime と送った時刻を比べるので mtime を戻すコピーやずれた時計で取り違えうる、と 1 文を足した (コードは変えていない)
- `go vet ./... && go test -count=1 ./...` は通る。`playwright-cli` が無いので、ギャラリーのブラウザの手順は走らせていない
