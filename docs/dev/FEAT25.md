# FEAT25: remove `y` / `n`; answers go through Ask. Smaller image previews.

Do after WEB8.

## A. Remove y/n reactions

Decision by the user (Ask answered: "両方やめて、問いで答える"). Evidence: in a day of heavy use — 140 comments, 94 hides, 13 `y`, 8 option answers, 0 `n`. His words: 「yとかnってあんまり使ってない感じがする」「質問で答えたらいい気もするんだよね」. One way to answer (Ask + Option) instead of two.

- Remove the `y` and `n` reactions from Items, Links, Notices and Folds everywhere: `page` actions/guards/words/refusals, terminal viewer, web (keys, bar, `?`), `changes` output, hooks text, SPEC, README, `-h`, skill, e2e, gallery variants/covers/enums (`bad` palette meaning loses `n`: keep `bad` for errors and removed diff lines only).
- Remove the `ask="y"` attribute: `check` error with the remedy in the message: `ask="y" was removed: write an <Ask> with <Option>s instead (e.g. key="a" …)`. The yellow waiting mark stays for Asks and `wait="you"` rows.
- `y` and `n` are no longer viewer keys → allowed as Option keys (`ViewerKeys` shrinks; keep the consistency test).
- Old records: `react` events with y/n in existing `events.jsonl` must not break replay; they are ignored for display and for `changes` (already delivered ones stay delivered). No migration of files.
- `-h` Manners + layout: to get a yes/no, write an Ask whose Options say what each choice means (not just はい/いいえ when you can say more: `a 入れる` / `b 見てから`). Remove every mention of y/n and `ask="y"`.
- What a person can do to a row is now: answer (Ask only), comment, hide, read. Re-check hint lines and the bar at 40/80 columns and 390px: the freed space goes to the remaining keys by `DropRank`.

## B. Image previews smaller

User, after FIX20 made them 240/320px tall: 「画像もうちょっとちいさくてもいいのかも。コメント欄の横の半分の正方形に収めるような」
- Every preview (editor attachments, placed comments under a row, comments panel) fits inside a square whose side is half the width of its text column (the textarea / the comment text column), keeping aspect ratio, never upscaled; several images wrap in a row with a small gap (so two fit side by side). Click opens the full viewer as before.
- Gallery variants updated (one large, one tall, three images; 1280/390).

`scripts/check.sh`. No git. Report `docs/dev/FEAT25-result.md` ≤12 lines (list anything that still mentions y/n on purpose). Reply on screen: one line.
