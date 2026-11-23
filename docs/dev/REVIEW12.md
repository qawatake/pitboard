# REVIEW12: review the browser front end (`pitboard serve`) at 93e70f7

Do not change code. Write only `docs/dev/REVIEW12-result.md`.

Setup: `git archive 93e70f7 | tar -x -C <scratchpad dir>`, `go build ./cmd/pitboard`. Run only against a scratch board in a scratchpad dir, always `--no-open`. Browser checks ONLY via `playwright-cli` (never the system Chrome: it triggers Keychain prompts; see `docs/dev/WEB2-result.md` for the executablePath the builder used). Close browsers and stop servers when done. Never touch any repo's `.pitboard/`. No real herdr; `PITBOARD_SEND` = harmless file append.

Read: `docs/dev/WEB2.md` (the brief), `docs/dev/WEB2-result.md` (guesses + unverified), `docs/dev/FIX15.md` (terminal rules added in parallel; the web was written before them).

Check:
a. Security, adversarially: token/cookie, Host and Origin checks (rebinding, cross-site POST/GET, missing headers, `localhost` vs `127.0.0.1`, IPv6), CSP actually enforced in the browser (console), every response's headers, path traversal on static and `/api/image/N`, oversized/odd uploads, non-image bytes, JSON abuse, report/comment text containing HTML/script/`javascript:` links/control chars rendered inertly, SSE without cookie, listening address (confirm not reachable via the machine's LAN address), token not logged or leaked in redirects/Referer.
b. Parity with the terminal viewer: same words, marks, counts, right-end words, comment states, guards. Diff concretely: list every user-visible rule where web and terminal differ (including FIX15's: `x`/`r` allowed on an untouched row, refusal messages, hidden-row-reappeared notice). Is each rule taken from `internal/page`, or re-implemented in `app.js`? List the re-implementations.
c. Operation per the brief: every key, every mouse path, editor (Enter/Shift+Enter/Esc/draft kept), image paste, live update preserving state, comment list, hidden toggle, `g`, `space`, undo, send. Two tabs + terminal at once.
d. Accessibility items of the brief (Tab reachability, focus visibility and retention, live region texts, names, no color-only meaning).
e. Looks: screenshots at 1280 and 390 wide, light and dark; overlap, cut text, contrast. Save them under your scratchpad and list paths.
f. Robustness: server restart while a tab is open, board deleted, broken report, huge report, session missing.

Report (≤60 lines): top 5-line summary; findings as `severity | repro | expected vs actual`; the parity table; unverified items. Reply on screen: one line.
