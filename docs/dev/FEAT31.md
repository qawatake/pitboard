# FEAT31: settled comments fold into one line under their row

Person's words: 「質問にコメントを付けまくるとすごい見にくくなるね」. Today every thought written on a row shows under it as `✎ comment` + `対応: reply` (status word 対応中 / 対応済 at the right; words in `internal/page/words.go:62-83`), done ones included, so the row itself gets buried.

Decided (person chose): under a row, thoughts settled as done fold into ONE line `✎ 対応済 N 件` (faint), which the person can open with enter (and close again) to see them as today; thoughts not done (unsent, sent, doing, hold) stay listed as today, above/below — keep today's order for them and put the folded line after them. N = 1 still folds. Same on the terminal viewer and the web page (words from `page`).

- The folded line is a stop for the cursor only if today's comment lines are stops; follow whatever today's rule is for comment lines (say what it is).
- Opening state per row lasts for the viewer's lifetime (like other opens); a new done thought keeps the line folded and updates N.
- The thoughts list tab is unchanged.
- SPEC.md: describe it where comments under rows are described. `-h`/SKILL only if they describe how comments show.

Tests: page/viewer/web, e2e if cheap; gallery fixtures `-update` and a variant if the gallery has one for comments. No go.mod changes. No company or personal names. Do not touch guard rules.

`go vet ./... && go test ./...` must pass (gallery browser step needs playwright-cli, absent: say so). No git. Report `docs/dev/FEAT31-result.md` ≤10 lines. Reply on screen: one line.
