# FEAT29 result

- `page.Report.Load`: once a version without problems (and not empty) was read, a version with problems keeps that version's nodes, leaves `Probs` empty and sets `Fixing`. If no good version exists yet (started broken, first report broken, or the previous one was empty or removed), the problems show as before.
- `page.MsgFixing` = `AI が報告を直しています`, passed on as `Page.Fixing`. The terminal draws it dim at the head of the report. The web draws it as `p.msg.fixing` (quiet) in `banners.js` `head`. It is not a Notice and not yellow.
- Guard choice: allow when the row is identical in the broken file. Actions on the shown good version still check the row against the file on disk (`fresh`). A row that is the same there is recorded, because the record is true of both versions. A row that was rewritten is refused with `MsgChanged`. `guard.go` and `ops.go` are unchanged. The cursor is not locked, because the shown nodes do not change.
- wait/changes/settle already ended with `ProblemNote`, and `TestCommandsTellProblems` and `TestWaitTellsProblems` cover it. No change was needed.
- `-h` (Manners, check bullet), SKILL.md and skill.go each get one sentence. SPEC.md gets a paragraph after the problem-note rule, and the Notice bullet mentions the quiet line.
- Tests: `internal/page/fixing_test.go`, `internal/viewer/fixing_test.go`, `internal/web/fixing_test.go`. `e2e` `TestBrokenReport` was rewritten: it starts broken, gets fixed, breaks again (quiet dim line, good version kept), then gets fixed again. In `TestCommentAnswersAsk`, the added option now uses key `d`, because `c` is a check problem.
- There is no gallery variant for the quiet line: a live server started on a broken report has no good version, so the live-fidelity check could not match it.
- `go vet ./... && go test ./...` passes. The gallery step of `scripts/check.sh` was not run, because playwright-cli is not installed here.
