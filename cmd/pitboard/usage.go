package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// usage is pitboard -h: what every session needs, read in full. What is
// needed only now and then is a topic of 'pitboard help TOPIC', listed at
// its end. The keys an Option cannot use come from report.ViewerKeys and the
// line 'wait' prints from board.StopHookNote, so they never disagree.
var usage = fill(usageText) + "\n" + topicList()

// helpTopic is one part of the manual read only when it is needed.
type helpTopic struct {
	name    string
	summary string // one line: when to read it
	text    string
}

var topics = []helpTopic{
	{"format", "every part and attribute, colors, ids, text rules; read when check reports a problem", formatText},
	{"changes", "every line 'changes' prints: retract, edit, unretract, \"→ now:\"; read when one arrives", changesText},
	{"send", "how a send wakes you: wait in detail, PITBOARD_SEND, herdr, tmux", sendText},
	{"hooks", "the Claude Code hooks (~/.claude/settings.json)", hooksText},
	{"browser", "pitboard serve: the board in a browser", browserText},
}

var filler = strings.NewReplacer("{viewerKeys}", report.ViewerKeyList(), "{stopHookNote}", board.StopHookNote)

func fill(s string) string { return filler.Replace(s) }

// topicList is the end of -h and what 'pitboard help' prints.
func topicList() string {
	var b strings.Builder
	b.WriteString("Topics (pitboard help TOPIC):\n")
	for _, t := range topics {
		fmt.Fprintf(&b, "  %-9s %s\n", t.name, t.summary)
	}
	return b.String()
}

// runHelp is 'pitboard help [TOPIC]'.
func runHelp(args []string, out io.Writer) error {
	switch len(args) {
	case 0:
		fmt.Fprint(out, topicList())
		return nil
	case 1:
		for _, t := range topics {
			if t.name == args[0] {
				fmt.Fprint(out, fill(t.text))
				return nil
			}
		}
	}
	names := make([]string, len(topics))
	for i, t := range topics {
		names[i] = t.name
	}
	return fmt.Errorf("no help topic %q (topics: %s)", strings.Join(args, " "), strings.Join(names, " "))
}

