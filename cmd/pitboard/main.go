package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
	"github.com/qawatake/pitboard/internal/viewer"
	"github.com/qawatake/pitboard/internal/web"
)

// hookStdin is where `pitboard hook` reads the hook's JSON (tests replace it).
var hookStdin io.Reader = os.Stdin

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
func settleArgs(args []string) ([]board.Settlement, error) {
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
	var out []board.Settlement
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
		out = append(out, board.Settlement{N: n, State: state, Text: strings.Join(rest, " ")})
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
	base := board.Root(cwd)
	cmd := "view"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if cmd == "help" {
		fmt.Fprint(out, usage)
		return nil
	}

	var st *board.Store
	if session != "" {
		b, name, ok, err := board.FindSession(cwd, session, true)
		if err != nil {
			return err
		}
		if !ok {
			b, name = base, session
		}
		st = board.Open(b, name)
	}
	if cmd == "view" || cmd == "serve" {
		if st == nil {
			if ss := board.Sessions(base); len(ss) > 0 {
				st = board.Open(base, ss[0])
			}
		}
		if cmd == "serve" {
			return runServe(base, st, args, out)
		}
		code, err := viewer.Run(base, st)
		if err == nil && code != 0 {
			err = exitError(code)
		}
		return err
	}
	if st == nil {
		return errors.New("no session: give -s SESSION or set $PITBOARD_SESSION")
	}
	if _, err := os.Stat(st.Dir); err != nil && cmd != "path" {
		return fmt.Errorf("no session %q yet (pitboard -s %s path creates it)", st.Session, st.Session)
	}

	switch cmd {
	case "path":
		if err := os.MkdirAll(st.Dir, 0o755); err != nil {
			return err
		}
		fmt.Fprintln(out, st.ReportPath())
	case "changes":
		lines, err := st.Changes()
		if err != nil {
			return err
		}
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		st.NoteOpen(out)
		st.NoteProblems(out)
	case "open":
		st.ListOpen(out)
	case "settle":
		if len(args) < 1 {
			return errors.New("usage: pitboard settle N [--doing|--hold|--done] TEXT, or settle --STATE N TEXT --STATE N TEXT ...")
		}
		ss, err := settleArgs(args)
		if err != nil {
			return err
		}
		err = st.SettleAll(ss)
		st.NoteProblems(out)
		return err
	case "wait":
		return runWait(st, args, out)
	case "check":
		b, err := os.ReadFile(st.ReportPath())
		if err != nil {
			return err
		}
		_, probs, perr := report.Parse(string(b))
		if perr != nil {
			return perr
		}
		for _, p := range probs {
			fmt.Fprintf(out, "%s:%d: %s\n", st.ReportPath(), p.Line, p.Msg)
		}
		if len(probs) > 0 {
			return exitError(1)
		}
	default:
		return fmt.Errorf("unknown command %q (see pitboard -h)", cmd)
	}
	return nil
}

// runWait is 'pitboard wait [--timeout DURATION]'.
func runWait(st *board.Store, args []string, out io.Writer) error {
	var timeout time.Duration
	for len(args) > 0 {
		v, ok := "", false
		switch a := args[0]; {
		case a == "--timeout" || a == "-timeout":
			if len(args) < 2 {
				return errors.New("--timeout needs a duration (e.g. 30m)")
			}
			v, ok, args = args[1], true, args[2:]
		case strings.HasPrefix(a, "--timeout="), strings.HasPrefix(a, "-timeout="):
			_, v, _ = strings.Cut(a, "=")
			ok, args = true, args[1:]
		default:
			return fmt.Errorf("wait: unknown argument %q (usage: pitboard wait [--timeout DURATION])", a)
		}
		if ok {
			d, err := time.ParseDuration(v)
			if err != nil || d < 0 {
				return fmt.Errorf("--timeout %q: use a duration like 30m or 1h", v)
			}
			timeout = d
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return st.Wait(ctx, timeout, out)
}

// runServe is 'pitboard serve [--port N] [--no-open]': the board in a
// browser, on 127.0.0.1, until ctrl+c.
func runServe(base string, st *board.Store, args []string, out io.Writer) error {
	o := web.Options{Base: base, Store: st, Open: true, Out: out}
	for len(args) > 0 {
		a := args[0]
		v, hasV := "", false
		if k, val, ok := strings.Cut(a, "="); ok {
			a, v, hasV = k, val, true
		}
		switch a {
		case "--no-open", "-no-open":
			if hasV {
				return fmt.Errorf("serve: %s takes no value", a)
			}
			o.Open, args = false, args[1:]
		case "--port", "-port":
			if !hasV {
				if len(args) < 2 {
					return errors.New("--port needs a number")
				}
				v, args = args[1], args[1:]
			}
			args = args[1:]
			p, err := strconv.Atoi(v)
			if err != nil || p < 0 || p > 65535 {
				return fmt.Errorf("--port %q: give a port number (0: any free one)", v)
			}
			o.Port = p
		default:
			return fmt.Errorf("serve: unknown argument %q (usage: pitboard serve [--port N] [--no-open])", args[0])
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return web.Run(ctx, o)
}
