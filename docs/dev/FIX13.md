# FIX13: REVIEW10 findings

Read `docs/dev/REVIEW10-result.md` "見つけたこと" (repro steps there). Decisions:

1. Jump letters skip `g` (a..f, h..z). `g` in goto mode always leaves it.
2. `↑ カーソルは上` / `↓ カーソルは下` outranks every key hint in the bottom line (never dropped while the cursor is off-screen).
3. Jump list: at most min(3, height/3) lines (≥1). `esc やめる` always shown, first. Overflow → `… +N`. Titles shortened evenly to ≥6 columns before dropping entries. Letters still work for entries not listed.
4. Fix the `?` text to what `j`/`k` do now.
5. While in goto mode, letters are frozen (assigned on entering). If the report changes, leave goto mode and show the usual "報告が書き換わりました" notice.
6. `at`: strict `YYYY-MM-DD HH:MM` (zero-padded, valid date/time), else check error.
7. Fix only if cheap: when Notices are collapsed into `ほか N 件のお知らせ`, that is already the notice; make the right-end word of the visible Notice stay on one line (drop it if it cannot fit).
8. No change.

Done when: vet/test/build pass; 1–3 re-run per repro in a scratchpad dir (never any repo's `.pitboard/`). SPEC updated. No git.
Report: `docs/dev/FIX13-result.md`, one line per number + guesses. Reply on screen: one line.