const usageText = `pitboard - a board beside an AI session: the AI reports, the person answers

Contents: Usage, Starting, Writing report.md, A layout to start from,
Manners, Reading 'changes', Thoughts, Send, Topics (read with
'pitboard help TOPIC' when the situation named there comes up).

Usage:
  pitboard [-s SESSION] [view]     the viewer (without -s: the most recently updated session)
                                   (-s may also come after the command)
  pitboard [-s SESSION] serve      the same board in a browser (pitboard help browser)
  pitboard -s SESSION path         print the path of the session's report.md (write the report there)
  pitboard -s SESSION check        check report.md; prints problems with line numbers, exit 1 if any,
                                   and warnings about removed rows (they do not change the exit code)
  pitboard -s SESSION changes      print what the person has sent since the last call, one per line
  pitboard -s SESSION settle N [--doing|--hold|--done] TEXT
                                   set thought N's state and what you did with it
                                   (several: --STATE N TEXT --STATE N TEXT ...)
  pitboard -s SESSION open         list the thoughts still --doing or --hold (nothing if none)
  pitboard -s SESSION wait [--timeout DURATION]
                                   wait until the person sends, then print what
                                   'changes' prints (see Send)
  pitboard hook post-tool|stop     for Claude Code hooks (pitboard help hooks)
  pitboard help [TOPIC]            one topic of this manual (the list is at the end)
  pitboard -h                      this help
  pitboard --skill                 print the Claude Code skill (SKILL.md); it points here

The board holds what must not scroll away in the chat: where things stand,
what the person must decide, what they should look at. You (the AI) write
report.md; the person answers Asks and writes thoughts on the board (into
events.jsonl, which you never edit). You read them with 'changes', rewrite
the report yourself, and answer thoughts with 'settle'.

Starting:
  1. Your session id is your Claude Code session id: the last directory but
     one of your scratchpad path (.../<project>/<session-id>/scratchpad).
     In commands, write only its first 8 characters (ID below), e.g.
     -s 0a1b2c3d for 0a1b2c3d-4e5f-...: the form pitboard itself prints.
     ($PITBOARD_SESSION may stand for -s; a unique prefix of an existing
     session is enough.)
  2. 'pitboard -s ID path' prints where the report goes (the first call
     makes the board in the nearest .pitboard/ above, or here). Write that
     file with Edit/Write. Commands find the board from the current
     directory upwards.
  3. Ask the person to open the viewer beside the chat: pitboard -s ID

Writing report.md:
  A line starting with <Name ...> opens a part and </Name> closes it (tags go
  at the start of a line, indentation is fine; part names start with a
  capital letter). Every other line is text: paragraphs and "- " list lines
  (2 spaces per level), with inline Markdown (` + "`code`" + `, **bold**,
  [title](url)); [text](#id) links to the row with that id. Attributes are
  name="value"; write " as &quot;. (Text rules in full: pitboard help format.)

    <Notice id="reopen" at="2026-11-23 21:46">Reopen pitboard
      I installed a new version: quit with q and run pitboard again.
    </Notice>
    <Title>Build the minimal pitboard</Title>

    <Section title="For you">
      <Ask id="hook">Install the hook? (it touches ~/.claude/settings.json)
        <Option key="a">install it</Option>
        <Option key="b">look at it first</Option>
      </Ask>

      <Item id="a-cli" look="y">> When does the CLI come?
        Next, after the viewer: [Add the CLI](#cli)
      </Item>
    </Section>

    <Section title="In progress">
      <Item id="impl" state="doing">Implement the viewer
        <Code lang="diff">
        -old
        +new
        </Code>
      </Item>
      <Link id="pr" url="https://github.com/OWNER/REPO/pull/1">The pull request</Link>
      <Item id="cli" status="queued">Add the CLI</Item>
      <Item id="parts" status="waiting on you" wait="you" note="needs the answer above">Fix the part list</Item>
    </Section>

    <Fold id="decided" title="Decided (1)">
      <Item id="r-chat" state="done">Rule: in the chat, one or two lines; details go on the board</Item>
    </Fold>

  Parts (anything else is an error; every attribute: pitboard help format):
    Title     the page title, one line
    Section   title (required). Holds Item Ask Link Code Fold and text.
              At the top level only.
    Fold      title (required), id (optional). Holds what a Section holds;
              may be at the top, in a Section or in a Fold. Starts closed.
    Item      id (required), state = todo | doing | done, status (a short
              word at the right end), note, wait="you", look="y". First
              line: the item; the lines after it (text, Code) show when
              the person opens it.
    Ask       id (required). First line: the question, written to make sense
              without the options (the board shows it closed). Then 2 or
              more Option.
    Option    key (required, one letter a-z, unique in its Ask). One line.
    Link      id (required), path or url (one of them). One line: its title.
    Notice    id (required), at="YYYY-MM-DD HH:MM". Shown above the page, in
              yellow. At the top level only, 3 at most.
    Code      lang (optional; "diff" is colored). Raw until </Code>.
  id: 1-16 of a-z 0-9 -, unique in the page. The person answers an Ask,
  writes thoughts on any row with an id, hides lines and says they read them.

  Marks: a yellow ● beside each Ask not answered yet, and a green ● beside
  each look="y" line the person has not read since it last changed (rewriting
  it marks it unread again). Only the person clears them. So: make an Ask of
  exactly what you need answered, and put look="y" exactly on what you need
  read (an answer you wrote, a result to check), never on progress lines.

A layout to start from:
  The layout is yours; this is a starting point. From the top:
    1. The title only: what this board is about. No summary paragraph under
       it: it repeats the sections below and goes stale.
    2. Notices, only when there is one (see Manners).
    3. "For you": first what the person is to answer or do (Ask, Item with
       wait="you"); under them, your answers to what they asked. (When it
       is empty, they know they need do nothing.)
    4. "In progress": what you are doing now (state="doing"; the board says
       作業中 itself) and next, with a status only for something else:
       waiting on you, queued, on hold.
    5. "Decided": in a Fold, one per line: each decision with its reason,
       and the person's standing rules on how to work with them. It is your
       memory as much as theirs: after a resume or a summary of your
       context, read it before acting.
  The more a section needs the person's hand, the higher it goes; leave out
  an empty one.

Manners:
  - The report is skimmed, not read. One item per line, its title one
    sentence; details go in its contents (they should fit one screen). A
    long explanation gets a Section of its own, pointed to from a short
    line in "For you", and removed once it has served.
  - Contents of more than 3 lines show their structure: lists, bold head
    words ("- **head**: body"), nesting for what you compare. One list line
    says one thing.
  - Whatever needs the person's decision or check goes near the top, as an
    Ask (with Options that say what each choice means, e.g. "install it" /
    "look at it first") or an Item with wait="you". Never only in the chat:
    decisions, answers and requests to the person go in the report too.
  - A Notice is for what the person must know first: the tools or the
    environment need something from them (reopen the viewer, set something
    up), or your work is stuck. Its title says what they are to do; steps
    go in its contents. Give it at="..." from date '+%Y-%m-%d %H:%M' (a new
    time when you rewrite it to tell something anew). Remove it once done.
  - Write a status only to say something else than 作業中 (a doing Item
    shows it itself), never "done". Drop a done line from "In progress"
    (git keeps the history).
  - Questions: when the person asks something in a thought, answer at the
    bottom of "For you" with <Item id="a-..." look="y">, titled "> " and
    their question (shortened at a clause boundary, never mid-sentence).
    Its first line of contents is your conclusion in one sentence; details
    follow. Settle the thought with "answered (in For you)". When an answer
    says work is queued or in progress, link to that row ([Add the CLI](#cli))
    instead of restating it.
  - What arrives from the board is answered in the report; in the chat say
    only a line or two ("Answered in the report; now doing X"). What the
    person writes in the chat is answered in the chat.
  - Keep ids stable while a line means the same thing (answers are tied to
    them); never reuse one for a different line. Rewrite an Ask once you
    have acted on its answer (the answer stops showing), and when it is
    settled drop it, keeping what was decided as one line in "Decided".
  - When "hide" arrives for a line, drop it unless you still use it
    yourself; always drop an answer the person hid. A row removed while the
    person still had business with it is warned of once ("warning: row
    ... was removed but ..."): put it back if removing it was wrong.
  - Edit report.md with the Edit tool, its old_string unique: include the
    row's id (<Item id="x"), never a bare close tag like </Item> or </Fold>,
    which matches every row. Run 'pitboard check' after every rewrite.
  - A report that fails check is not shown: the person keeps seeing the last
    one that passed, with a line saying you are fixing it, and it is your
    turn until you fix it. Every other command (and the hooks) ends with
    "report.md has N problem(s). Run: pitboard -s ID check" meanwhile.

Reading 'changes':
  It prints only what the person sent, and nothing twice:
    answer hook = a (install it)
    thought 7 on hook (Install the hook?): only post-tool
    thought 8: maybe drop the log
      continues here (the following lines of a thought are indented)
    thought 9: like this one? [Image #3]
      image 3: /abs/path/.pitboard/ID/images/3.png
    hide (2): Drop the log / Implement the vie…
  answer: the option chosen, with its text. thought N: "on ID" when written
  on that row (with its title then). An image's path follows its thought:
  open it (Read). hide: the person no longer needs to see that line. The
  text in parentheses is the title the person saw: "→ now: ..." after it
  means you rewrote the line since; think about which one they meant.
  Lines starting with retract, edit or unretract, and every detail:
  pitboard help changes ('changes' says so when one comes).

Thoughts:
  (The board calls them comments; commands and their output say thought.)
  Settle every thought you received, never leave one:
    pitboard -s ID settle N TEXT        (e.g. "made it a todo", "answered",
                                         "noted", "not doing it")
  A state goes before TEXT: --done (the default), --doing (working on it)
  or --hold (put off, say why); the latest settle counts. Several in one
  call only with a state before each number:
    pitboard -s ID settle --doing 50 "looking into it" --done 51 "answered"
  The board shows the person "AI の番" (your turn) from their send until
  you rewrite the report or settle (--done or --hold) every thought it
  carried, and while a thought is --doing; so settle --doing at once when
  work takes long, and --done with the result when it is finished (rewrite
  its answer line too). It is also "AI の番" whenever report.md fails
  'pitboard check' or is missing: fix it.
  What 'changes' and 'wait' print ends with the numbers still --doing or
  --hold (open: 186 201 (oldest 2h)); 'pitboard -s ID open' lists them.

Send:
  The person answers at their own pace, then sends it all at once (s on the
  board). How it reaches you:
  1. With the Claude Code hooks (pitboard help hooks), by itself. You cannot
     tell beforehand whether they are installed, so:
  2. Before every stop, run 'pitboard -s ID wait' in the background (in
     Claude Code: Bash with run_in_background). It ends with what was sent,
     and you are woken. Stop doing so once its output says the Stop hook
     waits instead.
  3. When a line "pitboard: N sent. Run: pitboard -s ID changes" arrives in
     the chat, run that command and act on what it prints.
  Details, and how the viewer types that line (PITBOARD_SEND, herdr, tmux):
  pitboard help send.
`

