# FEAT27: say why "Decided" exists — it is the AI's memory too

Docs only (usage.go -h, skills/pitboard, SPEC.md). No behavior change. Do not touch git.

User (pitboard comment 588), verbatim:
> pitboard自体がAIにとっても大事な役割を果たしているということですね。それは推奨の構成？についても触れておいたほうがいいのかもですね。「決まったこと」というセクションなのかは分からないけど、そうしたセクションが必要だと。

Background: in a long session the AI's context gets summarized, or the session is resumed. Decisions and the person's standing rules ("how to work with me") that live only in chat fade. What is on the report survives, and the person can see and correct it.

Change, in "A layout to start from" item 5 and its SPEC mirror:
- "Decided" holds (a) decisions with their reason and (b) the person's standing rules about how to work (writing style, what to ask first, what never to do) — one per line.
- Say the reason plainly: it is your memory as much as theirs. Your context may be summarized or the session resumed; chat fades, the report stays. After a resume or a summary, read the report's Decided before acting.
- Keep it short (3–5 lines in -h). Keep "a starting point, not a rule"; the section name is free, the need for such a place is the point.
- Update the sample report in -h only if it helps (e.g. one rule-type line in the Fold).

Done when: scripts/check.sh passes; result in docs/dev/FEAT27-result.md (a few lines).
