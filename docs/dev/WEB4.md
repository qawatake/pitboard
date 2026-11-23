# WEB4: page layout — one left edge, hanging gutter

User screenshot of the real page (light, ~1000px) + words: 「決まったことのインデントとか意図的ですか？ちゃんとレンダリングのチェックをcheckerはしてくれているのか？？」

What it shows (all wrong):
- Section headings (`あなたへ`, `進行中`) and paragraphs start at x=0 of the column; Item titles start ~6ch further right (after cursor/mark/sign columns); the top-level Fold heading `▸ 決まったこと (20)` is indented like an Item, not aligned with the Section headings it is a peer of.
- The current row has a dotted box around the whole row (focus outline) in addition to `>`.
- An empty Section (`あなたへ` with nothing in it) is a bare heading.

Rules to implement (CSS/DOM only unless noted):
1. One text edge. Section headings, top-level Fold headings, top-level paragraphs/lists, and top-level Item/Ask/Link TITLES all start at the same x. The gutter (cursor `>`, mark ●, sign ▸ ▾ · ? → ✓ !) hangs to the LEFT of that edge, inside the page's left padding (reserve it there; at 390px wide too). A top-level Fold's `▸` hangs in the gutter exactly like an Item's sign.
2. Nesting: contents of an opened Item / Fold / Notice are indented one step (≈1.5em) from their parent's text edge, and that becomes the text edge for everything inside (titles, paragraphs, lists; signs of nested rows hang left of the nested edge). Cursor and ● stay in the outermost gutter columns for every depth.
3. Right-end words share one right edge for all depths.
4. Current row: `>` only. No outline/box on the row, also when focused by keyboard (the `>` is the focus indicator; make it clearly visible in both themes). Keep a thin `:focus-visible` outline for bottom-bar buttons, textarea and dialog buttons.
5. Vertical rhythm: same gap above every Section/top-level Fold heading; rows evenly spaced; a wrapped title keeps its lines tight (line-height) so a 2-line row reads as one row.
6. Empty Section: show a faint `なし` under the heading — decide this in `internal/page` (a field/Thing) so the terminal shows the same faint line; update SPEC and any e2e/unit expectations accordingly.
7. Pinned Notices follow rule 1 (their `!`/`▸` in the gutter, title on the text edge).

Verify with `playwright-cli` (never system Chrome; `--no-open`; scratch board) at 1280 and 390, light and dark, with a report that has: empty Section, Section with paragraph + Items (one 2-line title with a right-end word), top-level Fold closed and opened (containing paragraph, list, Items, nested Fold), Notice closed/opened, Ask opened. Assert alignment by measuring `getBoundingClientRect().left` of the first text glyph box of: a Section heading, a top-level Fold title, a top-level Item title, a top-level paragraph → all equal; nested ones equal to each other at +1 step. Put these measurements in the result. Save screenshots under scratchpad and list paths. Stop every server you start.
vet/test/build (e2e included). No git. Report `docs/dev/WEB4-result.md` ≤20 lines. Reply on screen: one line.
