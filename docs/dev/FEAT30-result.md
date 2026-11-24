# FEAT30 result

- Rule: `page.Board.AITurn` (internal/page/board.go). AI の番 when a thought is still `doing`, or when report.md's mtime is not after the last `send` event's `t`; otherwise あなたの番 (also when never sent). `Board.Load` keeps `SentAt` from the last send.
- `send` events now carry `t` to the nanosecond (`time.RFC3339Nano`, internal/page/send.go) so a rewrite in the same second as the send is told apart. Older sends (seconds) still parse.
- Summary (internal/page/web.go): the turn word first, always one of `AI の番` / `あなたの番` (words in internal/page/words.go); `送信済 N` removed; the rest unchanged.
- Tones: `AI の番` quiet (faint), as the palette's "薄い: AI 待ち" — nothing for the person to do, so it should not pull the eye. `あなたの番` need (yellow), as "黄: あなたの手が要る" — the flip back to the person is what they want to notice.
- Live: report rewrites (viewer tickReload / web Changed reload the report, updating its mtime) and settles (events reload) both flip it; covered in tests.
- Docs: SPEC.md (new 「誰の番か」, summary lines), `-h` Thoughts section, SKILL.md + skill.go (one sentence each).
- Tests: internal/page/turn_test.go (both conditions and their combination), viewer bottom_test/draw_test, web web_test, e2e TestSend (colors), TestJump (the top row is now always taken), gallery fixtures regenerated (`-update`).
- `go vet ./... && go test ./...` pass. The gallery browser step (scripts/web-gallery.sh) was not run: playwright-cli is not installed.
