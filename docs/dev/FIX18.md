# FIX18: terminal — scroll freely, also while writing; simplify the removed-row warning

Do after WEBFIX3.

## A. Scrolling (terminal viewer)

User: 「折りたたみの長い文に対してコメントしたいが、上の方をどうやっても見ることができない。スクロール方法がないためだ」「これくらい長いと折りたたみ内部もスクロールしたくなりますね」

Today the only way to move the view is `j`/`k` (FEAT21: reads forward through tall contents between stops). There is no way back up inside a tall opened row, and none at all while the editor is open (the editor sits under the row = below the long body).

- Scroll keys that always move the VIEW (never the cursor), by half a screen: `ctrl+d` / `ctrl+u`, PageDown / PageUp, and `alt+↓` / `alt+↑` (decode the usual sequences: `ESC [ 6 ~`/`5 ~`, `ESC [ 1 ; 3 B/A`, `ESC ESC [ B/A`, kitty forms). One line: `ctrl+e` / `ctrl+y`. Clamped to the page; pinned Notices stay.
- They work in normal mode, in the comment pane/list, and WHILE THE EDITOR IS OPEN (in the editor only PageDown/PageUp and `alt+↓`/`alt+↑`, since `ctrl+d`/`ctrl+u`/`ctrl+e`/`ctrl+y` edit text there). The editor block stays where it is in the page (it scrolls with the page); if it scrolls out of view, typing any text key brings it back into view first. The terminal cursor (IME position) is hidden while the input line is off-screen.
- `k` mirrors `j` properly: from a row whose tall contents you read down through, `k` scrolls back up through them before moving to the previous stop.
- Cursor off-screen behaviour as today (`↑ カーソルは上` hint; reaction keys bring the row back and are refused once).
- Hint line: when the page is taller than the screen show `^d ^u スクロール` at low priority; `?` lists all of them; editor hints gain `pgup pgdn スクロール` when the page is taller than the screen.
- Web: nothing to do (the document scrolls natively) — but verify that with the inline editor open under a tall body you can scroll up with wheel/keys and the textarea keeps its text and focus.
- e2e tests: tall body, scroll up/down in normal mode and during editing, text preserved, comment placed on the right row.

## B. Removed-row warning: make it smaller

User's concern about FIX16: 「運用方法の問題がツールに染み出しすぎていないか。汎用的なものとしてクリアできるならいい」. Keep only the generic contract — "what the person has not dealt with must not vanish silently" — and drop the operational surface:
- Remove the `ack-removed` command. A removal is reported ONCE: in the output of the next AI-run command and in `check` until then; after that it is forgotten (state for it deleted).
- Keep the two cases (unanswered ask/look row; row with a sent, not-done comment). Keep `rows.json` minimal (ids + what is needed for the title) or fold it into an existing state file if that is simpler.
- `-h`: one sentence in Manners, not a section.

vet/test/build (+ `scripts/check.sh` if it exists by then). No git. Report `docs/dev/FIX18-result.md` ≤12 lines. Reply on screen: one line.
