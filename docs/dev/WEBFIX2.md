# WEBFIX2: REVIEW13 catalog — rendering in every state

Read `docs/dev/REVIEW13-result.md` sections i (state catalog), h (terminal), c (background server). The review was done at 0522aba; WEB4/WEB5/WEB6 landed since. First re-check each catalog item against the current tree (some are fixed by WEB4: 4, 5, 7, 8 — confirm by measuring, do not assume), then fix everything still true. Decisions where the catalog leaves a choice:

- 1 long unbroken strings: wrap anywhere (`overflow-wrap: anywhere`) in titles, paragraphs, lists, comments, link targets; Code scrolls horizontally inside its own box (the only allowed inner scroll besides the comments panel >70vh). Nothing may be clipped silently; add a check that no element has scrollWidth > clientWidth except Code.
- 2 pinned notices: separate the pinned band from the page with background = page color and a hairline bottom rule + small gap (allowed here: it is a floating layer, like the comments panel). Opened notice body taller than 30vh → unpin (as the terminal does).
- 3 focus on bar buttons: visible `:focus-visible` (fix the specificity), same quiet style as other controls.
- 6 state words never wrap (`white-space: nowrap`) anywhere.
- 9 bottom area: bar, status line, comment button share the page's text edge and width; bar stays on one line at ≥900px (shorten by the same dropping order the terminal uses, from `page.Bar`); at 390 wrap to at most 2 lines.
- 10–12 columns: link target under the title's text edge; option texts aligned whether chosen or not (the ● hangs); AI reply aligned with the comment text; images under the comment they belong to.
- 13 `g`: letters go in the gutter (cursor column), nothing shifts; the bar shows the jump list while in goto mode (as the terminal).
- 14 textarea grows with content up to 40vh, then scrolls.
- 15 `?` opens scrolled to top with focus on the dialog heading (or first item); the ● samples keep their colors.
- 16 hidden rows shown with `H` are faint for every kind, Notice included.
- 17 comments keep their newlines everywhere.
- 18 status messages: one rule in `page` for when a message is cleared (next key / entering the editor / opening a panel), used by both fronts; the bar never offers keys that would type into the editor.
- 19 disconnected: banner left-aligned on the text edge; bar dimmed and disabled too.
- 20 at 390: right-end word stays on the title's first line when it fits within 40% of the width, else moves to its own line right-aligned for ALL rows of that list (no mixed row heights); notice time likewise.
- 21 vertical rhythm: define spacing tokens (row gap, after-open-body gap, before-heading gap) and apply uniformly; list the tokens in the result.
- 22 comment panel list: number column fixed width, text hangs; `x` in the list says `取り消す` only for retract and `u` says `元に戻す` (take the words from `page`; fix the terminal too if it has the same collision).
- 23 titles starting with `> `: nothing to change in drawing.
- 24 sign glyphs share one box (same width, centered) so text edges match to the pixel.
- h1 (terminal + web): after an empty Section's `なし`, and generally before any top-level Fold, keep the same blank gap as before a Section heading.
- h2 terminal Code wrap: continuation lines align with the code's first column.
- h3 terminal: wrapped title lines after the first may use the full width when the right-end words are only on line 1 — NO: keep the reserved column (decided earlier so the right words read as a column); nothing to change.
- h4: order and time format are the same on both fronts: counts line above notices? → follow the terminal (notices first, then counts) on the web too; time format from `page.AtWord` on both.
- c (server; adapt to WEB6's one-server model): a lock so that concurrent `serve` calls all succeed and print the same URL; a board whose directory vanished is dropped and its tabs get the disconnected banner with `この pitboard は見つからなくなりました`; state file written 0600 even if a stale tmp exists (create with O_EXCL after unlink; verify mode after write); empty `Start` never matches a live pid; `pitboard serve -h` prints serve usage.

## Added: the `?` dialog, and a reproducible state gallery

User: 「キーモーダルの描画確認ってやったかな。もしやってないならなぜ漏れたのか、再発防止はできるか」
Reviewer's shot `15-help.png` shows more than catalog item 15 says: the pinned Notice's title is painted OVER the dialog's first row (stacking: sticky band above the dialog), the dialog opens scrolled to the bottom with no heading visible, descriptions are all faint, the page behind shows through at the edges with the bar on top.

- Overlays (the `?` dialog, comments panel, image viewer): above everything incl. the pinned band and the bar (one z-order scale, written down in the CSS as tokens); backdrop covers the whole viewport; body scroll locked; opens at scroll top with a visible heading; text in normal color (keys in mono, descriptions normal, only secondary notes faint); two-column grid with a fixed key column; max-height 85vh with its own scroll; at 390 full-screen sheet.
- State gallery (the structural fix for "a state nobody looked at"): add `docs/spec/web-states.md` — the list of every UI state of the page (rows of each kind × closed/open/current, editor variants, each overlay, goto mode, banners, empty/error, hidden shown, 390 variants…; start from REVIEW13's walk and add what it missed) — and a script `scripts/web-gallery.sh` that, with `playwright-cli` (never system Chrome), builds a dense scratch board, drives the page into each listed state, saves `NN-name.png` (1280 light; 390 + dark for the ones marked in the list) into an output dir and writes an `index.html` contact sheet (plain file, no deps). The script must fail if a state in the list has no screenshot step, so a new UI state cannot be added to the list without a shot, and SPEC says: any change that adds a UI state adds it to this list. Run it and give the output path in the result; the coordinator will look at every image before release.

Verify: reproduce the reviewer's state walk on a dense report and re-measure; put a before/after table (catalog number → fixed / not reproducible / unchanged+why) in the result. `playwright-cli` only, `--no-open`, temp `XDG_STATE_HOME`, never touch the user's running server, scratch boards, stop everything. vet/test/build incl. e2e; terminal changes need e2e/unit updates + SPEC.
No git. Report `docs/dev/WEBFIX2-result.md`. Reply on screen: one line.