const formatText = `pitboard help format: the report format in full

Text (paragraphs, an Item's contents, text in a Section or Fold):
    lines between blank lines     one paragraph (lines are joined); a blank
                                  line between paragraphs stays
    - text                        a list line (·)
    - text, indented 2 per level  nested list, 3 levels at most (· - ·)
    1. text                       numbered (the number is shown as written);
                                  may nest too
    a line indented under a list  continues that list line
    line (deeper than its mark)
    **word**: text                "word" in bold as a head word; such a line
                                  starts a paragraph of its own
    **...** alone on a line       a small heading (a blank line above it)
    two spaces or \ at the end    a line break (the next line is not joined)
    [text](#id)                   a link to the row with that id (an Item,
                                  Ask, Link, Notice or Fold of this report;
                                  check refuses one no row has). In a title
                                  or contents. The person follows it with g
                                  and its digit (1-9, the line's links in
                                  order), and comes back with g -
  Indentation counts from the first text line of the part (of an Item: its
  first line of contents). In a Section or Fold, only a first-level "- "
  line is a stop the person can hide (with its nested lines); nested and
  numbered lines, and lists in an Item, are read only. Inside Code nothing
  of this applies.

Parts (anything else is an error):
  Notice    id (required), at="YYYY-MM-DD HH:MM" (local time it was put
            up, zero-padded; shown at its right end). Like an Item: the
            first line is the title, the lines after it (text and Code) what
            opening it shows. At the top level only, 3 at most. The board
            shows it above the page, in yellow, however far the person
            scrolls; the title is never cut.
  Title     the page title, one line
  Section   title="..." (required). Holds Item Ask Link Code Fold and text.
            Not inside another Section or a Fold.
  Item      id (required), state = todo (default) | doing | done,
            status (a short word shown at the right end),
            note (why the status; shown when opened),
            wait="you" (it cannot move on until the person does
            something; not on a done Item),
            look="y" (you want the person to read it: see Marks).
            The status is colored by attributes, not by its words:
            yellow with wait="you" (needs the person), green when
            state="doing" (moving), faint otherwise (queued, on hold).
            So mark with wait="you" every Item that needs the person's hand.
            First line: the item. Following lines: what opening it shows
            (text and Code only).
            The head says what can be done with it: ▸ / ▾ when it opens (it
            has contents or a note), a faint · when it does not, ✓ when done.
  Ask       id (required). First line: the question. Then 2 or more Option.
            The board shows an Ask closed, as its first line only, until
            the person opens it. (look="y" is allowed but adds nothing: an
            Ask is marked anyway.) ask="y" on an Item was removed: write an
            Ask.
  Option    key (required, one letter a-z, unique in its Ask; not one of
            the viewer's own keys {viewerKeys}). One line. Keys y and n are
            fine when they read naturally.
  Link      id (required), and exactly one of path (a file, shown as
            written) or url (http:// or https://). One line: its title.
            The board shows the path or URL under the title; it does not
            open files. Put what must be read in the board in Code.
            look="y" as on Item.
  Fold      title (required), id (optional; keeps it open when folds above
            it are added or removed), look="y" (as on Item; read means
            everything it holds). Holds what a Section holds. Starts
            closed.
  Code      lang (optional; "diff" is colored). Raw until </Code>; the
            indentation of the <Code> tag is removed from its lines.
  id: 1-16 of a-z 0-9 -, unique in the page. The person acts on Item, Ask,
  Link and Notice through their id: an Option key on an Ask (the one way to
  answer), and thoughts written on any of them; they may also hide a line
  or say it is read. They may write thoughts on a Fold too: through its id,
  or by its title if it has none.

Marks: the board puts a yellow ● beside each Ask not answered yet (on
  what it says now: no option chosen and no comment written on it) and a
  green ● (unread) beside each line with look="y" that they have not read
  since it last changed. Only the person clears a mark: by answering or
  writing on the line, or (green) with r (read) on it; the cursor does
  not. space takes them to the nearest marked line below the cursor
  (yellow, green, or an Item with wait="you"; all alike), from the top
  when there is none below. Lines without look="y" get no green mark,
  whatever changes. Rewriting a look="y" line (title, state, status, note,
  contents) marks it unread again.

Hidden lines: the viewer hides a line by what it says: when you change its
  title, state or status, it shows again as new. Changing only its
  contents, note, path or options does not bring it back; neither does
  adding to a hidden Fold. To have the person look at a changed line again,
  change its title or state.
`

