# FIX28 の結果

1. `page.Report.Broken()` (いま読んだファイルが `check` で落ちる (Fixing か誤りを出している)・または無い) を足し、`AITurn` はこれが真なら、ほかの条件を見ずに `AI の番` にする。書き換えの比べはファイルの `Report.Mtime` で行う。FIX25/FIX27 の `Report.Shown` は消した。どの画面もディスクのファイルで決めるので、開いていた画面と新しい画面・一覧が同じ語を出す (REVIEW18 の 中・低)。空の report.md は今までどおり書き換え
2. SPEC「誰の番か」、`-h` (usage.go)、skill.go と SKILL.md に 1 文ずつ (`--skill` と SKILL.md は同じバイト列)。ギャラリーの fixtures.json を -update した (壊れた・無い報告の 3 か所が `AI の番`)。`web.TestTurnWithoutReportWeb` の missing は `AI の番` に直した
3. テスト: `page.TestTurnBrokenOrMissing` (開いていた board と新しい board。送る前・答えたあと・全部 settle したあとに壊す、消す、空)、`web.TestTurnAnsweredThenBrokenWeb` (開いていたタブ・新しいタブ・一覧)、`viewer.TestTurnAnsweredThenBrokenViewer` (開いていた端末と新しい端末)
- `go vet ./... && go test -count=1 ./...` は通る。`playwright-cli` が無いので、ギャラリーのブラウザの手順は走らせていない
