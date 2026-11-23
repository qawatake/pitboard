# WEB7: floating layers sit low-center; softer shadow

User: 「ブラウザの宛先なしコメントですが、右下は視線が変わりすぎるので真ん中の下方あたりにモーダルを配置してほしい。shadowはもうちょっと自然にできるといいかな」

- The floating composer (`m`): horizontally centered on the page's text column (same left/right edges as the column, max the column width), vertically in the lower part — bottom edge a small gap above the bar; grows upward with the text. At ≤480px: unchanged (sheet above the bar).
- Same placement for the comments panel (same eye-travel problem; one position token for "floating layer" so the two cannot drift). Note this in the result as my decision so the user can say no.
- Shadow: one token used by every floating layer. Soft and natural: two layered shadows (a tight low-opacity contact shadow + a wide very-low-opacity ambient one), no hard hairline plus shadow together — pick hairline OR shadow per theme (light: shadow only; dark: hairline + faint shadow, since shadows vanish on dark). No colored glow.
- Gallery variants updated (composer, panel; 1280/390 × light/dark); fidelity check passes; `scripts/check.sh` passes.

No git. Report `docs/dev/WEB7-result.md` ≤8 lines with the gallery output path. Reply on screen: one line.
