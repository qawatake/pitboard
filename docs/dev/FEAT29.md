# FEAT29: a report that fails check never reaches the person

Person's request (as written): 「AI が report.md を直接編集して構文エラーで壊すことがある。lint が通らないと受け取りを拒否する register 的な口があるとよい … 壊れた版は人に届かない」. Decision (person chose): no new command; the screens keep the last good version, and the AI is told through the commands it always runs.

Today: `internal/page/report.go` `Report.Load` keeps the previous nodes only when reading/parsing panics or fails (`ReadErr`); a report that parses but has check problems (`Probs`, e.g. an id over 16 chars) is shown with its problems in the person's face.

Build:
1. Terminal viewer and web page: when report.md has problems (what `pitboard check` exits 1 for; removed-row warnings do not count), keep showing the last version read that had none, plus one quiet line at the top (not a Notice, not yellow): `AI が報告を直しています` (words live in `page`, both screens use them). When a good version comes, it replaces it and the line goes. Marks, hides, answers act on what is shown (the good version) — keep the existing "report changed" guards correct against the file on disk (a press on the shown good version when the file is broken: refuse with the existing changed message, or allow if the row is identical in the broken file — pick the safe one, say which).
2. No good version yet (the viewer started on a broken report, or the first report is broken): show the problems as today.
3. `pitboard wait` and `pitboard changes`: when report.md has problems, their output ends with the same note the hook already uses (`ProblemNote`, `cmd/pitboard/hook.go`); `wait` returns at once if the report already has problems when it starts? No — keep wait waiting for the person; just append the note when it returns. Also `settle` prints the note if problems exist.
4. `-h` and SKILL text: one or two sentences: a report that fails check is not shown to the person (they keep seeing the last good one), so fix what `check` / `wait` / `changes` report.
5. SPEC.md: add the rule where report problems are described.

Tests: terminal, web, cmd (wait/changes/settle output), e2e where cheap. Do not touch FIX22's guard rules (`internal/page/guard.go`) beyond what is needed. No go.mod changes. No company or personal names anywhere.

`go vet ./... && go test ./...` must pass (the gallery step of scripts/check.sh needs playwright-cli, absent here: say so). No git. Report `docs/dev/FEAT29-result.md` ≤12 lines. Reply on screen: one line.
