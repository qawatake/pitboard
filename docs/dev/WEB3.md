# WEB3: `serve` runs in the background; quieter page

Do after WEBFIX1. Note: PR #1 (e2e suite, `e2e/`) is now merged and pulled into this checkout; `go test ./...` runs it (~9 s). If an e2e test fails because of your change and the SPEC did not change, fix the code.

## A. Background by default (like `mo`)

User: 「mo とかみたいに background で動くように」 (`mo --help` on this machine shows the model: returns immediately, `--status`, `--shutdown`, `--foreground`).

- `pitboard [-s S] serve`: if no server is running for this session → start one detached (re-exec self with `--foreground`, new session/process group, stdio to `.pitboard/<session>/serve.log`), wait until it answers, print the URL, open the browser, return 0. If one is already running → do not start another: print its URL, open the browser, return 0.
- State file `.pitboard/<session>/serve.json` (0600): pid, process start time (same stale-pid check as `waiting`), port, token, started-at. Removed on clean exit; stale files ignored and replaced.
- `pitboard [-s S] serve --stop`: stop this session's server (SIGTERM, wait, report). `pitboard serve --status`: list servers of every session of the board found from cwd (session, URL without… no: WITH token only when stdout is a terminal? keep simple: print session, port, pid, started; and the full URL only with `--url`). `--foreground`: old behaviour (stay attached; ctrl+c stops). `--no-open`, `--port` as before.
- Idle exit: the background server stops itself after 30 min with no SSE client connected (not in `--foreground`). A tab reconnecting within that time keeps it alive.
- Reopening must keep working for an already open tab after `serve` is run again (same server, same token).
- The token is never written to the log.
- `-h`, README, SPEC updated (how to stop; that it idles out).

## B. Quieter page (user feedback after trying it)

User: 「ブラウザモードのフォーカスが強すぎる。左の枠線もAI スロップっぽいからいや。あと、ショートカットが追随してくるのもいや」

- Current row: mark it the terminal's way and nothing more — a `>` in the gutter (same column idea as the terminal: cursor column, mark column, sign column). No background fill, no left border/accent bar, no box, no shadow. Keyboard focus ring: keep one for accessibility but make it the same quiet thing (the `>` plus a thin outline only when focus came from the keyboard, `:focus-visible`), not a second highlight on top.
- Remove the action buttons/shortcut hints that appear on (and move with) the current row. Put them where the terminal has them: the fixed bottom bar shows the hints for the current row (same words/order as the terminal's hint line, from `page`), each one a real button (click = the action). The row itself shows no chrome.
- General: no decorative borders, cards, pills, gradients, or colored side stripes anywhere. Hierarchy by spacing, weight and the three meaning colors only (as the terminal). Check every element against that and strip.

## Verify

Go tests for start/reuse/stop/stale state/idle exit (short timeouts injected), token not in log. `playwright-cli` screenshots (1280/390, light/dark) of: a page with a current row, open row, editor open — save under scratchpad, list paths. Never system Chrome. Scratch boards only; `--no-open` in tests; kill every server you start.
No git. Report: `docs/dev/WEB3-result.md` ≤25 lines. Reply on screen: one line.
