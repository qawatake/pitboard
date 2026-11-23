# REVIEW11: investigate — "x on a Notice does not react"

Do not change code. Write only `docs/dev/REVIEW11-result.md`.

User report: 「お知らせでxをつけても反応しないバグがある気がする。気の所為？」 (The hide did eventually reach the AI, so it works in some state and not in another.)

Setup: `git archive 693db54 | tar -x -C <scratchpad dir>`, `go build ./cmd/pitboard`, run only in a scratchpad dir via pty. Never touch any repo's `.pitboard/`.

Find every state in which `x` on a Notice is ignored or appears to do nothing, e.g.:
- right after opening the viewer (cursor placed on the Notice; the "placed cursor accepts no reaction" guard), with/without moving first
- right after the report was rewritten (changed-row guard, 0.4 s viewer-moved guard)
- Notice opened (body shown) vs closed; Notice unpinned because its opened body is tall; several Notices; after `g` jump to it
- when there are unsent items (x means retract elsewhere?), in the comment pane/list
- whether any message is shown when the key is ignored; whether the Notice visibly disappears at once after a successful x
Also check the same for `r`, `y`, `c` on a Notice.

Report (≤25 lines): table of state → what x did → message shown; the most likely thing the user hit; a proposed rule that makes it never feel dead (which guarded keys should work on a placed cursor because they are undoable, and what to show when a key is refused).
Reply on screen: one line.