const changesText = `pitboard help changes: every line 'changes' prints

'changes' (and 'wait' and the hooks) prints lines like:
    answer unit = a (per session)
    answer hook = b (look at it first; question: Install the hook? → now: Install the hook now?)
    thought 7 on hook (Decide whether to install the hook): only post-tool
    thought 8: maybe drop the log
    thought 9: a thought written on two lines
      continues here (the following lines of a thought are indented)
    thought 10: like this one? [Image #3]
      image 3: /abs/path/.pitboard/SESSION/images/3.png
    hide (2): Drop the log / Implement the vie…
    unhide (1): Drop the log
    retract thought 7: only post-tool
    edit thought 8: maybe drop the log, and the history too
    unretract thought 7: only post-tool
    retract answer unit = a (per session)
    retract hide: Drop the log

answer is the option chosen, with its text. The text in parentheses (or
after "question:") is the line's title when the person pressed the key (or
began writing). "→ now: ..." means you have rewritten the line since: the
person judged the old title, so think about which one the answer is for
before applying it. "→ now gone" means the line is gone.
An Ask the person answered more than once (one option, then another) is
told once, with the last.

thought N is a thought; "on ID" when written on that line (the titles are
cut to 30 characters there), 'on fold "TITLE"' when written on a Fold
without an id. A thought may hold images, "[Image #N]" in its text: the
line under it gives the image's path; open it (Read) to see it.

hide / unhide come last, all on one line (titles cut to 20 characters).
hide means the person decided they no longer need to see that line: drop it
from report.md unless you still need it (always drop an answer). unhide
means they want to see it again.

edit thought N means the person rewrote a thought you already received: the
text is the whole new thought. Act on the new text; if you had settled it,
settle it again (the board shows it as 既読 again, with no state from you).
(A thought rewritten before it reached you just arrives as "thought N" with
the new text.)

retract thought N means the person took that thought back. If you were
acting on it, stop. It needs no settle. "unretract thought N" means a
retract you were told was undone: the thought stands again.

retract followed by an answer, hide or unhide line means the person took
back that one, which you were told before (u on the board): act as if it
had not been given (an answer given before it, if any, stands again).
Taking back a rewrite is an "edit thought N" with the text before it.
Whatever the person takes back before you were told it is never told.

The board shows, at the right end of a line, the state of the person's
latest input on it: 未送信 (not sent), 送信済 (sent), 既読 (you read it),
then for a thought 対応中 / 保留 / 対応済 (doing / hold / done).
`

