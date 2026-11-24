# FIX28: a broken or missing report is always AI の番

Read `docs/dev/REVIEW18-result.md`. Person's decision: while report.md fails check, or is missing, it is **AI の番** on every screen, whatever else holds (the AI owes a fix). Otherwise the rule stays (FIX26: sent, not rewritten since, some thought of the last send unsettled; or a thought --doing).

- Already-open screens must not use `Report.Shown` to decide this: they look at the file on disk (broken/missing → AI の番), same as fresh screens and the board list. Remove what FIX25/FIX27 added that becomes unneeded.
- Fixes REVIEW18 中 (open vs fresh screens differ after answer-then-break) and 低 (delete after answer).
- An empty report.md (no problems) is a rewrite as today.
- SPEC 「誰の番か」, `-h`, SKILL: one sentence each where the rule is described.
- Tests: answer-then-break on open tab, new tab, board list, open and new terminal viewer; all-settled-then-break → AI の番; missing → AI の番.

`go vet ./... && go test -count=1 ./...` must pass (no playwright-cli: say so). No go.mod changes. No company or personal names. No git. Report `docs/dev/FIX28-result.md` ≤6 lines. Reply on screen: one line.
