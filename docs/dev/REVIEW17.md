# REVIEW17: review of FEAT31, FIX25, FIX26 (and REVIEW16 regressions)

Do not change code. Write only `docs/dev/REVIEW17-result.md`. Same setup and safety rules as REVIEW16 (own worktree at the given commit; scratch boards; temp `XDG_STATE_HOME`; `--port 0`; `PITBOARD_SEND=true` and a PATH without the real herdr from the start; never touch the user's real server, state dir or the main checkout's `.pitboard/`; stop everything and prove with `ps`). `playwright-cli` is absent: say what that leaves unchecked. Always `go test -count=1`.

Read: REVIEW16-result.md, FEAT31, FIX25, FIX26 briefs and results.

a. REVIEW16 findings: each → fixed / not, measured (terminal pty and web HTTP API).
b. FEAT31: done comments fold into `✎ 対応済 N 件` after the not-done ones, on both screens; enter cycle on the row (closed → open folded → shown → closed); a new done thought keeps it folded and updates N; rows with no done comment unchanged; nothing else in enter/guard behaviour regressed; thoughts list unchanged.
c. FIX26 + FIX25 turn rule: AI の番 iff (sent, version on screen not rewritten since, and some thought of the last send unsettled) or some thought --doing; send without thoughts (answers/hides only) needs a rewrite; broken rewrite while Fixing keeps AI の番; missing/empty report shows the word on the web. Flips live on both screens.
d. Docs (SPEC, -h, SKILL) agree with the code.

End the result with one line: `VERDICT: LGTM` (nothing 中 or above) or `VERDICT: NOT LGTM` with the reason. Report: 3-line summary; findings by severity with reproduction; unverified. ≤35 lines. Reply on screen: one line.
