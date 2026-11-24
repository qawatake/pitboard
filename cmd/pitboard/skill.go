package main

// skill is the Claude Code skill that points an AI at pitboard -h
// (pitboard --skill > ~/.claude/skills/pitboard/SKILL.md). It says nothing
// itself, so it does not go out of date when pitboard changes.
//
// It is the text of skills/pitboard/SKILL.md (which gh skill install reads),
// kept here because go:embed cannot reach above this directory. SKILL.md is
// the one to edit; TestSkillMatchesSKILLmd fails until this follows it.
const skill = `---
name: pitboard
description: Keeps a pitboard for the current session — a report the AI writes and the person reads in a viewer beside the chat, answering questions and writing thoughts that come back through ` + "`" + `pitboard changes` + "`" + `. Use when the user mentions pitboard, asks to put a report on the pitboard, when the current directory or one above it has a .pitboard/ directory, or when a line starting with "pitboard:" arrives in the chat.
---

# pitboard

Before using pitboard, run ` + "`" + `pitboard -h` + "`" + ` and read all of it.
It is the manual of the version installed now: how to start a board, the report format,
the manners, how to read what the person did, hooks, and sending.
In short: the person answers on the board and then sends it all at once.
Before you end your turn, run ` + "`" + `pitboard -s SESSION wait` + "`" + ` in the background; it ends with what was sent, and you are woken.
When a line starting with "pitboard:" arrives in the chat instead, run the command it gives.
Settle every thought (the board calls them comments), with ` + "`" + `--doing` + "`" + ` / ` + "`" + `--hold` + "`" + ` while it is not done.
Put ` + "`" + `look="y"` + "`" + ` only on lines you want the person to read (answers, results to check), not on progress lines.
After a resume or a summary of your context, read the report's "Decided" (decisions and the person's standing rules) before acting: the board is your memory too.
This skill does not repeat it, so it never goes out of date when pitboard is updated.
`
