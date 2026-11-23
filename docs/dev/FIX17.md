# FIX17: gallery fixtures must not look like secrets

The pre-commit secret scanner (gitleaks, rule `generic-api-key`) rejects the WEBFIX2 commit: 43 hits, all in `internal/web/static/gallery/fixtures.json` — the per-row `Token` values (32-hex content hashes). They are not secrets, but the hook must stay un-bypassed and no allowlist is to be added.

- In fixture generation only (`internal/web/gallery_test.go` and whatever it calls), replace every token-like value (row `Token`, `Seen` tokens, selection tokens, anything hex/base64 ≥ 16 chars) with short stable placeholders derived from position/id (`row-1`, `row-2`, … or the row id), consistently wherever the same token is referenced, so the gallery still works. Do not change how real tokens are made.
- Check: run `gitleaks protect --staged --no-banner` after `git add -A` (then `git reset -q` to unstage; do not commit) → must report no leaks. Also make `TestGalleryFixtures` fail if a fixture contains a `[0-9a-f]{16,}` or base64-looking run, so this cannot come back.
- Regenerate fixtures; gallery script still passes.

Do this AFTER FIX16's result file is written (or now, if FIX16 is already done). vet/test/build. No commit.
Report `docs/dev/FIX17-result.md` ≤6 lines. Reply on screen: one line.
