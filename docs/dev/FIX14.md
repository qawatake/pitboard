# FIX14: remove `re`; the AI writes the quote itself

User, after trying FIX12 (tool-rendered quote):
「やっぱり、ここは機械的にやるよりも見え方含めてAIにまかせるのがいいんじゃないかとか思うんだよね。要約するケースもあるだろうし」

- Remove the `re` attribute and everything it does (parser, page `Quote*`, viewer drawing, tests, SPEC, `-h`). `re="…"` becomes an unknown-attribute check error like any other.
- `-h` manners + layout text for answers: the Item's title is `> ` + the person's question in their words, shortened or summarized when long (one line is best); the first line of the contents is the conclusion in one sentence; details after it. One answer Item may cover several comments. When shortening, never stop mid-sentence (user saw a title ending in 「…PR を」 and asked 「これ途切れてるのは意図的？？」): cut at a clause boundary and make it read complete, or summarize.
- Nothing else changes (an Item title starting with `> ` is plain text; no special drawing).

Done when: vet/test/build pass. No git.
Report: `docs/dev/FIX14-result.md`, ≤6 lines. Reply on screen: one line.