const sendText = `pitboard help send: how a send reaches you

Nothing the person does on the board reaches you until they send it: s on
the board marks everything so far as sent (a "send" in events.jsonl).
'changes' (and the hooks) deliver only what comes before the latest send;
what the person does after it waits for the next send. How you are woken,
in order:

1. The hooks (pitboard help hooks), when installed: nothing for you to run.
2. 'pitboard -s SESSION wait [--timeout DURATION]', when they are not.
   Run it in the background before your first stop, and again before each
   stop. If a send has not been delivered yet, it prints what 'changes'
   prints and ends at once; otherwise it waits for the next send, then
   prints and ends (exit 0). With --timeout (e.g. 30m; default: none) or on
   SIGINT / SIGTERM it ends quietly. Once the Stop hook has run in this
   session, 'wait' no longer waits, and its output ends with:

     {stopHookNote}

3. Typing into your terminal, when nothing is waiting. The viewer types:

     pitboard: N sent. Run: pitboard -s SESS changes

   (N things were sent; SESS is the shortest head of your session id, 8
   characters or more, that is unique on this board.) When a line starting
   with "pitboard: N sent." arrives, run the command it gives and act on
   what it prints.

While the Stop hook or 'wait' waits, .pitboard/SESSION/waiting holds its
pid, and the board shows "AI 待機中" at the top right: sending then only
records the send. A second waiter on the same session ends at once (exit 0;
'wait' prints "already waiting (pid N)"), leaving it to the first. If
waking you fails, the send is still recorded, and the person is asked to
tell you in the chat.

To type the line, the viewer runs $PITBOARD_SEND if set (with sh -c, given
$PITBOARD_SESSION and $PITBOARD_MESSAGE), or else, when herdr is on PATH,
'herdr agent prompt' to the agent whose session is SESSION (found with
'herdr agent list'; the viewer itself need not run inside herdr). Each try
is cut off after 5 seconds. An example for tmux (keep "$PITBOARD_MESSAGE"
in double quotes, or it is split into words):

  export PITBOARD_SEND='tmux send-keys -t work:0.1 "$PITBOARD_MESSAGE" Enter'
`

