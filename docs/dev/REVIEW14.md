# REVIEW14: review at 83eefe9 (WEB6 one server, WEBFIX2 gallery + catalog fixes, FIX16)

Do not change code. Write only `docs/dev/REVIEW14-result.md`. Setup as REVIEW13 (git archive 83eefe9; scratch boards; temp `XDG_STATE_HOME`; `--port 0`/fallback ports — the user's REAL server runs on 7482 with the real state dir: never touch or stop it; `playwright-cli` only, never system Chrome; no real herdr). Stop everything you start; prove with `ps`.

Read: `docs/dev/REVIEW13-result.md` (your catalog), `WEBFIX2.md` + result, `WEB6.md` + result, `FIX16.md` + result, `docs/spec/web-states.md`.

a. Gallery as the review surface: run `sh scripts/web-gallery.sh <out>` and look at EVERY image (per-variant, sheets, pages). Critique each module as layout (alignment measured where in doubt, spacing rhythm, hierarchy, what the eye lands on, color meaning — only yellow/green/faint carry meaning; list any 4th color and where it comes from). Catalog: module/variant → problems, by severity.
b. Is the gallery complete in the sense the user cares about ("not a flat list; missing things must be visible")? Find real page states that NO variant shows: drive the live page (dense report) through your REVIEW13 state walk and for each state name the variant that covers it, or "none". Also judge the completeness test: can I add a `page` enum value / UI mode / visible difference without a variant and still pass? Try it in your scratch copy (add a fake Mark value, a fake mode) and report what fails.
c. Live page regression of the 25 catalog items (the builder says all fixed): verify each by measurement on the live page, not the gallery.
d. One server (WEB6), adversarially: per-path isolation (board A's cookie/tab acting on board B's rows, tab id reuse across boards, undo history crossing), registration abuse (path traversal, symlinked `.pitboard`, other user's dir if testable, registering while the dir is being deleted, 1000 registrations), `/` index leaking anything without cookie, id collisions/`~2`, `--forget`, restore after restart, board gone → banner, lock under 8 concurrent `serve`, idle exit with only the index open, state dir perms, token never in logs/ps/redirect Location/Referer.
e. FIX16: removed-row warnings — false positives (row removed after the person hid it; after `x` then AI removes; id renamed; row moved into a Fold; done comment), false negatives, `rows.json` growth over 500 rewrites, `ack-removed`.
f. Terminal viewer regressions from WEBFIX2's shared changes (`page.DropRank`, `MsgClears`, `EditorHints`, words 取り消す/元に戻す, Code wrap, bold open sign): pty at 80 and 40.

Report: 5-line summary; then findings by severity `sev | where | repro | expected vs actual`; the coverage table for b; unverified. No length limit on the catalog parts. Reply on screen: one line.
