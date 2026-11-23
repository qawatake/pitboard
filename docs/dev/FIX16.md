# FIX16: tell the AI when it removed a row the person is still talking about

Do after WEBFIX2. Small.

What happened: the AI rewrote report.md and dropped two answer Items the person had NOT hidden; the person's next comment on one of them arrived as `… → now gone`. The AI only noticed from that. A manners line ("remove only hidden or resolved rows") is not enough (user's standing rule: fixes must be mechanisms, not "be careful").

- `check` (and therefore the note every AI-run command appends) gets a WARNING class, separate from problems (exit code unchanged, viewer does not show it): for each id that existed in the report as the person last saw it (use the `viewed`/state the board already records, or record the set of row ids+titles at each `changes`/`wait` delivery in the session dir — your call, keep it small) and is now gone, warn if (a) it had `look="y"`/`ask="y"`/was an Ask and the person had not reacted/read/hidden it, or (b) it has a comment not yet `対応済`:
  `warning: row "a-flat" (> storybook がいいとは…) was removed but the person has not hidden or answered it` / `… has an open comment (N)`.
  Shown once per removal (cleared when the AI runs `pitboard -s S ack-removed` or the row comes back or the comment is settled done).
- Appended line format: `report.md has N problem(s), M warning(s). Run: pitboard -s <id> check`.
- `-h` manners: when to remove a row; that the tool warns.
- Tests + e2e for both cases. SPEC.

vet/test/build. No git. Report `docs/dev/FIX16-result.md` ≤10 lines. Reply on screen: one line.
