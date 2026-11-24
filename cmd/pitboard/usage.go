package main

import (
	"strings"

	"github.com/qawatake/pitboard/internal/report"
)

// usage is pitboard -h. The keys an Option cannot use come from report.ViewerKeys,
// so the two never disagree.
var usage = strings.ReplaceAll(usageText, "{viewerKeys}", report.ViewerKeyList())

const usageText = `pitboard - a board beside an AI session: the AI reports, the person answers

Usage:
  pitboard [-s SESSION] [view]     the viewer (without -s: the most recently updated session)
                                   (-s may also come after the command)
  pitboard [-s SESSION] serve [--port N] [--no-open] [--foreground]
                                   the same board in a browser: one server for all
                                   boards, in the background (see Browser);
                                   serve --stop, serve --status [--url], serve --forget
  pitboard -s SESSION path         print the path of the session's report.md (write the report there)
  pitboard -s SESSION changes      print what the person has sent since the last call, one per line
  pitboard -s SESSION settle N [--doing|--hold|--done] TEXT
                                   set thought N's state and what you did with it
                                   (several: --STATE N TEXT --STATE N TEXT ...)
  pitboard -s SESSION open         list the thoughts still --doing or --hold (nothing if none)
  pitboard -s SESSION check        check report.md; prints problems with line numbers, exit 1 if any,
                                   and warnings about removed rows (they do not change the exit code)
  pitboard -s SESSION wait [--timeout DURATION]
                                   wait until the person sends, then print what
                                   'changes' prints (run it in the background
                                   before you stop; see Send)
  pitboard -h                      this help
  pitboard hook prompt-submit|post-tool|stop   for Claude Code hooks: delivers 'changes' (see Hooks)
  pitboard --skill                 print the Claude Code skill (SKILL.md); it points here

The board is a place for what must not scroll away in the chat: where things
stand, what the person must decide, what they should look at. You (the AI)
write the report; the person answers on the board (an option of an Ask)
and writes thoughts. The person never edits the report, and you
never edit their record.

Starting:
  1. Your session id is your Claude Code session id: the last directory but
     one of your scratchpad path (.../<project>/<session-id>/scratchpad).
     In commands, write only its first 8 characters (ID below), e.g.
     -s 0a1b2c3d for 0a1b2c3d-4e5f-...: the form pitboard itself prints.
  2. 'pitboard -s ID path' prints where the report goes (the first call
     makes the board). Write that file with Edit/Write.
  3. Ask the person to open the viewer beside the chat: pitboard -s ID

SESSION is -s or $PITBOARD_SESSION ([A-Za-z0-9_-]+; an existing session may be
given by a unique prefix). Use the first 8 characters of your Claude Code
session id. The board lives in
.pitboard/SESSION/, found from the current directory upwards (not above your
home directory), so commands and hooks work from subdirectories too. 'path'
makes a new board in the nearest .pitboard/ above, or here if there is none.

Who writes what:
  report.md     you (the AI) only, with your usual file edits (Edit/Write)
  events.jsonl  the person only, through the viewer. Never edit it; use
                'pitboard settle' to answer thoughts.
The person's answers and thoughts never change report.md. You read them with 'changes'
and rewrite the report yourself.

Writing report.md:
  A line starting with <Name ...> opens a part and </Name> closes it (tags go
  at the start of a line; indentation is fine). Every other line is text:
  paragraphs and "- " list lines, with inline Markdown (` + "`code`" + `,
  **bold**, [title](url)). Attributes are name="value"; write " as &quot;.

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
  Indentation counts from the first text line of the part (of an Item: its
  first line of contents). In a Section or Fold, only a first-level "- "
  line is a stop the person can hide (with its nested lines); nested and
  numbered lines, and lists in an Item, are read only. Inside Code nothing
  of this applies.

    <Notice id="reopen" at="2026-11-23 21:46">Reopen pitboard
      I installed a new version: quit with q and run pitboard again.
    </Notice>
    <Title>Build the minimal pitboard</Title>

    <Section title="For you">
      <Ask id="unit">One board per what?
        <Option key="a">per session</Option>
        <Option key="b">per project</Option>
      </Ask>
      <Ask id="hook">Install the hook? (it touches ~/.claude/settings.json)
        <Option key="a">install it</Option>
        <Option key="b">look at it first</Option>
      </Ask>

      <Item id="a-cwd" look="y">> Is there one board per project?
        Yes: each project gets its own board.
        The data lives in the working directory, so boards never mix.
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

    <Fold title="Decided (1)">
      <Item id="one-col" state="done">One column: the viewer sits beside the chat
        A narrow pane has no room for two.
      </Item>
    </Fold>

  Parts (anything else is an error):
    Notice    id (required), at="YYYY-MM-DD HH:MM" (local time it was put
              up, zero-padded; shown at its right end). Like an Item: the first line is
              the title, the lines after it (text and Code) what opening it
              shows. At the top level only, 3 at most. The board shows it
              above the page, in yellow, however far the person scrolls; the
              title is never cut. See Manners for when to use it.
    Title     the page title, one line
    Section   title="..." (required). Holds Item Ask Link Code Fold and text.
              Not inside another Section or a Fold.
    Item      id (required), state = todo (default) | doing | done,
              status (a short word shown at the right end; see Manners),
              note (why the status; shown when opened),
              wait="you" (it cannot move on until the person does
              something; not on a done Item),
              look="y" (you want the person to read it: see Marks).
              The status is colored by attributes, not by its words:
              yellow with wait="you" (needs the person), green when
              state="doing" (moving), faint otherwise (queued, on hold).
              First line: the item. Following lines: what opening it shows
              (text and Code only).
    Ask       id (required). First line: the question. Then 2 or more Option.
              The board shows an Ask closed, as its first line only, until
              the person opens it: write the question so that it makes
              sense on its own, without the options. (look="y" is allowed
              but adds nothing: an Ask is marked anyway.)
    Option    key (required, one letter a-z, unique in its Ask; not one of
              the viewer's own keys {viewerKeys}). One line.
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
  (ask="y" was removed: to get a yes or no, write an Ask.)

  Marks: the board puts a yellow ● beside each Ask not answered yet (on
  what it says now) and a green ● (unread) beside each line with look="y"
  that they have not read since it last changed. Only the person clears
  it, with r (read) on the line, by answering or by writing on it; the
  cursor does
  not. space takes them to the nearest marked line below the cursor
  (yellow, green, or an Item with wait="you"; all alike), from the top
  when there is none below. Lines without look="y" get no green mark,
  whatever changes.
  So: make an Ask of exactly what you need them to answer, and mark with
  look="y" exactly what you need them to read. Rewriting a
  look="y" line (title, state, status, note, contents) marks it unread
  again.

A layout to start from:
  The layout is yours; this is a starting point, not a rule. From the top:
    1. The title only: what this board is about (it rarely changes). No
       summary paragraph under it: it repeats "For you" and "In progress"
       and goes stale, while those stay current because work flows
       through them.
    2. Notices, only when there is one (see Manners).
    3. "For you": first what the person is to answer or do (Ask, Item with
       wait="you"); under them, your answers to what they asked,
       each <Item id="a-..." look="y"> titled "> " and their question,
       its contents starting with the conclusion in one sentence, details
       after it (they hide it with x once read; see Manners). (When the
       section is empty, they know they need do nothing.)
    4. "In progress": what you are doing now (state="doing"; the board
       says 作業中 itself) and next, with a status only for something else:
       waiting on you, queued, on hold. (They see what moves without
       asking.)
    5. "Decided": in a Fold, one decision per line, with its reason.
       (Decisions made in the chat are not lost.)
  Manners for it: the more a section needs the person's hand, the higher it
  goes. Leave out a section that would be empty. When a line is done, drop
  it from "In progress" (git keeps the history; do not pile done lines up).
  Put a link in the contents of the line that needs it, or as a Link right
  under that line; do not make a section of links. When a question is
  answered, drop the Ask and keep what was decided as one line in
  "Decided".

Manners:
  - Give -s the first 8 characters of your session id (e.g. -s 0a1b2c3d),
    the form pitboard prints in its own lines; never the whole id. (A unique
    prefix is enough, and the full id only costs words in every command.)
  - The layout is yours (which sections, what goes in them), but the report
    is skimmed, not read. One item per line; details go in the item's
    following lines (shown when the person opens it).
  - An Item's contents should fit one screen. When something needs a long
    explanation, add a Section for that topic (paragraphs, lists, Items)
    and point to it from a short line in "For you"; do not pile it into
    one Item's contents. Remove the Section once it has served.
  - A list line is one short clause. When it needs a sentence or two, give
    it a head word ("- **head**: body") or sub-items; keep an Item's
    contents to what the person must read, not everything you know.
  - A line you want an answer on must be an Ask; one you want a thought on
    must be an Item, Ask or Link with an id.
  - To get a yes or no, write an Ask whose Options say what each choice
    means, not just yes / no when you can say more (<Option key="a">install
    it</Option> <Option key="b">look at it first</Option>). Keys y and n are
    fine when they read naturally.
  - A Notice is for what the person must know first: when tools or the
    environment need something from them (reopen the viewer, set
    something up), or when your work is stuck. Not for ordinary questions
    or answers. Remove it once it has done its job. Its title is one
    sentence saying what the person is to do; steps and commands go in its
    contents. Always give it at="..." from date '+%Y-%m-%d %H:%M', and when
    you rewrite it to tell something anew, give the new time (a change of at
    alone does not show a hidden notice again).
  - Say a title in one sentence; details go in the contents. (A long title
    wraps, it is not cut; still, do not cram who does it or remarks in
    parentheses into it.)
  - The head of an Item says only what can be done with it: ▸ / ▾ when it
    opens (it has contents or a note), a faint · when it does not, ✓ when
    it is done. How far it is goes at the right end, in words: a
    state="doing" Item without a status shows 作業中 (working) there by
    itself, so write a status only to say something else (waiting on you,
    queued, on hold), and never "done" (済み) on a state="done" one.
  - When a line is done, drop it from the sections in progress (git keeps
    the history).
  - When contents run past 3 lines, do not pour them into a paragraph: show
    their structure with lists and bold head words. Nest what you compare.
    One list line says one thing.
  - Keep ids stable while a line means the same thing: answers are tied to
    ids. An answer stops showing when you rewrite that Ask (its question or
    options), so rewrite an Ask once you have acted on its answer.
  - Never reuse an id for a different line. When the thing changes, give it
    a new id. (Answers and thoughts record the title the person saw;
    'changes' shows it next to what the line says now.)
  - Tags start at the beginning of a line and part names start with a
    capital letter (<Item>, not <item>).
  - Run 'pitboard check' after every rewrite of report.md and fix what it
    reports. The other commands ('changes', 'settle', 'wait') and the hooks
    also end with "report.md has N problem(s). Run: pitboard -s SESS check"
    while the report has problems: when you see it, run that and fix them.
  - Where to reply: what arrives from the board (through 'wait', 'changes'
    or the hooks) is answered in the report: the answer to a question, what
    you changed and how, what happens next. In the chat, say only a line or
    two (e.g. "Answered in the report; now doing X"), and do not repeat what
    the report says. When the person writes to you in the chat directly,
    answer in the chat (and put the answer in the report too if it should
    be kept).
  - Whatever needs the person's decision or check goes near the top of the
    report, as an Ask or an Item with wait="you". Never only in the chat.
  - Questions and answers: when the person asks you something on the board
    (a thought), answer in the report, so that the board alone tells them
    where things stand when they come back. Put an answer Item at the
    bottom of "For you", with look="y". Its title is "> " and the
    person's question in their words; shorten or summarize it when it is
    long (one line is best). When shortening, never stop mid-sentence
    (a title ending in "...the PR to" reads as cut off): cut at a clause
    boundary so it reads complete, or summarize. The first line of the
    contents is your conclusion in one sentence; the details follow. One
    answer Item may cover several thoughts asking the same thing. The
    person hides
    it with x once read; when "hide" for it arrives, drop that Item from
    report.md (always; see the next manner). You may
    also drop an answer the person replied to, once you judge it settled.
    When a thought was a question, settle it with "answered (in For you)".
  - When 'changes' brings "hide" for a line, drop that line from report.md
    unless you still need it yourself: a hidden line left in the report
    only spends your context every time you read it. Always drop an answer
    the person hid. A line you still use (progress you keep updating, a
    line still in progress) may stay.
  - Remove a row once the person hid it or it is resolved; a row removed
    while they have not answered it, or with a sent comment you have not
    settled done, is warned of once ("warning: row ... was removed but
    ..."): put it back if removing it was wrong.
  - Mark with wait="you" every Item that needs the person's hand (its
    status words alone get no color).
  - Put look="y" only on lines you want the person to read: an answer you
    wrote, a result they should check. Not on lines that only tell how
    things move (progress, queued work): they need not read those.
  - Nothing that matters lives only in the chat: decisions, answers and
    requests to the person go in the report too.
  - The person answers on the board at their own pace (answers, thoughts,
    hides) and then sends them all at once (Send, below). You
    need not call 'changes' often yourself: it prints only what was sent,
    and nothing twice.
  - Before you end your turn, run 'pitboard -s SESSION wait' in the
    background (in Claude Code: Bash with run_in_background). When the
    person sends from the board, it prints what was sent and ends, and you
    are woken. Act on it, and run 'wait' again before you stop. You may
    start it before you begin working, too: what the person sends midway
    reaches you then. Once it has ended, start it again.

'changes' prints lines like:
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
    retract answer unit = a (per session)
    retract hide: Drop the log
  answer is the option chosen, with its text. The text in parentheses (or
  after "question:") is the line's title when the person pressed the
  key (or began writing). "→ now: ..." means you have rewritten the line
  since: the person judged the old title, so think about which one the
  answer is for before applying it. "→ now gone" means the line is gone.
  thought N is a thought; "on ID" when written on that line (the titles are
  cut to 30 characters there), 'on fold "TITLE"' when written on a Fold
  without an id. A thought may hold images, "[Image #N]" in
  its text: the line under it gives the image's path; open it (Read) to see
  it.
  An Ask the person answered more than once (one option, then another) is
  told once, with the last.
  hide / unhide come last, all on one line (titles cut to 20 characters).
  hide means the person decided they no longer need to see that line. Drop it
  from report.md unless you still need it (always drop an answer; see
  Manners).
  unhide means they want to see it again.
  edit thought N means the person rewrote a thought you already received:
  the text is the whole new thought. Act on the new text; if you had
  settled it, settle it again (the board shows it as 既読 again, with no
  state from you). (A thought rewritten before it reached you just arrives as
  "thought N" with the new text.)
  retract thought N means the person took that thought back. If you were
  acting on it, stop. It needs no settle. (A thought taken back before it
  reached you is never told; "unretract thought N" means a retract you were
  told was undone, and the thought stands again.)
  retract followed by an answer, hide or unhide line means the
  person took back that one, which you were told before (u on the board):
  act as if it had not been given (an answer given before it, if any,
  stands again). Taking back a thought is "retract thought N", taking back
  a rewrite an "edit thought N" with the text before it. What the person
  takes back before you were told it is never told.
  (The viewer hides a line by what it says: when you change its title, state
  or status, it shows again as new. Changing only its contents, note, path
  or options does not bring it back; neither does adding to a hidden Fold.
  If you want the person to look at a changed line again, change its title
  or state.)

Thoughts:
  (The board calls them comments; commands and their output say thought.)
  You need not handle every thought at once, but settle every thought you
  received with 'pitboard -s ID settle N TEXT' (e.g. "made it a todo",
  "answered", "noted", "not doing it"). The board shows it, so the person
  can write a thought and forget it. Never leave one unsettled.
  A state goes before TEXT: --done (the default), --doing (working on it)
  or --hold (put off); the latest settle counts. When a thought takes a
  while, tell it at once with 'settle N --doing' (or --hold, with why),
  and settle it --done with the result when finished.
  What you put --doing, settle --done when it is done, saying what
  happened; if you wrote an answer line for it, rewrite that too. What is
  left is shown by 'changes' every time: what 'changes' and 'wait' print
  ends with one line of the numbers still --doing or --hold (oldest first):

    open: 186 201 202 (oldest 2h). Details: pitboard -s ID open

  'pitboard -s ID open' lists them, one a line:
    doing 186 (12m): the comments pane sits too low…
  Settle several in one call only with a state before each number:
    pitboard -s ID settle --doing 50 "looking into it" --done 51 "answered"
  Without a state first, it is one thought and every word after N is its
  text ('settle 9 fixed 5 bugs' settles 9 with "fixed 5 bugs"). Two states
  for one thought are an error.
  The board shows, at the right end of a line, the state of the person's
  latest input on it: 未送信 (not sent), 送信済 (sent), 既読 (you read it),
  then for a thought 対応中 / 保留 / 対応済 (doing / hold / done).

Browser:
  'pitboard -s ID serve' shows the same board in the person's browser: the
  same parts, words, marks and rules as the viewer, and the two may run at
  once (each sees what the other does). There is one server per user, in
  the background: serve starts it if it is not running, adds the board to
  it, prints the board's URL (http://127.0.0.1:7482/s/<short id>/?t=…),
  opens it (--no-open: only prints it) and returns at once. Run for another
  session or project, it adds that board to the same server; nothing
  restarts. The index (/) lists every board added. It listens on 127.0.0.1
  only, and the URL holds a key, kept across restarts (an open tab comes
  back by itself, and serve then opens no second one); 'pitboard serve
  --rotate' changes it.
  Its port is 7482 ("PITB" on a phone keypad), or the next free one up to
  7491, else any free one; --port N is exact.
  The server stops by itself after 30 minutes with no page open;
  'pitboard serve --stop' stops it now, 'pitboard serve --status' shows it
  and its boards (--url: with their URLs), 'pitboard -s ID serve --forget'
  takes a board off it, --foreground keeps it in the terminal (ctrl+c stops
  it). Its state and log: $XDG_STATE_HOME/pitboard/ (or
  ~/.local/state/pitboard/): token, serve.json, serve.log, boards.json (the boards
  added, shown again when it starts again, unless their directory is gone).
  If the person would rather use a browser than a terminal pane, tell them
  to run:
    pitboard -s ID serve
  It does not change what you do: write report.md, read 'changes'.

Hooks:
  (A second way, when 'wait' cannot be used.) With hooks, what the person
  has sent reaches you while you work (after your next tool call) and just
  before you stop, without waiting for the line in the chat. In
  ~/.claude/settings.json:

    "hooks": {
      "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "pitboard hook prompt-submit" }] }],
      "PostToolUse":      [{ "hooks": [{ "type": "command", "command": "pitboard hook post-tool" }] }],
      "Stop":             [{ "hooks": [{ "type": "command", "command": "pitboard hook stop" }] }]
    }

  - The hooks do nothing in a session without a board (no output, no
    files), and stay silent when the board cannot be read or written.
  - The stop hook does not keep you from stopping when all there is to
    deliver is hide / unhide (they go with the next delivery).
  - You cannot tell whether the hooks are installed.

Send:
  Nothing the person does on the board reaches you until they send it: s on
  the board marks everything so far as sent (a "send" in events.jsonl).
  'changes' (and the hooks) deliver only what comes before the latest send;
  what the person does after it waits for the next send. How you are woken,
  in order:

  1. 'pitboard -s SESSION wait [--timeout DURATION]' (the way to use; it
     needs nothing from the terminal). Run it in the background before you
     stop. If a send has not been delivered yet, it prints what 'changes'
     prints and ends at once; otherwise it waits for the next send, then
     prints and ends (exit 0). With --timeout (e.g. 30m; default: none) or
     on SIGINT / SIGTERM it ends quietly, printing nothing. While it waits,
     .pitboard/SESSION/waiting holds its pid, and the board shows "AI 待機中"
     at the top right: sending then only records the send, and 'wait' wakes
     you. A second 'wait' on the same session prints "already waiting (pid
     N)" and ends (exit 0), leaving it to the first.
  2. The hooks (above), while you work and before you stop. (s does not
     wake you through them; they deliver while you work.)
  3. Typing into your terminal, when no 'wait' is waiting. The viewer types:

    pitboard: N sent. Run: pitboard -s SESS changes

  (N things were sent; SESS is the shortest head of your session id, 8
  characters or more, that is unique on this board.) When a line starting
  with "pitboard: N sent." arrives, run the command it gives and act on
  what it prints.
  'wait' is the cheapest: what was sent arrives as its output, with no line
  typed into the chat and no extra command to run. Its output is the same
  as 'changes'. If waking you fails, the send is still recorded, and the
  person is asked to tell you in the chat.
  To type it, the viewer runs $PITBOARD_SEND if set (with sh -c, given
  $PITBOARD_SESSION and $PITBOARD_MESSAGE), or else, when herdr is on PATH,
  'herdr agent prompt' to the agent whose session is SESSION (found with
  'herdr agent list'; the viewer itself need not run inside herdr). Each try
  is cut off after 5 seconds. An example for tmux (keep "$PITBOARD_MESSAGE"
  in double quotes, or it is split into words):

    export PITBOARD_SEND='tmux send-keys -t work:0.1 "$PITBOARD_MESSAGE" Enter'
`
