# REVIEW19: re-review after FIX28

Do not change code. Write only `docs/dev/REVIEW19-result.md`. Same setup and safety rules as REVIEW18 (create your scratch dir with its own `.pitboard/` and cd there BEFORE any pitboard command; `PITBOARD_SEND=true`, PATH without herdr, temp `XDG_STATE_HOME`, `--port 0`; stop everything, prove with `ps`; `go test -count=1`). `playwright-cli` is absent.

Read REVIEW18-result.md, FIX28.md + result, `git diff 7b517e3..HEAD`.

a. Rule: report.md failing check or missing → `AI の番` on every screen (open tab, new tab, board list, open and new terminal viewer), whatever the sends/settles; otherwise FIX26's rule. Cases: answer-then-break, all-settled-then-break, never-sent-then-break, delete after answer, empty file, fix after break (all flip back together).
b. Regression smoke: FEAT29 kept good version + quiet line, FEAT31 fold, FIX26 settle rule, x rule.
c. SPEC/-h/SKILL agree with the code; nothing still mentions `Report.Shown` semantics.

End with `VERDICT: LGTM` (nothing 中 or above) or `VERDICT: NOT LGTM` + reason. ≤20 lines. Reply on screen: one line incl. the VERDICT.
