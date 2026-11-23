# WEB5: default port by phone keypad

User: 「port番号のデフォルトはmoみたいに電話パッド方式が良いかも」 (`mo` uses 6275 = "MARK" on a phone keypad.)

- Default port for `serve`: 7482 ("PITB": P=7 I=4 T=8 B=2). Each session has its own server, so when 7482 is taken (by another pitboard session or anything else) try 7483, 7484, … up to 7491, then fall back to an OS-chosen port. `--port N` stays exact (error if taken).
- The cookie is per port; a stale cookie from another session's earlier server on the same port must just fail auth and be replaced by opening the printed URL (verify).
- `-h`, README, SPEC: mention the default and the reason for the number in one line.
- Tests: default chosen, next one when taken, exact `--port` error.

vet/test/build (e2e included). No git. Report `docs/dev/WEB5-result.md` ≤8 lines. Reply on screen: one line.
