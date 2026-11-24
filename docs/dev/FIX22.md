# FIX22: pressing x twice does not hide two rows in a row

Person's words: 「x を 2 回続けて押して、2 行を続けて hide できない」

Expected: `x` hides the cursor row and the cursor moves to the next row; pressing `x` again right away hides that next row too, and so on. Same on the terminal viewer and on the web page (check both; the person may have seen it on either).

Start here: terminal `internal/viewer/keys.go:206` (`app.hide`, `pv.moveFrom`), shared op `internal/page/ops.go:118` (`Board.Hide` — refuses with `ErrChanged` when the row's Key/Sig/HideKey no longer match what was seen; suspect the second press sees a stale row/cursor, a report reload or the moved cursor pointing at a non-hideable/stale node). Find the actual cause by reproducing it first (e2e or a unit test that presses x twice), then fix it.

- Add a regression test (terminal and web if both affected) that hides 3 consecutive rows with 3 presses.
- u after the presses undoes the last hide only (keep current undo contract).
- Do not touch other features; no go.mod changes.

`scripts/check.sh` must pass. No git. Report `docs/dev/FIX22-result.md` ≤10 lines (cause with file:line, fix, tests). Reply on screen: one line.
