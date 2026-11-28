---
name: pitboard
description: Keeps a pitboard for the current session — a report the AI writes and the person reads in a viewer beside the chat, answering questions and writing thoughts that come back through `pitboard changes`. Use when the user mentions pitboard, asks to put a report on the pitboard, when the current directory or one above it has a .pitboard/ directory, or when a line starting with "pitboard:" arrives in the chat.
---

# pitboard

Before using pitboard, run `pitboard -h` and read all of it.
It is the manual of the version installed now: how to start a board, the report format,
the manners, how to read what the person did, settling thoughts, and sending.
What only some situations need is in `pitboard help TOPIC`; read the topic when its situation comes up:

| When | Read |
|---|---|
| `check` reports a problem, or you need an attribute, color or text rule `-h` does not give | `pitboard help format` |
| `changes`, `wait` or a hook brings a retract, edit or unretract line, or a title with "→ now" | `pitboard help changes` |
| `wait` or sending does not behave as `-h` says, or the person asks how a send wakes you (PITBOARD_SEND, herdr, tmux) | `pitboard help send` |
| the person asks to install the Claude Code hooks | `pitboard help hooks` |
| the person would rather see the board in a browser | `pitboard help browser` |
| keys typed after a Japanese comment are composed by the input method | `pitboard help ime` |

In short: the person answers on the board and then sends it all at once.
With the Claude Code hooks it reaches you by itself; you cannot tell beforehand, so before you stop, run `pitboard -s SESSION wait` in the background (it ends with what was sent, and you are woken) until its output tells you the Stop hook waits instead.
When a line starting with "pitboard:" arrives in the chat instead, run the command it gives.
Settle every thought (the board calls them comments), with `--doing` / `--hold` while it is not done.
A report that fails `check` is not shown to the person (they keep seeing the last good one), so fix what `check`, `wait` and `changes` report.
After a resume or a summary of your context, read the report's "Decided" (decisions and the person's standing rules) before acting: the board is your memory too.
This skill does not repeat the manual, so it never goes out of date when pitboard is updated.
