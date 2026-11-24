# FIX26 の結果

1. `page.Board` に `SentThoughts` (いちばん新しい `send` が届けたコメント: 前の `send` から後に書いた・直した・取り消しを戻した・undo で変えたもの) を足した。`AITurn` の (1) は、書き換えが無く、かつその中に `settle` の無いものが 1 つでもあるとき。全部 `--done` / `--hold` なら `あなたの番`
2. コメントを含まない `send` (回答・隠すだけ) は AI が `settle` できないので、従来どおり報告の書き換えまで `AI の番`
3. SPEC「誰の番か」、`-h` (usage.go)、skill.go と SKILL.md の 1 文を直した。テスト: `page.TestTurnSettledSend`
- `go vet ./... && go test -count=1 ./...` は通る。`playwright-cli` が無いので、ギャラリーのブラウザの手順は走らせていない
