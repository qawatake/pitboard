# WEB2: browser front end — `pitboard serve`

Self-contained. Read first: `docs/dev/WEB1-result.md` (the `internal/page` API and its "WEB2 で足りなそうなもの" list), `pitboard -h`, `docs/spec/SPEC.md` (wording/rules of the terminal viewer).

User approved this plan (`y`), plus: 「moみたいに勝手に開いてもいいよ。勝手に開かないオプションをフラグで用意してくれたらいいかも」

## Goal

A second front end on the same board files, built ONLY on `internal/page` (+ `board`, `report`). Same parts, same words, same marks, same rules as the terminal viewer. Terminal and browser can run at once; each sees the other's actions.

## Command

- `pitboard [-s S] serve [--port N] [--no-open]`. Session resolution identical to the viewer.
- Listen on `127.0.0.1` only (never 0.0.0.0, never ::). Default port: OS-chosen free port.
- Print one line: the URL (with token). Then open it in the default browser (`open` on macOS, `xdg-open` elsewhere; failure is not an error) unless `--no-open`.
- ctrl+c / SIGTERM: shut down cleanly.
- stdlib only (`net/http`, `embed`). No JS/CSS dependencies, no build step: hand-written static files under `internal/web/static/`, embedded.

## Security (must hold; write tests for each)

- Random token (≥128 bits, per start). First load via `/?t=TOKEN` sets an HttpOnly, SameSite=Strict cookie and redirects to `/`; every other request needs the cookie. No token/cookie → 403 with a plain message telling to use the URL printed by `pitboard serve`.
- Reject requests whose `Host` is not `127.0.0.1:PORT` or `localhost:PORT` (DNS rebinding). For state-changing requests also require `Origin` to match.
- State changes are POST with JSON body only.
- Headers on every response: `Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`.
- Report text is DATA: build DOM with `textContent` / element creation from `page.Span`s. Never `innerHTML` with report or comment text. Links: only http/https, `rel="noopener noreferrer"`, `target="_blank"`.
- Images: serve only the session's saved comment images by number (no paths from the client). Uploads: size cap as in the terminal (reuse `Store.SaveImage` limits), content sniffed as image.
- No external requests of any kind from the page.

## Server ↔ page

- `GET /api/page` → JSON of `page.Page` (names, not ints, for Kind/Tone/Mark/Style; no `report.Node`). Include each Thing's `Seen` token (hash is fine) and "starts open" flag (decide in `page`, so both fronts agree).
- `GET /api/events` → SSE; push "changed" when `Board.Poll`/`Report.Stale` say so (poll interval as the viewer's). Page refetches.
- `POST /api/act` → `{action, target, seen, ...}` → runs the `page` action, returns `{ok, message}` (message = the same notice text the terminal shows). Stale `seen` → refuse with the terminal's "報告が書き換わりました…" message and let the page refresh.
- `POST /api/send` → wake; may take seconds: return when done, page shows a busy state meanwhile.
- `POST /api/image` → save pasted/dropped image, returns its number.
- Undo history (`Hist`): one `page.Board` per browser tab (id in sessionStorage); drop idle ones.
- If `page` lacks something, add it to `page` (not to `web`), keeping the terminal viewer on the same code path. Terminal behaviour must not change.

## Page (what the person sees)

- One column, max readable width (~46em), comfortable line height. Light/dark via `prefers-color-scheme`. System fonts; monospace only for Code and inline code. Color meanings identical to the terminal: yellow = needs the person, green = moving/unread, faint = quiet. Contrast ≥ 4.5:1 for faint text too.
- Notices pinned at top (sticky), yellow, title never cut, body opens. Time at the right.
- Top-right summary: 反応待ち N / 未読 N / 未送信 N / AI 待機中 — same words.
- Sections, top-level folds styled as headings. Fold contents indented. Items: mark column (yellow/green ●), open indicator, title (wraps), right-end words never cut. `re` rows drawn per FIX12 (quote first line, AI title second).
- Comments under a row when open; a comment list view (terminal `tab`) and the "コメント" area equivalent at the bottom for unsettled ones.
- Hidden rows hidden; a toggle (terminal `H`) shows them faint.
- Empty/missing/error states use the same texts as the terminal (`page.Msg*`).

## Operation

- Keyboard: same keys as the terminal when focus is not in a text field: `j k` (and arrows), `enter` open/close, `y n`, option letters on an open Ask, `c`, `m`, `e`, `x`, `r`, `A`, `u`, `s`, `g`+letter, `space`, `H`, `tab`, `?`. A visible current row (same `>`-like marker or outline). The terminal's guard "viewer-moved cursor / changed row → no reaction until the person moves" applies equally (use `seen`).
- Mouse/touch: clicking anywhere on a row selects it and toggles open. The selected row shows its available actions as real buttons (labels = the terminal's hint words: よし / だめ / 行に書く / 隠す / 読んだ, options as buttons). A fixed bottom bar: 書く (m), 送る (s, shows unsent count, primary style only when there is something to send), 取り消す (u), キー (?).
- Editor: a `<textarea>` placed directly under the row (same placement rule as the terminal), Enter = place, Shift+Enter = newline, Esc = cancel (confirm only if non-empty? no: keep the draft in sessionStorage so nothing is lost), IME-safe (ignore Enter during composition). Paste/drop an image → uploaded, inserted as `[Image #N]`, thumbnail shown. Comments show their images inline (click = full size in the page, Esc closes).
- Accessibility: everything reachable by Tab and operable by Enter/Space; real `<button>`s; focus always visible; after open/close/act, focus stays on the row; result messages in an `aria-live="polite"` status region with self-contained text; single-key shortcuts only when focus is on the board (not in inputs), and can be seen in `?`; marks never by color alone (● plus a text alternative: 反応待ち / 未読); respect `prefers-reduced-motion`.
- Live update keeps selection, open states, scroll position and any draft.

## Tests

- Go: handlers (auth, Host/Origin, CSP headers, act round-trips for every action against a scratch board, stale `seen`, image limits), JSON shape.
- No browser automation required from you. Keep JS small and plain (ES modules, no framework) so it can be reviewed by reading. If you can run a headless check with tools already on this machine without installing anything and without launching the system Chrome app, do; otherwise say it is unverified.
- Manual pty/terminal regression: viewer unchanged.

## Docs

- `-h`: add `serve` to the command list and a short "Browser" paragraph for the AI (how to tell the person: `pitboard -s ID serve`). README: one paragraph for people. SPEC: a "ブラウザ" section stating that rules live in `page` and what differs from the terminal (pinning height, list sizes).

## Rules

- Scratchpad dirs only for running; never touch any repo's `.pitboard/`. Never run real `herdr agent prompt` / `herdr pane send-*`. Do not open a browser window on this machine during tests (`--no-open`).
- No git.
- Work in stages that each pass vet/test/build: (1) server skeleton + security + `/api/page`, (2) read-only page, (3) actions, (4) editor + images, (5) live update, (6) a11y pass + docs.
- Report: `docs/dev/WEB2-result.md`: what exists (routes, files, 1 line each), guesses, unverified points (be exact about what was only read, not run in a browser). Reply on screen: one line.
