---
name: pitboard
description: Keeps a pitboard for the current session — a report the AI writes and the person reads in a viewer beside the chat, answering questions and writing thoughts that come back through `pitboard changes`. Use when the user mentions pitboard, asks to put a report on the pitboard, when the current directory or one above it has a .pitboard/ directory, or when a line starting with "pitboard:" arrives in the chat.
---

# pitboard

Before using pitboard, run `pitboard -h` and read all of it.
It is the manual of the version installed now: how to start a board, the report format,
the manners, how to read what the person did, hooks, and sending.
In short: the person answers on the board and then sends it all at once.
Before you end your turn, run `pitboard -s SESSION wait` in the background; it ends with what was sent, and you are woken.
When a line starting with "pitboard:" arrives in the chat instead, run the command it gives.
Settle every thought (the board calls them comments), with `--doing` / `--hold` while it is not done.
The board shows the person "AI の番" (your turn) from their send until you rewrite the report or settle (--done or --hold) every thought it carried, and while a thought is --doing; so settle --doing at once when work takes long, and --done when it is finished. It is also "AI の番" whenever report.md fails `pitboard check` or is missing: fix it.
Put `look="y"` only on lines you want the person to read (answers, results to check), not on progress lines.
A report that fails `check` is not shown to the person (they keep seeing the last good one), so fix what `check`, `wait` and `changes` report.
After a resume or a summary of your context, read the report's "Decided" (decisions and the person's standing rules) before acting: the board is your memory too.
This skill does not repeat it, so it never goes out of date when pitboard is updated.
