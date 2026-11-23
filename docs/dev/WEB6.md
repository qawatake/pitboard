# WEB6: one server for all sessions (by path); floating comments; clearer fold signs

Three independent parts; do A last (largest). Result file after all three.

## B. Comments float instead of a fixed inner-scrolling pane

User: 「ここのコメントエリアの高さが一定で内部スクロールなのは気に入らないね。なんというかブラウザバージョンはモーダルのように浮いていてもいい気がするな」
(My reading: the bottom "コメント" pane of unsettled comments, fixed height with its own scrollbar.)

- Remove the fixed-height pane and its inner scroll from the page flow. The page has exactly one scroll (the document), plus pinned notices and the bottom bar.
- Comments are reached through one floating panel (a non-full-screen dialog anchored bottom-right on wide screens, full-width sheet at ≤480px): opened by the bar button `コメント N` (N = not yet 対応済; show `未送信 M` in yellow inside the label when M>0) and by the terminal's list key equivalent you already have. It lists all comments (unsettled first, then settled, newest first within each), each with its state word, target title (click/Enter → closes the panel, moves the cursor to that row, scrolls it into view), AI reply, image thumbs; `e` edit, `x` retract, `u` undo work inside as in the current list. Esc / click outside closes; focus trapped while open and restored after. The panel sizes to its content up to 70vh and only then scrolls.
- Merge the existing "コメント一覧" view into this panel (one way to see comments, not two).
- Comments attached to a row still show under that row when it is open (unchanged).
- No decorative chrome: a hairline border or a soft shadow is allowed here because it floats; nothing else.

## C. Fold/open signs easier to read

User: 「折りたたみ記号はもうちょっと大きくできない？今右を向いているのか下を向いているのか見づらい。(これはterminal版も同じかも。)」

- Web: draw the open/closed sign for Fold, openable Item, Notice, at about the cap height of the title (≈0.8em wide), clearly a right-pointing vs down-pointing triangle/chevron, vertically centred on the first line, same color as now. Pure CSS/glyph, no images. Keep the gutter width; re-measure alignment (WEB4 numbers must still hold).
- Terminal: `▸`/`▾` → `▶`/`▼` only if they render one column wide and aligned in the user's terminal class (Ghostty, iTerm2, kitty, WezTerm, tmux) — these are East-Asian-ambiguous in some fonts. Decide by checking the width tables you already use (`internal/viewer` width function) and what those terminals do with U+25B6/U+25BC vs U+25B8/U+25BE; if ambiguous-width risk exists, keep the glyphs but make state unmistakable another way (e.g. bold for open) and say so in the result. Decide the glyph in `internal/page` (sign names) so both fronts stay in sync; update e2e/unit expectations and SPEC.

## A. One server, sessions by path

User: 「セッションごとにサーバーを立てるのは嫌だな。パスとかで分けるようにしたい。最終的にはpitboardは1セッションだけでなく複数セッションも統合可能な機能に育てていきたいと思っているので。(まだそこまでやらなくていいんだけどさ。)」

- One background server per user (port 7482, fallbacks as now). State moves from `.pitboard/<session>/serve.json` to the user state dir (`$XDG_STATE_HOME/pitboard/` or `~/.local/state/pitboard/`; `serve.json` 0600, `serve.log`).
- `pitboard [-s S] serve` in any directory: resolve the board as now (root dir + session); if no server → start it; then REGISTER that board with the server (authenticated local call) and open `http://127.0.0.1:7482/s/<short-session>/` (short = shortest unique prefix among registered, ≥8 chars). Running it again for another session/project adds that one; nothing restarts; open tabs keep working.
- Routes become per board: `/s/<id>/`, `/s/<id>/api/page|events|act|send|image…`. Static assets shared. Per-tab undo history keyed by (board, tab).
- `/` = a plain index: registered boards as rows (title from the report's Title, directory, 反応待ち/未読/未送信 counts, AI 待機中), same quiet style, keyboard `j k enter`. Live-updating. This is the seed of the multi-session view; do NOT build more than this list.
- Registry persistence: boards registered are remembered in the state dir and restored on server start if their directory still exists; a board whose `.pitboard/<session>` vanished is dropped silently. `pitboard serve --forget [-s S]` removes one.
- `--stop` stops the one server; `--status` lists the server and its boards; idle exit as before (no SSE client on any board for 30 min).
- Security unchanged in strength: one token/cookie for the server; a board id in the path is not a secret and grants nothing without the cookie; registration endpoint requires the token (from the 0600 state file), validates that the path is an existing directory containing `.pitboard/<session>/` owned by the current user, and never serves files outside `.pitboard/<session>/images`. Host/Origin/CSP as now. Re-run the security tests per board path (traversal via `<id>`, registering `/`, symlinks).
- Migration: an old per-session `serve.json` is ignored and deleted when found; an old per-session server still running is stopped by `--stop` (best effort).
- `-h`, README, SPEC.

## Verify

vet/test/build incl. e2e. `playwright-cli` only (never system Chrome), `--no-open`, scratch boards, a temp `XDG_STATE_HOME` so the user's real server/state is never touched (their real server may be running on 7482: your tests must use other ports or the fallback, and must never stop it). Screenshots (1280/390, light/dark): index, a board, comments panel open, fold open/closed close-up. Stop everything you start; prove with `ps`.
No git. Report `docs/dev/WEB6-result.md` ≤35 lines (per part: what, guesses, unverified). Reply on screen: one line.
