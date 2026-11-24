# WEB9: clear boundaries for floating layers and images (web)

Do after FEAT26, BEFORE FIX21.

User (screenshot: the `m` composer over the page — white on white, the shadow barely visible; three pasted screenshots shown as large white images that melt into the page and into each other) + a ChatGPT composer screenshot as reference (one rounded card with a visible soft edge; the attached image is a small rounded thumbnail at the top INSIDE the card, above the text): 「全体的に境界が見えづらい印象。chatgptとかを参考になるかな？」

Earlier rule "no cards/borders anywhere" was about the PAGE CONTENT (rows). Keep that. Things that float above the page, and images (foreign content), need visible edges:

1. One "surface" style for every floating layer (composer, comments panel, `?` dialog, image viewer frame): background a step off the page color (light: pure white on a very slightly gray page? no — keep the page white and give the surface: 1px border in `line` color + the two-layer soft shadow, visibly stronger than now) and radius 14px; in dark theme: surface one step lighter than the page + 1px border. The edge must be findable at a glance in both themes — verify by contrast of border vs page ≥ 1.3:1 and by screenshot.
2. Composer laid out like the reference: a single card. Inside, top to bottom: (a) attachments row (only when there are images), (b) the text area with NO border of its own (the card is the field; focus shows on the card: border in `structure` color), (c) nothing else. Title 宛先なしのコメント stays as a small label above the text inside the card (quiet color, not a blue heading). Inline editor under a row (the `c` editor): same card style in place (not floating), so the field's edge is clear there too.
3. Attachments: fixed small thumbnails, 88px squares, `object-fit: cover`, radius 10px, 1px border, 8px gap, wrapping; a small × button on each (removes the image reference from the text too; keyboard reachable; undo via normal text undo is not required). This replaces the "half-width square" sizing from FEAT25-B in the editor. Click a thumbnail = full viewer.
4. Images in placed comments (under a row, in the comments panel): same thumbnail treatment (88px, border, radius) in a row under the comment text; click = viewer. The viewer shows the image with a border on a scrim, so white screenshots do not melt into a white page.
5. The pinned band and the bar keep their hairlines (already). Nothing else gains borders.
6. Gallery: surface variants in both themes (composer empty / with text / with 1, 3, 7 images; inline editor; panel; `?`; viewer with a white image), a DIFFS check that floating layers have a visible border (computed border-width ≥ 1 and color ≠ background), live fidelity.

`scripts/check.sh`. No git. Report `docs/dev/WEB9-result.md` ≤10 lines + gallery path. Reply on screen: one line.

## Correction (user's follow-up) — overrides points 2(a), 3 and 4 above

User: 「画像の位置自体はそんなに悪いと感じなかったけどなぁ。chatgptは小さすぎるんじゃないかとも思ったけどどうなんだろう」 — I over-copied the reference. The complaint is the missing EDGES, not position or size.
- Image position: unchanged — previews stay directly UNDER the text area (inside the same card for the composer / inline editor), and under the comment text for placed comments.
- Image size: unchanged from FEAT25-B (fit inside a square of half the text column's width, aspect kept, never upscaled, wrapping row). No 88px thumbnails, no cover-cropping.
- What IS added to images: 1px border in `line` color, radius 8px, a clear gap between images (≥ 12px) and from the text; a small × on each editor preview. In the viewer, the image has a border on the scrim.
- Everything else in WEB9 (surface style for floating layers, borderless textarea inside the card, focus on the card, label instead of blue heading, DIFFS checks, gallery variants) stands.
