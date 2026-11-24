# FIX26: settling everything sent gives the turn back

Person's decision (REVIEW16 低 "settle --done alone does not bring the turn back"): condition 1 of the turn rule counts only while the last send's thoughts are not all settled.

New rule — **AI の番** while either holds, otherwise **あなたの番**:
1. the person sent, the version on screen was not rewritten since the last send (as FIX25: `Report.Shown`), AND at least one thought delivered by that last send is not yet settled (no settle at all; a settle with --doing counts as not done by rule 2 anyway);
2. a thought is still --doing.

So: send → settle --done (or --hold) of every thought of that send, no rewrite → あなたの番. A send that carried only answers/hides (no thoughts)? Answers to an Ask are not settled by the AI: treat a send with no thoughts as needing a rewrite (rule 1 as before, without the settle clause). Say what you did.

Update `page.Board.AITurn` (`internal/page/board.go`), SPEC 「誰の番か」, `-h` and SKILL sentence (`cmd/pitboard/usage.go`, `cmd/pitboard/skill.go`, `skills/pitboard/SKILL.md`): settling every thought also gives the turn back. Tests in page (+ viewer/web if cheap). No go.mod changes. No company or personal names.

`go vet ./... && go test -count=1 ./...` must pass (gallery browser step needs playwright-cli, absent: say so). No git. Report `docs/dev/FIX26-result.md` ≤6 lines. Reply on screen: one line.
