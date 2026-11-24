# FEAT30: whose turn it is (AI の番 / あなたの番)

Person's request: after pressing s, the person cannot tell whether what they sent has not arrived, is being worked on, or has been answered. Counts do not matter; they want 「あなたの番」「AI の番」. Their words: 「こちらが送信してから書き込みがない間は絶対に AI の番にしたい。さらに、まだ回答途中があればそれも AI の番にはしたい」.

Decided rule — **AI の番** while either holds, otherwise **あなたの番**:
1. the person sent (s) and report.md has not been rewritten since the last send (compare report.md's mtime with the time of the last send event; a send counts even if the AI has not read it yet);
2. a thought the AI settled as doing (`settle --doing`) is still doing.

Display (terminal viewer and web, words in `internal/page`, `Board.Summary` in `internal/page/web.go:88`):
- Show exactly one of `AI の番` / `あなたの番` in the top-right summary. `送信済 N` goes (this replaces it); keep `反応待ち N`, `未読 N`, `AI 待機中`, `未送信 N` as they are. Order: put the turn word first.
- Tone: pick from the existing palette (`internal/page/palette.go`), not yellow for AI の番; say what you picked and why in the result.
- A board where nothing was ever sent: `あなたの番`.
- It must update live: the viewer already reloads on report/record changes (`Board.Changed`); make sure a settle and a report rewrite both flip it.

Docs: SPEC.md (where the summary is described), `-h` and SKILL (`cmd/pitboard/usage.go`, `cmd/pitboard/skill.go`, `skills/pitboard/SKILL.md`): one sentence telling the AI that the board shows AI の番 until it rewrites the report after a send or while a thought is --doing, so settle --doing right away for long work and --done when finished.

Tests: page (the rule, incl. both conditions and their combination), terminal, web, e2e if cheap. Do not touch guard rules. No go.mod changes. No company or personal names.

`go vet ./... && go test ./...` must pass (gallery step needs playwright-cli, absent: say so; if the gallery has a summary variant, update its expected text). No git. Report `docs/dev/FEAT30-result.md` ≤12 lines. Reply on screen: one line.
