# FEAT26: links between rows (jump and come back); nothing stays hidden under a floating layer

Do after FEAT25.

## A. In-page links

User (looking at an answer Item that says "…そうする (順番待ち)"): 「こういうときにその作業が進行中とかに並ぶと思うんだよね。そのときに、そこへのリンク、ジャンプボタンみたいなのを用意できるといいんじゃないかな。すると、たしかにここで作業しているね、と確認できるようなイメージ。そういう画面内リンク機能みたいなのが欲しいかも。cmd + enterとかで遷移するのかなぁ。キーバインドは推奨してほしい」

Format (report): an inline link whose target is a row id: `[text](#row-id)` anywhere inline Markdown is allowed (titles, contents, list lines). `check`: target id must exist in the report (error with the line; ids of Item/Ask/Link/Notice/Fold). A link to a row inside a closed Fold is fine.

Decide in `internal/page`: each Thing exposes its outgoing row links in order (`Links: [{text, target}]`, collected from its title and its contents), and `Board.Follow(target)` → what must be opened (ancestor Folds) and which row becomes current.

Keys — reuse goto mode, no new top-level key (my recommendation to the user; cmd+enter is not deliverable in terminals):
- `g` enters goto mode as now (letters = sections/top Folds/Notices). Additionally the CURRENT row's links are numbered `1`…`9` in the order they appear: shown next to each link in the row (terminal: the digit in the `structure` color right before the link text; web: same) and listed in the goto list after the letters (`1 題…`). Pressing the digit: open what is needed, move the cursor to the target row, scroll it into view (same viewer-moved guard as other jumps).
- Back: every jump made by `g` (letter or digit) pushes the origin on a small stack; in goto mode `-` goes back to the previous origin (reopening nothing, just cursor+scroll). Shown in the goto list as `- 戻る` when the stack is not empty.
- If the current row has exactly one link, the hint line/bar shows `g 1 → <target title, shortened>` at low priority so the person learns it exists.
- Web additionally: the link is a real `<a>`/button — click or Enter on it (Tab-focusable) follows it; browser back is NOT used; a small `← 戻る` appears in the bar after a jump (same stack).
- Drawing a link: `structure` color; terminal marks it with a trailing `↗`-free plain style (no extra glyphs) — color + underline if the terminal supports it; web underline on hover/focus.
- Targets that vanish: `check` catches at write time; at view time an unknown target is drawn as plain text.

`-h`: format line, one manners sentence (when an answer says work is queued/in progress, link to that row instead of restating it), example. SPEC, e2e (jump, back, closed Fold target, digit only for the current row), gallery variants + `covers`/`differences`.

## B. Floating layers never make content unreachable (web)

User (screenshot: the composer open at the bottom, tall because of text + a pasted image; the page content behind/below it cannot be read): 「ブラウザのコメントの下に隠れているコンテンツが読めないのが気になる。スクロール可能にするかなにかいい方法を考えて修正してほしい」

- While a non-modal floating layer is open (composer; comments panel is modal-ish but apply too), the document gets extra bottom space equal to the layer's current height + gap (updated as it grows), so every line of the page can be scrolled to above the layer. Wheel/trackpad over the page scrolls the page even when focus is in the textarea; over the layer it scrolls the layer if it overflows.
- The layer's own height is capped (≤ 55vh): the textarea grows to a limit then scrolls; previews (small after FEAT25) sit in one wrapping row.
- A collapse handle on the composer: `esc` already closes keeping the draft; add a one-key peek — while writing, `alt+↑/↓` (same as the terminal's scroll-while-writing keys) scroll the PAGE by half a screen without leaving the textarea. Put these in `?`.
- Declare the difference (`content-above-float` or similar) with a gallery variant: tall page, composer open with long text + image, scrolled to the end — last page line visible above the layer; live fidelity check.

`scripts/check.sh`. No git. Report `docs/dev/FEAT26-result.md` ≤14 lines. Reply on screen: one line.
