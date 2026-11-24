# FIX27: REVIEW17 findings

Read `docs/dev/REVIEW17-result.md`.

1. 中: while the report on disk fails check, every screen — the board list (`internal/web/boards.go:389`, `page.Open` each time), a newly opened web tab, a newly started terminal viewer — must say `AI の番` exactly as the already-open screens do. Make the rule not depend on having seen a good version: a version on disk that fails check never counts as "rewritten since the last send" (whether or not a good version is held). Keep FIX25's behaviour for open screens. Tests: board list, fresh `page.Open` on a broken report after a send, fresh terminal viewer.
2. 低 report removed vs emptied after a send: make both cases agree with what a person expects: a deleted report.md is not a rewrite (AI の番 stays, as today), an emptied one is a rewrite (as today) — just write both in SPEC 「誰の番か」 so it is defined. No code change unless they are inconsistent across screens.
3. 低 undo of an edit after it was settled: not in scope; leave a one-line note in SPEC's known limits only if such a section exists, else nothing.

`go vet ./... && go test -count=1 ./...` must pass (gallery browser step needs playwright-cli, absent: say so). No go.mod changes. No company or personal names. No git. Report `docs/dev/FIX27-result.md` ≤6 lines. Reply on screen: one line.
