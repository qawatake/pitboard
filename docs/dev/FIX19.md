# FIX19: lists that read — spacing between wrapped items, head words that do not depend on bold

Do after FIX18. Terminal and web (rule in `internal/page`, both fronts follow).

User (screenshot of an opened Item with five `- **head**: long sentence…` bullets, each wrapping to 2–3 lines, no gaps): 「またしても読みづらい。行と行の間隔がないこととか文字数が単純に多いこととか」. In the screenshot the CJK head words are NOT visibly bold (only the ASCII `3` and `builder` are): the user's terminal font has no bold CJK face, so `**語**:` run-in heads disappear into the sentence.

1. Loose lists: if ANY item of a list (same level, same parent) takes more than one drawn line at the current width, draw one blank line between all items of that list (terminal: an empty row; web: an item gap token ≈ .5em). Lists whose items all fit on one line stay tight. Nested lists decide for themselves.
2. Head words (`- **head**: body` and paragraph `**head**: body`): when `head: body` fits on ONE line → as today (run-in). When it does not fit → definition layout: the head alone on the first line (no colon), the body on the following lines indented one step under it. So the head stands out by position, not by weight.
3. Head emphasis without relying on bold: besides bold, give head words (and small headings `**…**` alone on a line, and opened titles — everything currently "bold") a second, non-color cue only where needed: terminal = keep bold AND make the BODY of a definition item one shade quieter? NO — do not dim body text (readability). Use position (rule 2) as the cue; additionally, for small headings alone on a line keep the blank line above (already) — nothing else. Do not add colors.
4. Width: wrap list/paragraph text at min(available, 72 columns of text) in the terminal so long lines do not run edge to edge on wide panes (web already has a max width).
5. `-h` Manners (two sentences): a list line is one short clause — if it needs a sentence or two, use a head word with the body, or sub-items; keep an Item's contents to what the person must read, not everything you know.
6. e2e/unit: loose vs tight decision at widths 40/80/120; definition layout switch; gallery variants (`text`: tight list, loose list, run-in head, definition head) + `differences` declared.

vet/test/build + `scripts/check.sh`. No git. Report `docs/dev/FIX19-result.md` ≤10 lines. Reply on screen: one line.
