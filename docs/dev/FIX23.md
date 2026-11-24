# FIX23: back to the settle rule for x (undo the rule part of FIX22)

The person chose to keep the rule from before FIX22: on a line the screen moved the cursor to (incl. after x hid the previous line), the first x within 0.4 s is dropped with 「いま移った行です。もう一度押すと隠れます」, and the same x again on that line works (as SPEC said before FIX22). Their reason: a quick double press already works, so x x x does not need to skip the guard.

Undo from FIX22 (commit db13104, see `git show db13104`): `Touch.Hid` and everything that sets/uses it (guard.go, guardtable, viewer `hidTo`, web `Cur.How = "hid"`), the SPEC.md lines 31 and 101 wording, the tests asserting the skip (restore the old assertions of `TestHide`, `TestHideAndUnhide`, `TestSettlingSaysWhy`, guard table cases).

Keep from FIX22 (they are bug fixes, not the rule):
- web presses run one after another (main.js `press` queue), so a second x never goes to the row just hidden;
- `Board.Hide` refuses to show again a hidden line when hidden lines are not shown (`ErrChanged`).
Rewrite FIX22's regression tests to the restored rule: pressing x on 3 rows means x, then x x on each moved-to line (or waiting 0.4 s); on the web a quick second x must not unhide the hidden row (it is dropped with the settling message instead).

Do not touch FEAT29/FEAT30 work (report.go, Summary/turn words) beyond conflicts. No go.mod changes. No company or personal names.

`go vet ./... && go test ./...` must pass (gallery needs playwright-cli, absent: say so). No git. Report `docs/dev/FIX23-result.md` ≤8 lines. Reply on screen: one line.
