# FIX21: REVIEW15 findings (after the second and last review round)

Do after FEAT26. Read `docs/dev/REVIEW15-result.md` (findings, d renderings). Fix every 中 and the 低 that are cheap; decisions:

- Terminal palette (the 5 leftovers): Code blocks and inline code = normal foreground (inline code keeps a non-color cue only if one exists already, e.g. none); editor heading = `structure`; plain status messages = normal, refusals = `need`, report problems = `bad`; goto letters/digits = `structure`; diff `@@` = `quiet`, `+` = `move`, `-` = `bad`. Purple disappears. Update SPEC 143 wording if needed, e2e color assertions, the palette table in `page`.
- Web 390: row titles on the same text edge as headings/paragraphs/bar (measure all pages incl. index at 390; add the measurement to the gallery DIFFS so it cannot regress at either width).
- Web: the top band always has its boundary (hairline + gap) when anything scrolls under it, with or without Notices; gallery variant for the no-notice case.
- Gallery fidelity compares EVERY variant that can be reproduced on a live board (all parts incl. code/text/image/comment, and home), not 17; a class added only to `.code` must fail.
- Web 390 definition layout: below ~480px the body goes back to the head's column (no extra indent); loose spacing stays.
- Terminal, scrolling while writing: the hint line stays pinned at the bottom whenever the editor is open, even when the input is off-screen, and says so (`書いている途中 — 何か打つと戻る`, words from `page`); the off-screen draft is never lost: `esc` keeps the draft for that target for the viewer's lifetime and reopening restores it (same contract as the web); `j` while off-screen types `j` as now but the view returns first (already) — verify.
- Terminal editor hints at 40 columns keep the newline key for the keyboard protocol actually in use (kitty/Ghostty: `shift+enter 改行`, shortened to `S-enter 改行` if needed).
- Web `?`: `↑↓` described as on the terminal? No: on the web the document scrolls natively and arrows move the cursor row today — make the web match the terminal: `j k` move the row, arrows scroll the page (default browser behaviour, do not intercept); fix `?` and bar texts.
- Removed-row warning gaps: rows are remembered when the PERSON'S viewer/page has shown them (viewer and web record the set of row ids+titles they displayed into the session's state, small and deduped), not only when the AI ran a command; a reaction/answer given to an older title does not count as answered for the current row (use the same staleness rule the mark uses).
- Not changing: long logs written as plain paragraphs get joined (that is the format); `-h` already says to use Code. No check for it.

`scripts/check.sh`. No git. Report `docs/dev/FIX21-result.md` ≤14 lines. Reply on screen: one line.
