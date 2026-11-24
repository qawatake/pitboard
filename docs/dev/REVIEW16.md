# REVIEW16: review of FIX22, FEAT29, FEAT30, FIX23, FIX24

Do not change code. Write only `docs/dev/REVIEW16-result.md`. Work in your own worktree at the commit you were given. Scratch boards only; temp `XDG_STATE_HOME`; `--port 0`; never touch the user's real server or real state dir, nor `.pitboard/` of the main checkout; no real herdr. `playwright-cli` is not installed on this machine: say what you could not check because of it. Stop every process you start; prove with `ps`. Do not trust the builders' "verified": run things yourself (pty for the terminal, HTTP API for the web, mutation of the code in your worktree to see a test fail is welcome).

Read: FIX22, FEAT29, FEAT30, FIX23, FIX24 briefs and results in `docs/dev/`; `git log -6`.

a. x (FIX23 is the rule now): on a line the screen moved the cursor to, the first x within 0.4 s is dropped with the message, the same x again hides it; after 0.4 s one x hides. Web: a quick second x never unhides the row just hidden; presses are handled in order; `Board.Hide` refuses to unhide when hidden lines are not shown. u undoes one at a time. Any leftover of FIX22's `Hid` rule?
b. FEAT29: a report with check problems after a good one → both screens keep the good one with the quiet `AI が報告を直しています`; fixed → it goes; started broken → problems shown; removed-row warnings do not trigger it; a press on the shown good version when the file is broken: recorded only when the row is identical on disk; empty report / deleted file / ReadErr cases; wait/changes/settle print the problem note.
c. FEAT30 + FIX24: AI の番 iff (sent and report.md not rewritten since the last send) or a thought still --doing; otherwise あなたの番 (normal color; AI の番 faint; yellow only for 反応待ち/未送信). Edge cases: rewrite in the same second as send (nanosecond t; old second-precision events), send then settle --done without rewriting, --hold, several sends, mtime going backwards (editor/rsync), clock skew, report deleted. Does it flip live on both screens? `送信済 N` gone everywhere (SPEC, -h, SKILL, web)?
d. Docs: SPEC.md, `-h`, SKILL agree with the code; nothing still describes the FIX22 rule.

Report: 3-line summary; findings by severity (高/中/低) each with a reproduction; unverified. ≤40 lines. Reply on screen: one line.
