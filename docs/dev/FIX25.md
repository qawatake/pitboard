# FIX25: REVIEW16 findings

Read `docs/dev/REVIEW16-result.md`. Do after FEAT31 (same tree). Fix:

1. 中 FEAT29×FEAT30: while `Fixing`, the turn must not count the broken rewrite. `AITurn` uses the mtime of the last good version shown (keep it in `Report` when a good version is read), not the broken file's. SPEC 「誰の番か」: one sentence for this case.
2. 低 web: the turn word is always present in the Summary (page and board list), also when the report is missing or empty (`internal/page/page.go:216-226`, `internal/web/boards.go`), as SPEC:142 says.
3. 低 web: a late x refused by `Board.Hide` because the row is already hidden must not say 「報告が書き換わりました」 nor 「…もう一度押すと表示に戻ります」. Use a refusal of its own: 「この行はもう隠れています」 (word in `page`), on both screens if reachable.
4. 低 clocks: SPEC 「誰の番か」: one sentence that the turn compares the report file's modification time with the send time, so a file whose time is set back (rsync -t etc.) or a skewed clock can show the wrong turn. No code change.
5. TURN RULE — wait for the person's decision before touching it: the coordinator will add it to this brief if it changes.

Tests for 1-3 (page/viewer/web; e2e if cheap). No go.mod changes. No company or personal names.

`go vet ./... && go test -count=1 ./...` must pass (gallery browser step needs playwright-cli, absent: say so). No git. Report `docs/dev/FIX25-result.md` ≤8 lines. Reply on screen: one line.
