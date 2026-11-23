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

// skill is the Claude Code skill that teaches an AI to use pitboard
// (pitboard --skill > ~/.claude/skills/<name>/SKILL.md).
//
//go:embed skill.md
var skill string

// hookStdin is where `pitboard hook` reads the hook's JSON (tests replace it).
var hookStdin io.Reader = os.Stdin

const usage = `pitboard - a board beside an AI session: the AI reports, the person reacts

Usage:
  pitboard [-s SESSION] [view]     the viewer (without -s: the most recently updated session)
                                   (-s may also come after the command)
  pitboard -s SESSION path         print the path of the session's report.md (write the report there)
  pitboard -s SESSION changes      print what the person did since the last call, one per line
  pitboard -s SESSION settle N TEXT   say where thought N went
  pitboard -s SESSION check        check report.md; prints problems with line numbers, exit 1 if any
  pitboard -h                      this help
  pitboard hook prompt-submit|post-tool|stop   for Claude Code hooks: delivers 'changes' (see --skill)
  pitboard --skill                 print a Claude Code skill (SKILL.md) on using pitboard

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

    <Title>Build the minimal pitboard</Title>

    One paragraph saying where things stand.

    <Section title="For you">
      <Ask id="unit">One board per what?
        <Option key="a">per session</Option>
        <Option key="b">per project</Option>
      </Ask>
      <Item id="hook">Decide whether to install the hook
        Installing it touches ~/.claude/settings.json.
      </Item>
    </Section>

    <Section title="In progress">
      <Item id="impl" state="doing" status="working">Implement the viewer</Item>
      <Item id="parts" status="waiting on you" note="needs the answer above">Fix the part list</Item>
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
    Title     the page title, one line
    Section   title="..." (required). Holds Item Ask Link Code Fold and text.
              Not inside another Section or a Fold.
    Item      id (required), state = todo (default) | doing | done,
              status (a short word shown at the right end),
              note (why the status; shown when opened).
              First line: the item. Following lines: what opening it shows
              (text and Code only).
    Ask       id (required). First line: the question. Then 2 or more Option.
    Option    key (required, one letter a-z, unique in its Ask; not one of
              the viewer's own keys c j k m n q x y). One line.
    Link      id (required), and exactly one of path (a file, shown as
              written) or url (http:// or https://). One line: its title.
              The board shows the path or URL under the title; it does not
              open files. Put what must be read in the board in Code.
    Fold      title (required), id (optional; keeps it open across retitling
              when folds are added or removed above it). Holds what a
              Section holds. Starts closed.
    Code      lang (optional; "diff" is colored). Raw until </Code>; the
              indentation of the <Code> tag is removed from its lines.
  id: 1-16 of a-z 0-9 -, unique in the page. The person reacts to Item, Ask
  and Link through their id: y / n on Items and Links, an Option key on an
  Ask, and thoughts written on any of them.

Manners:
  - The report is skimmed, not read. One item per line; details go in the
    item's following lines (shown when the person opens it).
  - Keep ids stable while a line means the same thing: reactions are tied to
    ids. A reaction stops showing when you rewrite that line (text, state,
    status or note), so rewrite a line once you have acted on its reaction.
  - Never reuse an id for a different line. When the thing changes, give it
    a new id. (Reactions and thoughts record the title the person saw;
    'changes' shows it next to what the line says now.)
  - Tags start at the beginning of a line and part names start with a
    capital letter (<Item>, not <item>).
  - Run 'pitboard check' after every rewrite and fix what it reports.
  - Read 'pitboard changes' at the start of each turn and before you stop.
    You need not handle every thought at once, but give every thought you
    received a destination with 'pitboard settle N TEXT' (e.g. "made it a
    todo", "answered", "noted", "not doing it").

'changes' prints lines like:
    y hook (Decide whether to install the hook)
    n impl (Implement the viewer → いまは: Implement the viewer and the CLI)
    y old (Drop the log → いまは無い)
    answer unit = a (per session)
    thought 7 on hook (Decide whether to install the hook): only post-tool
    thought 8: maybe drop the log
    hide: Drop the log
    unhide: Drop the log
  hide means the person decided they no longer need to see that line. Nothing
  needs doing; you may drop the line the next time you rewrite the report.
  (The viewer hides a line by what it says: when you change its title, state
  or status, it shows again as new. Changing only its contents, note, path
  or options does not bring it back; neither does adding to a hidden Fold.)
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
	case "settle":
		if len(args) < 2 {
			return errors.New("usage: pitboard settle N TEXT")
		}
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("settle: %q is not a thought number", args[0])
		}
		return st.Settle(n, strings.Join(args[1:], " "))
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
