# FIX15: keys must never feel dead (Notice `x`, guards)

You work ALONE in the worktree `~/dev/github.com/qawatake/pitboard/.worktrees/fixes` (branch `fixes`). Another agent is editing the main checkout (adding `internal/web`, small additions to `internal/page`). Stay inside your worktree; keep changes to `internal/page` minimal and local to ease the later merge. Commit on `fixes` when done (small commits, English). Do not push, do not merge.

Read `docs/dev/REVIEW11-result.md` (in your worktree after `git merge main` / it is on main) — the state table and proposals a–d. User report: 「お知らせでxをつけても反応しないバグがある気がする」. Implement:

a. `x` (hide) and `r` (read) work on a placed cursor (undoable, unsent until `s`, visible result). `y`/`n`/option keys keep the placed guard.
b. No silent drops. Whenever a key shown in the hint line (or any reaction key) is refused, say why in the notice line, in the terminal's wording style:
   - viewer-moved cursor (0.4 s window): do not refuse `x`/`r` at all there if that is safe; for `y`/`n`/options show `いま移った行です。もう一度押すと付きます` (and accept the next press).
   - placed + `y`/`n`/option: `j k で動かすと y n が効きます`
   - `lock` (row rewritten): existing message, shown for every refused key, not only some.
   - cursor row off-screen: `カーソルの行に戻りました`
   The hint line must not advertise a key that would be refused right now.
c. `隠しました (H で表示、u で戻す)`. Do not shrink the placed hint to bare `j k`; keep the explanation.
d. When a hidden row reappears because the AI rewrote it, show once: `隠した行が書き換わったので、また出しました: <title>`.
- A Notice whose opened body is tall (unpinned): `j` reading through it must clear `placed`.
- Put rule decisions in `internal/page` only where they are UI-independent (which actions are allowed on a not-yet-touched row); the 0.4 s timing stays in viewer.

Done when: vet/test/build pass in the worktree; pty re-run of the REVIEW11 table rows in a scratchpad dir (never any repo's `.pitboard/`). SPEC updated.
Report: `~/dev/github.com/qawatake/pitboard/docs/dev/FIX15-result.md` (main checkout path; the only file you write outside the worktree): one line per item, guesses, files touched in `internal/page`. Reply on screen: one line.