const hooksText = `pitboard help hooks: the Claude Code hooks

With the hooks, what the person sends reaches you by itself, with no
command of yours: while you work (after your next tool call), and once you
have stopped (the Stop hook waits for the send in the background and wakes
you with it). In ~/.claude/settings.json:

  "hooks": {
    "PostToolUse": [{ "hooks": [{ "type": "command", "command": "pitboard hook post-tool" }] }],
    "Stop":        [{ "hooks": [{ "type": "command", "command": "pitboard hook stop", "asyncRewake": true, "timeout": 86400 }] }]
  }

- The hooks do nothing in a session without a board (no output, no files),
  and stay silent when the board cannot be read or written.
- Claude Code ends the Stop hook after "timeout" seconds (the default is
  far too short: give it). After that, any chat message or your next stop
  starts the waiting again.
- A send of only hide / unhide does not wake you through the Stop hook
  (they go with the next delivery).
`

const browserText = `pitboard help browser: the board in a browser

'pitboard -s ID serve [--port N] [--no-open] [--foreground]' shows the same
board in the person's browser: the same parts, words, marks and rules as the
viewer, and the two may run at once (each sees what the other does). It
does not change what you do: write report.md, read 'changes'. If the person
would rather use a browser than a terminal pane, tell them to run:

  pitboard -s ID serve

There is one server per user, in the background: serve starts it if it is
not running, adds the board to it, prints the board's URL
(http://127.0.0.1:7482/s/<short id>/?t=…), opens it (--no-open: only prints
it) and returns at once. Run for another session or project, it adds that
board to the same server; nothing restarts. The index (/) lists every board
added. It listens on 127.0.0.1 only, and the URL holds a key, kept across
restarts (an open tab comes back by itself, and serve then opens no second
one); 'pitboard serve --rotate' changes it.
Its port is 7482 ("PITB" on a phone keypad), or the next free one up to
7491, else any free one; --port N is exact.
The server stops by itself after 30 minutes with no page open; 'pitboard
serve --stop' stops it now, 'pitboard serve --status' shows it and its
boards (--url: with their URLs), 'pitboard -s ID serve --forget' takes a
board off it, --foreground keeps it in the terminal (ctrl+c stops it). Its
state and log: $XDG_STATE_HOME/pitboard/ (or ~/.local/state/pitboard/):
token, serve.json, serve.log, boards.json (the boards added, shown again
when it starts again, unless their directory is gone). 'pitboard serve -h'
lists its flags.
`
