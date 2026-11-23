---
name: pitboard
description: Keeps a pitboard board for the current session — a report the AI writes and the person reads in a viewer beside the chat, reacting with y/n, answers and thoughts that come back through `pitboard changes`. Use when the user mentions pitboard or "板", asks to put a report on the board, when the current directory or one above it has a .pitboard/ directory, or when a line starting with "pitboard:" arrives in the chat.
---

# pitboard

Before using pitboard, run `pitboard -h` and read all of it.
It is the manual of the version installed now: how to start a board, the report format,
the manners, how to read what the person did, hooks, and sending.
In short: the person answers on the board and then sends it all at once.
Before you end your turn, run `pitboard -s SESSION wait` in the background; it ends with what was sent, and you are woken.
When a line starting with "pitboard:" arrives in the chat instead, run the command it gives.
Settle every thought (the board calls them comments), with `--doing` / `--hold` while it is not done.
This skill does not repeat it, so it never goes out of date when pitboard is updated.
