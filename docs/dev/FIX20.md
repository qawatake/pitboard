# FIX20: drop the summary paragraph from the suggested layout

Do after WEB7. `-h` "A layout to start from" item 1 says: the title, and where things stand in a sentence or two. In practice the AI did not keep that paragraph current (it went stale for hours) and the person asked 「タイトルの下の要約ってあんまりメンテされてないよね？要る？？」. "For you" and "In progress" already say where things stand, and they are maintained because work flows through them.

- Layout item 1 becomes: the title only (what this board is about; rarely changes). Remove the "where things stand" sentence; add one line of reasoning: do not keep a summary that duplicates the sections — it goes stale.
- Update the example report in `-h`, SPEC/README if they show the summary paragraph as recommended. Paragraphs under the title remain allowed (format unchanged).
- Also: the index page heading says `ボード`; use `pitboard` (the user's rule: call it pitboard, not 板/ボード). Check all Japanese UI strings for ボード/板 again (web included) and extend `TestNoOldWords` to the web strings and `page` texts.

## Also (web editor)

User (screenshot of the line under the textarea: 「s で送るまで AI には届かない · esc でやめても書いたものはこのタブに残る · 画像は貼り付け・ドロップ」): 「ここの説明が丁寧すぎる気がする。あと、画像previewが小さすぎないか？？いいところにもうちょっと大きめに表示するとよさそう」
- Remove that explanatory line under the textarea entirely (composer and inline editor). The bar already shows the keys; nothing replaces it. (The same facts stay in `?`.)
- Image previews: in the editor, show attached images directly under the textarea at a readable size — each up to 240px tall and up to the editor's width, keeping aspect ratio, in a wrapping row; never upscale beyond natural size. Placed comments (under a row and in the comments panel) show their images at up to 320px tall / column width. Click still opens the full viewer. `[Image #N]` in the text stays as the reference.
- Gallery variants for: editor with one large image, with three images; comment with image at the new size; 390 width.

vet/test/build + `scripts/check.sh`. No git. Report `docs/dev/FIX20-result.md` ≤6 lines. Reply on screen: one line.
