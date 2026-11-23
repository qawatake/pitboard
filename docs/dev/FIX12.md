# FIX12: `re` quote becomes the row's title line

Context: FEAT23 added `re="N"` on Item; the viewer draws the quoted comment dim, ABOVE the title line. User rejected it (screenshot):
「普通に濃い色で>がよかったかも。そのトピックが扱われているんだ、というのがひと目で分かるのが良かったし、セマンティック的にもカーソルより上に補助情報がでるのは気持ちが悪い」

Change (page layer + viewer; keep page as the single source of the rule):
- Item with `re` whose thought exists: the row's FIRST line (the cursor line, where marks/right-end words sit) is `> ` + the comment text, normal weight/color (not dim), `>` same color as text. Wrap fully like any title; cap at 3 lines with `…`. Newlines in the comment become spaces.
- The AI-written title (the conclusion) goes on the next line(s), aligned with the quote text (after `> `), normal color, always visible (not part of the openable body). Wrap fully.
- Body opens/closes as before (`▸`/`▾` at the row head as for any openable Item).
- Nothing is drawn above the cursor line.
- Thought missing/retracted: draw as a plain Item (AI title on the first line).
- hide key / unread key: unchanged from FEAT23.
- `-h`: adjust the description of `re` (what the person sees). SPEC likewise.

Done when: vet/test/build pass; pty check at width 80 and 40 in a scratchpad dir (never touch any repo's `.pitboard/`). No git.
Report: `docs/dev/FIX12-result.md`, ≤10 lines: changes (1 line each), guesses. Reply on screen: one line.
