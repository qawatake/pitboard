# FIX24: あなたの番 in the normal color

Person's decision: the top-right `あなたの番` (FEAT30) is shown in the normal foreground color, not yellow; yellow stays with 「反応待ち N」 (and 未送信 N). `AI の番` stays faint.

Change the tone in `internal/page/web.go` `Board.Summary` (use the palette's normal tone; add one to `internal/page/palette.go` only if none exists), both screens follow from it. Update SPEC.md's wording of the summary colors, the palette table/comment, and the tests/e2e color assertions and gallery fixtures (`-update`) that expect yellow.

`go vet ./... && go test ./...` must pass (gallery browser step needs playwright-cli, absent: say so). No git. No company or personal names. Report `docs/dev/FIX24-result.md` ≤5 lines. Reply on screen: one line.
