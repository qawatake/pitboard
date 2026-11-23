# WEBFIX1: fix REVIEW12 findings — one source for the guards

Read `docs/dev/REVIEW12-result.md` (findings + parity table). Main checkout (fixes branch is already merged; FIX15 code is here: `internal/page/untouched.go`, viewer). Terminal behaviour must not change (e2e suite is not merged yet; unit tests must pass).

## A. Guards live in `internal/page` (the core finding)

Today `app.js` re-implements placed / lock / 0.4 s / "next press works" with different rules and texts. Make `page` the only place that decides and words it:
- Add a UI-neutral guard to `page` (e.g. `type Touch struct{ Placed bool; MovedAgo time.Duration; Locked bool; LastRefused … }` + `func (b *Board) Allow(action, thing, touch) (ok bool, msg string, next Touch)` — shape is yours) that encodes exactly the terminal's current rules incl. FIX15: `x`/`r`/`c` on a placed row, `y`/`n`/options refused with the terminal's texts, the 0.4 s window texts, "same key again on the same row works", lock message for every refused key, a row reached by hiding stays placed.
- viewer uses it (replacing its own copy). web: JS only reports raw facts with each act (how the current row became current: by the person / by the app, and ms since); the server asks `page` and returns `{OK, Message, …}`. No rule text or rule logic in `app.js`. The hint/buttons shown for the current row also come from the server's answer (`/api/page` or a small `/api/hint`), so the page never advertises a refused key.
- Hidden-row-reappeared notice: decide in `page` (both fronts show it).
- All action result texts come from `page` (hide/unhide/read/undo/react/comment…), identical on both fronts incl. `(H で表示、u で戻す)`, `読んだことにしました: <題> (u で戻す)`. Where the terminal is silent but the web needs a live-region text, add the text to `page` and let the terminal ignore it.
- Row sign (▸▾✓?→⌀·!), closed-Ask answer head, jump letters (skip `g`), problem heading: from `page` (names/strings in the JSON), not recomputed in JS.

## B. Focus is the cursor

Focusing a row by Tab or click makes it the current row (counts as moved by the person). Keys act on the focused row only. After any button click or action, focus stays on that row (or the row the cursor moves to). `?` and image dialogs trap focus and restore it on close.

## C. Connection state

SSE error / 403 / server gone: show a persistent banner `つながっていません。pitboard serve が出した URL を開き直してください` (text in `page`), stop pretending the view is live, retry quietly; on reconnect with a valid cookie, clear it and refetch.

## D. Performance

Cursor movement must not re-render the page (toggle classes/attributes on two rows). On refetch, update by key (reuse unchanged row nodes). Target: 5000 Items → `j` < 16 ms, refetch render < 300 ms. Consider `ETag`/unchanged short-circuit for `/api/page`.

## E. Small ones (all "低" rows of the review)

390-wide open Ask option layout; bottom padding follows the status height; `retract`/`edit` with unknown N refused in `page`; request-too-large text; status not overwritten by a later reload failure; stale `飛ぶ先の字を…` cleared; returning from the comment list restores the cursor.

## Verify

vet/test/build; Go tests for `page` guard table (one table drives both fronts' expectations); web handler tests updated. Browser checks via `playwright-cli` only (never system Chrome), `--no-open`, scratch board. Re-run the review's repro for every 高/中 row. pty spot check that the terminal is unchanged.
No git. Report: `docs/dev/WEBFIX1-result.md` (≤30 lines: per letter, guesses, unverified). Reply on screen: one line.
