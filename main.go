package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// skill is the Claude Code skill that points an AI at pitboard -h
// (pitboard --skill > ~/.claude/skills/pitboard/SKILL.md). It says nothing
// itself, so it does not go out of date when pitboard changes.
//
//go:embed skills/pitboard/SKILL.md
var skill string

// hookStdin is where `pitboard hook` reads the hook's JSON (tests replace it).
var hookStdin io.Reader = os.Stdin

// usage is pitboard -h. The keys an Option cannot use come from viewerKeys,
// so the two never disagree.
var usage = strings.ReplaceAll(usageText, "{viewerKeys}", viewerKeyList())

const usageText = `pitboard - a board beside an AI session: the AI reports, the person reacts

Usage:
  pitboard [-s SESSION] [view]     the viewer (without -s: the most recently updated session)
                                   (-s may also come after the command)
  pitboard -s SESSION path         print the path of the session's report.md (write the report there)
  pitboard -s SESSION changes      print what the person has sent since the last call, one per line
  pitboard -s SESSION settle N [--doing|--hold|--done] TEXT
                                   set thought N's state and what you did with it
                                   (several: --STATE N TEXT --STATE N TEXT ...)
  pitboard -s SESSION check        check report.md; prints problems with line numbers, exit 1 if any
  pitboard -s SESSION wait [--timeout DURATION]
                                   wait until the person sends, then print what
                                   'changes' prints (run it in the background
                                   before you stop; see Send)
  pitboard -h                      this help
  pitboard hook prompt-submit|post-tool|stop   for Claude Code hooks: delivers 'changes' (see Hooks)
  pitboard --skill                 print the Claude Code skill (SKILL.md); it points here

The board is a place for what must not scroll away in the chat: where things
stand, what the person must decide, what they should look at. You (the AI)
write the report; the person reacts on the board (y / n, answers to
questions) and writes thoughts. The person never edits the report, and you
never edit their record.

Starting:
  1. Your session id is your Claude Code session id: the last directory but
     one of your scratchpad path (.../<project>/<session-id>/scratchpad).
  2. 'pitboard -s ID path' prints where the report goes (the first call
     makes the board). Write that file with Edit/Write.
  3. Ask the person to open the viewer beside the chat: pitboard -s ID

SESSION is -s or $PITBOARD_SESSION ([A-Za-z0-9_-]+; an existing session may be
given by a unique prefix). Use your Claude Code session id. The board lives in
.pitboard/SESSION/, found from the current directory upwards (not above your
home directory), so commands and hooks work from subdirectories too. 'path'
makes a new board in the nearest .pitboard/ above, or here if there is none.

Who writes what:
  report.md     you (the AI) only, with your usual file edits (Edit/Write)
  events.jsonl  the person only, through the viewer. Never edit it; use
                'pitboard settle' to answer thoughts.
The person's reactions never change report.md. You read them with 'changes'
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

    <Notice id="reopen">Reopen pitboard: I installed a new version</Notice>
    <Title>Build the minimal pitboard</Title>

    One paragraph saying where things stand.

    <Section title="For you">
      <Ask id="unit">One board per what?
        <Option key="a">per session</Option>
        <Option key="b">per project</Option>
      </Ask>
      <Item id="hook" ask="y">Decide whether to install the hook
        Installing it touches ~/.claude/settings.json.
      </Item>
    </Section>

    <Section title="Answers">
      <Item id="why-cwd" look="y">Why keep the data in the working directory?
        So that each project has its own board.
      </Item>
    </Section>

    <Section title="In progress">
      <Item id="impl" state="doing">Implement the viewer</Item>
      <Item id="cli" status="queued">Add the CLI</Item>
      <Item id="parts" status="waiting on you" wait="you" note="needs the answer above">Fix the part list</Item>
    </Section>

    <Section title="To look at">
      <Link id="spec" path="pitboard/SPEC.md">The spec</Link>
      <Link id="pr" url="https://github.com/OWNER/REPO/pull/1">The pull request</Link>
      <Code lang="diff">
      -old
      +new
      </Code>
    </Section>

    <Fold title="Done (1)">
      <Item id="tui" state="done">One-column viewer</Item>
    </Fold>

  Parts (anything else is an error):
    Notice    id (required). One paragraph of text. At the top level only,
              3 at most. The board shows it above the page, in yellow,
              however far the person scrolls (3 rows at most each). See
              Manners for when to use it.
    Title     the page title, one line
    Section   title="..." (required). Holds Item Ask Link Code Fold and text.
              Not inside another Section or a Fold.
    Item      id (required), state = todo (default) | doing | done,
              status (a short word shown at the right end; see Manners),
              note (why the status; shown when opened),
              ask="y" (you want a y / n on it; not on a done Item),
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
              ask="y" and look="y" as on Item.
    Fold      title (required), id (optional; keeps it open when folds above
              it are added or removed), look="y" (as on Item; read means
              everything it holds). Holds what a Section holds. Starts
              closed.
    Code      lang (optional; "diff" is colored). Raw until </Code>; the
              indentation of the <Code> tag is removed from its lines.
  id: 1-16 of a-z 0-9 -, unique in the page. The person reacts to Item, Ask,
  Link and Notice through their id: y / n on Items, Links and Notices, an
  Option key on an Ask, and thoughts written on any of them.

  Marks: the board puts a yellow ● beside each line waiting for the person
  (an Ask not answered yet, an Item or Link with ask="y" without a y / n on
  what it says now) and a green ● (unread) beside each line with look="y"
  that they have not read since it last changed. Only the person clears
  it, with r (read) on the line or by reacting to it; the cursor does
  not. space takes them to the nearest marked line below the cursor
  (yellow, green, or an Item with wait="you"; all alike), from the top
  when there is none below. Lines without look="y" get no green mark,
  whatever changes.
  So: mark with ask="y" (or make an Ask) exactly what you need them to
  answer, and with look="y" exactly what you need them to read. Rewriting a
  look="y" line (title, state, status, note, contents) marks it unread
  again.

A layout to start from:
  The layout is yours; this is a starting point, not a rule. From the top:
    1. The title, and where things stand in a sentence or two.
       (What the person reads first when they come back.)
    2. "For you": what needs the person's decision, check or work: Ask, or
       Item with ask="y". (When it is empty, they know they need do nothing.)
    3. "Answers": answers to what they asked, each with look="y". The title
       is the question, the contents the answer; they hide it with x once
       read.
       (Answers do not scroll away with the chat.)
    4. "In progress": what you are doing now (state="doing"; the board
       says 作業中 itself) and next, with a status only for something else:
       waiting on you, queued, on hold. (They see what moves without
       asking.)
    5. "Decided": one decision per line; they hide what they no longer need.
       (Decisions made in the chat are not lost.)
    6. "Where": repos, files, URLs (Link). (One place to find things.)
    7. "Done": in a Fold, newest first. Move a line here as soon as it is
       done; do not let done lines pile up in "In progress". (Kept, but out
       of the way.)
  Manners for it: the more a section needs the person's hand, the higher it
  goes. Leave out a section that would be empty. When the report no longer
  fits on one screen, move what is done into the Fold, or drop it.

Manners:
  - The layout is yours (which sections, what goes in them), but the report
    is skimmed, not read. One item per line; details go in the item's
    following lines (shown when the person opens it).
  - A line you want a reaction or a thought on must be an Item, Ask or Link
    with an id.
  - A Notice is for what the person must know first: when tools or the
    environment need something from them (reopen the viewer, set
    something up), or when your work is stuck. Not for ordinary questions
    or answers. Remove it once it has done its job.
  - Say a title in one sentence; details go in the contents. (A long title
    wraps, it is not cut; still, do not cram who does it or remarks in
    parentheses into it.)
  - The head of an Item says only what can be done with it: ▸ / ▾ when it
    opens (it has contents or a note), a faint · when it does not, ✓ when
    it is done. How far it is goes at the right end, in words: a
    state="doing" Item without a status shows 作業中 (working) there by
    itself, so write a status only to say something else (waiting on you,
    queued, on hold), and never "done" (済み) on a state="done" one.
  - When a line is done, move it into the "Done" Fold; do not leave done
    lines in the sections in progress.
  - When contents run past 3 lines, do not pour them into a paragraph: show
    their structure with lists and bold head words. Nest what you compare.
    One list line says one thing.
  - Keep ids stable while a line means the same thing: reactions are tied to
    ids. A reaction stops showing when you rewrite that line (text, state,
    status or note), so rewrite a line once you have acted on its reaction.
  - Never reuse an id for a different line. When the thing changes, give it
    a new id. (Reactions and thoughts record the title the person saw;
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
    report, with ask="y" (or an Ask) or wait="you". Never only in the chat.
  - Questions and answers: when the person asks you something on the board
    (a thought), answer in the report, so that the board alone tells them
    where things stand when they come back. Keep a section like "Answers"
    with one Item per question: its title is the question (short, close to
    their words), its contents the answer, with look="y". The person hides
    it with x once read; when "hide:" for it arrives, drop that Item the
    next time you rewrite the report. You may
    also drop an answer the person replied to, once you judge it settled.
    When a thought was a question, settle it with "answered (in Answers)".
  - Mark with wait="you" every Item that needs the person's hand (its
    status words alone get no color).
  - Put look="y" only on lines you want the person to read: an answer you
    wrote, a result they should check. Not on lines that only tell how
    things move (progress, queued work): they need not read those.
  - Nothing that matters lives only in the chat: decisions, answers and
    requests to the person go in the report too.
  - The person answers on the board at their own pace (y / n, answers,
    thoughts, hides) and then sends them all at once (Send, below). You
    need not call 'changes' often yourself: it prints only what was sent,
    and nothing twice.
  - Before you end your turn, run 'pitboard -s SESSION wait' in the
    background (in Claude Code: Bash with run_in_background). When the
    person sends from the board, it prints what was sent and ends, and you
    are woken. Act on it, and run 'wait' again before you stop. You may
    start it before you begin working, too: what the person sends midway
    reaches you then. Once it has ended, start it again.

'changes' prints lines like:
    y hook (Decide whether to install the hook)
    n impl (Implement the viewer → now: Implement the viewer and the CLI)
    y old (Drop the log → now gone)
    answer unit = a (per session)
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
    retract y hook (Decide whether to install the hook)
    retract hide: Drop the log
  y means "good / done", n "not needed / no". answer is the option chosen.
  The text in parentheses is the line's title when the person pressed the
  key (or began writing). "→ now: ..." means you have rewritten the line
  since: the person judged the old title, so think about which one the
  reaction is for before applying it. "→ now gone" means the line is gone.
  thought N is a thought; "on ID" when written on that line (the titles are
  cut to 30 characters there). A thought may hold images, "[Image #N]" in
  its text: the line under it gives the image's path; open it (Read) to see
  it.
  A line the person reacted to more than once (y, then n; one option, then
  another) is told once, with the last.
  hide / unhide come last, all on one line (titles cut to 20 characters).
  hide means the person decided they no longer need to see that line. Nothing
  needs doing; you may drop the line the next time you rewrite the report.
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
  retract followed by a reaction, answer, hide or unhide line means the
  person took back that one, which you were told before (u on the board):
  act as if it had not been given (a reaction given before it, if any,
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
  Settle several in one call only with a state before each number:
    pitboard -s ID settle --doing 50 "looking into it" --done 51 "answered"
  Without a state first, it is one thought and every word after N is its
  text ('settle 9 fixed 5 bugs' settles 9 with "fixed 5 bugs"). Two states
  for one thought are an error.
  The board shows, at the right end of a line, the state of the person's
  latest input on it: 未送信 (not sent), 送信済 (sent), 既読 (you read it),
  then for a thought 対応中 / 保留 / 対応済 (doing / hold / done).

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

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		var ex exitError
		if errors.As(err, &ex) {
			os.Exit(int(ex))
		}
		fmt.Fprintln(os.Stderr, "pitboard:", err)
		os.Exit(1)
	}
}

// settleArgs reads 'settle N [--doing|--hold|--done] TEXT...' (one
// thought: everything after N and its state is the text), or several, each
// with its state first: 'settle --STATE N TEXT... --STATE N TEXT...' (a
// group starts at a state followed by a number). Nothing is guessed: a
// group with two states is an error.
func settleArgs(args []string) ([]settlement, error) {
	flag := func(a string) (string, bool) {
		switch a {
		case "--doing", "-doing":
			return "doing", true
		case "--hold", "-hold":
			return "hold", true
		case "--done", "-done":
			return "done", true
		}
		return "", false
	}
	isNum := func(a string) bool { _, err := strconv.Atoi(a); return err == nil }
	usage := errors.New("usage: pitboard settle N [--doing|--hold|--done] TEXT, or settle --STATE N TEXT --STATE N TEXT ...")
	if len(args) == 0 {
		return nil, usage
	}
	// Split into groups: a group starts at a state followed by a number
	// (or, for the first, at a number).
	// Several only when the first group starts with its state.
	_, several := flag(args[0])
	var groups [][]string
	for i := 0; i < len(args); i++ {
		_, isFlag := flag(args[i])
		if i == 0 || (several && isFlag && i+1 < len(args) && isNum(args[i+1])) {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], args[i])
	}
	var out []settlement
	for _, g := range groups {
		state, rest := "", g
		if f, is := flag(rest[0]); is {
			state, rest = f, rest[1:]
		}
		if len(rest) == 0 {
			return nil, usage
		}
		n, err := strconv.Atoi(rest[0])
		if err != nil {
			return nil, fmt.Errorf("settle: %q is not a thought number", rest[0])
		}
		rest = rest[1:]
		if len(rest) > 0 {
			if f, is := flag(rest[0]); is {
				if state != "" {
					return nil, fmt.Errorf("settle %d: two states (--%s and --%s); give one", n, state, f)
				}
				state, rest = f, rest[1:]
			}
		}
		for _, w := range rest {
			if f, is := flag(w); is {
				if state != "" {
					return nil, fmt.Errorf("settle %d: two states (--%s and --%s); give one. To settle several, put a state before each number: settle --done 9 TEXT --done 5 TEXT", n, state, f)
				}
				return nil, fmt.Errorf("settle %d: %s inside the text. To settle several, put a state before each number: settle --done 9 TEXT --done 5 TEXT", n, w)
			}
		}
		out = append(out, settlement{n, state, strings.Join(rest, " ")})
	}
	return out, nil
}

// exitError ends the program with a code, its message already printed.
type exitError int

func (e exitError) Error() string { return "exit " + strconv.Itoa(int(e)) }

func run(args []string, out io.Writer) error {
	// Flags go before the command, and -s may also come right after it
	// (pitboard path -s abc). Anything later belongs to the command, so a
	// "-s" inside settle's TEXT stays text.
	session := os.Getenv("PITBOARD_SESSION")
	sflag := func(args []string) (string, int, bool) {
		switch {
		case len(args) >= 2 && args[0] == "-s":
			return args[1], 2, true
		case len(args) >= 1 && strings.HasPrefix(args[0], "-s="):
			return strings.TrimPrefix(args[0], "-s="), 1, true
		}
		return "", 0, false
	}
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch a := args[0]; {
		case a == "-h" || a == "--help" || a == "-help":
			fmt.Fprint(out, usage)
			return nil
		case a == "--skill" || a == "-skill":
			fmt.Fprint(out, skill)
			return nil
		case a == "-s" && len(args) < 2:
			return errors.New("-s needs a session")
		default:
			s, n, ok := sflag(args)
			if !ok {
				return fmt.Errorf("unknown flag %s (see pitboard -h)", a)
			}
			session, args = s, args[n:]
		}
	}
	if len(args) > 0 && args[0] == "hook" {
		// The session and directory come from the hook's input, never from
		// -s, $PITBOARD_SESSION or the working directory.
		if len(args) != 2 {
			return errors.New("usage: pitboard hook prompt-submit|post-tool|stop (reads the hook's JSON on stdin)")
		}
		return hook(args[1], hookStdin, out)
	}
	if len(args) > 0 {
		if s, n, ok := sflag(args[1:]); ok {
			session = s
			args = append([]string{args[0]}, args[1+n:]...)
		} else if len(args) == 2 && args[1] == "-s" {
			return errors.New("-s needs a session")
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// The board is looked for from here upwards; a new one goes where the
	// boards are (the nearest .pitboard/ above), or here.
	base := boardRoot(cwd)
	cmd := "view"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if cmd == "help" {
		fmt.Fprint(out, usage)
		return nil
	}

	var st *Store
	if session != "" {
		b, name, ok, err := findSession(cwd, session, true)
		if err != nil {
			return err
		}
		if !ok {
			b, name = base, session
		}
		st = newStore(b, name)
	}
	if cmd == "view" {
		if st == nil {
			if ss := sessions(base); len(ss) > 0 {
				st = newStore(base, ss[0])
			}
		}
		return runViewer(base, st)
	}
	if st == nil {
		return errors.New("no session: give -s SESSION or set $PITBOARD_SESSION")
	}
	if _, err := os.Stat(st.dir); err != nil && cmd != "path" {
		return fmt.Errorf("no session %q yet (pitboard -s %s path creates it)", st.session, st.session)
	}

	switch cmd {
	case "path":
		if err := os.MkdirAll(st.dir, 0o755); err != nil {
			return err
		}
		fmt.Fprintln(out, st.reportPath())
	case "changes":
		lines, err := st.Changes()
		if err != nil {
			return err
		}
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		st.noteProblems(out)
	case "settle":
		if len(args) < 1 {
			return errors.New("usage: pitboard settle N [--doing|--hold|--done] TEXT, or settle --STATE N TEXT --STATE N TEXT ...")
		}
		ss, err := settleArgs(args)
		if err != nil {
			return err
		}
		err = st.SettleAll(ss)
		st.noteProblems(out)
		return err
	case "wait":
		return runWait(st, args, out)
	case "check":
		b, err := os.ReadFile(st.reportPath())
		if err != nil {
			return err
		}
		_, probs, perr := parseReport(string(b))
		if perr != nil {
			return perr
		}
		for _, p := range probs {
			fmt.Fprintf(out, "%s:%d: %s\n", st.reportPath(), p.Line, p.Msg)
		}
		if len(probs) > 0 {
			return exitError(1)
		}
	default:
		return fmt.Errorf("unknown command %q (see pitboard -h)", cmd)
	}
	return nil
}
