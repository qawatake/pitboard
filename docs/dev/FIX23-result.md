# FIX23 result

- Undone from FIX22: `Touch.Hid` (guard.go, guardtable `Hid` cases), viewer `hidTo`, web `Cur.How = "hid"` / state.js `hiding`; restored old assertions in TestHide (e2e), TestHideAndUnhide, TestSettlingSaysWhy and the guard tables.
- SPEC.md line 31 back to the settle rule; line 101 says the moved-to line waits 0.4 s and a quick second x on the web is dropped there (does not unhide).
- Kept: main.js `press` queue; `Board.Hide` returning `ErrChanged` for a hidden line when hidden lines are not shown.
- FIX22 regression tests rewritten to the rule (e2e/react_test.go TestHideRowsInARow, internal/viewer/fix22_test.go, internal/web/fix22_test.go): x, then x x on each moved-to line; the first x there gives 「いま移った行です。もう一度押すと隠れます」.
- `go vet ./... && go test ./...` pass. Gallery not run: playwright-cli is not installed.
