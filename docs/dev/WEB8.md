# WEB8: the URL and the open tab survive a server restart

User: 「今のブラウザ機能って同じセッションでもurlが都度変わるの？？もしそうなら不便だなと」 — today the token is new on every server start, so after each restart the old tab is dead ("つながっていません…URL を開き直して") and `serve` opens another tab.

- The server's token persists across restarts: kept in the state dir (`token`, 0600, written with `WritePrivate`), created once, reused by later starts. `pitboard serve --rotate` makes a new one (and restarts the server if running); `--stop` does not delete it. Idle exit does not delete it.
- Cookie lifetime: persistent (e.g. 30 days, renewed on use) instead of session-only, still HttpOnly + SameSite=Strict, so `http://127.0.0.1:7482/s/<id>/` works from a bookmark without `?t=`.
- An open tab reconnects by itself when the server comes back (same port, same token): the disconnected banner clears and the page refetches; drafts/open state kept. If the port changed (7482 taken), the banner says so cannot be known — keep the current text.
- `pitboard serve` when a tab is probably already open: still prints the URL; opens the browser only if no SSE client for that board connected within ~3 s after start (so a restart with a live tab does not spawn a second tab). `--open` forces, `--no-open` suppresses.
- Printed URL: without `?t=` when… no — always print with `?t=` (it must work in a fresh browser); the page strips `?t=` from the address bar after login (history.replaceState) as now/if not already.
- Security notes for the result: what changes in exposure (token long-lived on disk, same user only; cookie persistent in the browser profile) and why that is acceptable for a loopback-only server; tests for perms, rotate invalidating old cookies, restart keeping them.
- `-h`, README, SPEC.

Verify incl. a real restart with a tab open via `playwright-cli` (scratch state dir, `--port 0`, never the user's server). `scripts/check.sh`. No git. Report `docs/dev/WEB8-result.md` ≤10 lines. Reply on screen: one line.
