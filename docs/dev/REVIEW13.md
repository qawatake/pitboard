# REVIEW13: re-review the browser mode at 0522aba (WEBFIX1 + WEB3)

Do not change code. Write only `docs/dev/REVIEW13-result.md`. Same setup rules as REVIEW12 (git archive 0522aba, scratch boards, `--no-open`, `playwright-cli` only — never system Chrome, no real herdr). You WILL start background servers: track every pid, stop each with `serve --stop`, and prove at the end with `ps` that none of yours is left.

Read: `docs/dev/REVIEW12-result.md` (your previous findings), `docs/dev/WEBFIX1.md` + result, `docs/dev/WEB3.md` + result.

a. Regression of every REVIEW12 finding (高/中/低): fixed / not fixed / changed.
b. Guard parity: is there any rule or user-visible text still decided in `app.js` rather than `internal/page`? Diff terminal vs web again with the same table.
c. Background server, adversarially: double start (race two `serve` at once), stale `serve.json` (dead pid, reused pid, garbage, wrong perms), `--stop` when none / when foreign pid, `--status` output (no token unless `--url`), log never contains the token, state file 0600 and inside the session dir, idle exit (use the test override env), behaviour when the session dir is deleted while running, SIGKILL then restart, `--foreground` while a background one runs, parent shell exit (does the child survive, is it detached from the tty), zombie/orphan check, port in use, two sessions at once.
d. Does putting the token in `serve.json` weaken anything? (who can read it; compare with the cookie model.) Is the token visible in `ps` args or env of the child?
e. Quiet page: screenshots 1280/390 light/dark of current row, open row, editor, comment list, `?`. List anything decorative left (borders, fills, pills), anything hard to see now (is the current row findable? focus-visible?), font weight (the builder's shots look uniformly bold — real or headless artifact? check computed font-weight), bottom bar wrapping at 390.
f. Keyboard a11y after the change: Tab order incl. bottom bar buttons, names, live region.

Report ≤50 lines: summary (5 lines), findings `severity | repro | expected vs actual`, table for a, unverified. Reply on screen: one line.
