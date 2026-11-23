# REVIEW15: review at 5ab4829 (WEBFIX3, FIX18, FIX19) — second and last round

Do not change code. Write only `docs/dev/REVIEW15-result.md`. Setup as REVIEW14 (git archive 5ab4829; scratch boards; temp `XDG_STATE_HOME`; `--port 0`; never touch the user's real server on 7482 or real state dir; `playwright-cli` only; no real herdr). Stop everything; prove with `ps`.

Read: `REVIEW14-result.md`, `WEBFIX3.md` + result, `FIX18.md` + result, `FIX19.md` + result.

a. REVIEW14 regression: every finding (高/中/低) → fixed / partly / not, measured on the LIVE page; and whether the gallery now tells the truth (fidelity check: try to break it — make a variant diverge in your scratch copy and see that `scripts/check.sh` fails; add a `differences` name with no variant; a `covers` value with no variant).
b. Run `scripts/check.sh`; look at every gallery image again with the layout eye (alignment, rhythm, hierarchy, the six palette meanings — any use outside its meaning?). New since last time: home, floating composer, wrapped Code, loose lists, definition heads, bars, banners.
c. Terminal (pty 40/80/120): scroll keys in normal mode / comment pane / list / while writing (text preserved, cursor hidden off-screen, typing brings it back), `k` reading back up, alternate-scroll request/restore sequences, ↑/↓ now scroll (anything still documenting them as cursor keys?), loose lists and definition heads at each width (does the 72-column cap look right at 120? right-end words column?), Code wrap with tabs, hints at 40.
d. Readability of real content: take the user's two complaint screenshots as the bar (an opened Item with five long `- **head**: sentence` bullets; an Item whose body was a 40-line log). Recreate both as reports and show how they render now in terminal (80) and web (1280/390): paste the terminal rendering as text in the result and give screenshot paths. Say plainly whether it reads well now and what still hurts.
e. One server leftovers: drafts per board, id tombstones, forget → gone → recover, registry serialization, perms.
f. Removed-row warning after simplification: shown once, cases, no `ack-removed`.

Report: 5-line summary; findings by severity; the renderings for d; unverified. Reply on screen: one line.
