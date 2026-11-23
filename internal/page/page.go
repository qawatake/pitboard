package page

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/qawatake/pitboard/internal/board"
	"github.com/qawatake/pitboard/internal/report"
)

// Page is everything a screen shows, built from the report and the record
// as they are now. A screen adds only its own state (what is open, where
// the cursor is) and its look.
type Page struct {
	// Message is what to show in place of the report, when there is none
	// to show (no session, no report yet, an empty one): MsgNoSession,
	// MsgNoSuchSession, MsgNoReport, MsgEmpty. A screen that knows there
	// is no .pitboard/ at all says MsgNoBoard instead of MsgNoSession.
	Message string
	Err     string // the report could not be read: "読めませんでした: " + this
	ReadErr string // the last read failed (a bug); what was read before is shown
	// Problems are the report's format errors, as shown under MsgProblems
	// (+ MsgPartial when Partial: some of it could not be shown).
	Problems []string `json:",omitempty"`
	Partial  bool     `json:",omitempty"`
	// ProblemHead is the heading over Problems (MsgProblems, + MsgPartial).
	ProblemHead string `json:",omitempty"`

	Notices []*Thing // shown above the page, in their order
	Things  []*Thing // the page, in order (notices left out)

	Waiting    int  // lines waiting for the person's reaction (反応待ち)
	Unread     int  // lines unread, not waiting (未読)
	Unsent     int  // what the person has not sent (未送信)
	SentUnread int  // what was sent and the AI has not read (送信済)
	AIWaiting  bool // a 'pitboard wait' is waiting (AI 待機中)
	Hidden     int  // lines hidden (隠した行 N)
	CanWrite   bool // the person's actions can be recorded
	CanUndo    bool // there is something to take back
	CanSend    bool // there is something to send
	// Jumps are the places to go straight to (notices, Sections, top-level
	// Folds), each a Thing of Notices or Things. JumpTo is, for each, the
	// line to go to: its own, or a Section's first (a Thing's Jump says
	// which place it is).
	Jumps  []*Thing `json:"-"`
	JumpTo []string
	// Next is the lines "next marked line" walks through, in the order of
	// the page (see Board.Lines): those with a mark or waiting on the
	// person.
	Next []Next
	// Summary is the counts at the top right, in order (Board.Summary).
	Summary []Word
	// Comments are all the person's comments, in the order to show them
	// under the report (Board.Pending); Newest is the same, newest first.
	Comments []*Comment
}

// Kind is the kind of a Thing.
type Kind int

const (
	KindNotice  Kind = iota
	KindTitle        // the page title
	KindSection      // a heading and what it holds
	KindItem
	KindAsk
	KindOption // an Ask's option
	KindLink
	KindFold
	KindPara // a paragraph (Head: a small heading)
	KindList // a list line ("- " or numbered; Subs nested under it)
	KindCode
)

// Style is how a Span of inline text looks.
type Style int

const (
	StylePlain Style = iota
	StyleBold
	StyleCode
	StyleLink // URL is where it goes
)

// Span is a run of inline text in one style. Text may hold "\n" where the
// AI asked for a line break.
type Span struct {
	Text  string
	Style Style
	URL   string
}

// Thing is one line of the page (and what it holds).
type Thing struct {
	Kind Kind
	Node *report.Node `json:"-"` // what it was built from
	// Key says which line it is (the cursor follows it); Target is what
	// reactions and comments on it are recorded against ("" when none).
	Key    string `json:",omitempty"`
	Target string `json:",omitempty"`
	Title  []Span `json:",omitempty"` // the line (a paragraph's or list line's text too)
	// Stop: the person can point at it (and so act on it).
	Stop    bool    `json:",omitempty"`
	CanOpen bool    `json:",omitempty"`
	Actions Actions `json:",omitzero"`
	Mark    Mark    `json:",omitempty"`
	// Sign is the mark at the head of the line (SignOf), closed and open.
	Sign     string `json:",omitempty"`
	OpenSign string `json:",omitempty"`
	// You: an Item with wait="you" (it cannot move on until the person
	// does something). Not a mark, but a place to be taken to.
	You    bool `json:",omitempty"`
	Hidden bool `json:",omitempty"` // hidden by the person (shown only with showHidden)
	Seen   Seen `json:"-"`          // what an action on it must carry
	// Token stands for Seen (Seen.Token): a screen that cannot keep Seen
	// itself carries this instead.
	Token string `json:",omitempty"`
	// Open says whether it starts open (StartsOpen).
	Open  bool      `json:",omitempty"`
	Right Right     `json:",omitzero"`
	State string    `json:",omitempty"` // an Item's: todo, doing, done
	Note  string    `json:",omitempty"` // an Item's note: why its status (shown when opened)
	Path  string    `json:",omitempty"` // a Link's file path, as written
	URL   string    `json:",omitempty"` // a Link's URL
	At    time.Time `json:",omitzero"`  // a Notice's: when the AI put it up
	// Comments are the comments written on it, oldest first.
	Comments []*Comment `json:",omitempty"`
	// Body is what opening an Item or Notice shows (paragraphs, list
	// lines, code); Kids what a Section or Fold holds, or an Ask's
	// options; Subs the lines nested under a list line.
	Body      []*Thing `json:",omitempty"`
	Kids      []*Thing `json:",omitempty"`
	Subs      []*Thing `json:",omitempty"`
	OptionKey string   `json:",omitempty"` // an Option's key
	Chosen    bool     `json:",omitempty"` // an Option the person chose
	Num       string   `json:",omitempty"` // a numbered list line's number as written ("1.")
	Level     int      `json:",omitempty"` // a list line's nesting (0-2)
	Head      bool     `json:",omitempty"` // a paragraph that is a small heading
	Gap       bool     `json:",omitempty"` // a blank line came before it
	Lang      string   `json:",omitempty"` // Code: its language ("diff" is colored)
	Code      []string `json:",omitempty"` // Code: its lines
	Depth     int      `json:",omitempty"` // how deep in Folds (and Asks, for options)
	// None is what a Section that holds nothing shows, faint, under its
	// heading (MsgNone; "" otherwise). See EmptySection.
	None string `json:",omitempty"`
	// Jump is which of Page.Jumps it is (1 for the first; 0 none), and
	// JumpKey the letter g goes there with (JumpLetter).
	Jump    int    `json:",omitempty"`
	JumpKey string `json:",omitempty"`
}

// firstStop is the key of the line going to t goes to: t's, or the first
// a Section holds ("" none).
func firstStop(t *Thing) string {
	if t.Stop {
		return t.Key
	}
	for _, k := range t.Kids {
		if f := firstStop(k); f != "" {
			return f
		}
	}
	return ""
}

// Comment is one of the person's comments, as shown.
type Comment struct {
	N      int
	Text   string // may hold "[Image #N]"
	Target string // the line it was written on ("" for none)
	// About is that line's title (see Board.About).
	About string
	// TitleThen is the line's title when it was written, if it is not
	// what the line says now ("" otherwise).
	TitleThen   string
	Word        Word   // its status in one word
	Settled     string // what the AI wrote on it ("" none), after SettledHead
	SettledHead string
	Note        string // when the AI has written nothing: how far it got
	Finished    bool   // the AI is done with it
	Images      []int  // the images it holds ([Image #N])
}

// Page builds the page as it is now. showHidden keeps the lines the person
// hid (with Hidden set) instead of leaving them out.
func (b *Board) Page(showHidden bool) *Page {
	p := &Page{
		Unsent: b.Unsent, SentUnread: b.Unread, AIWaiting: b.Waiter,
		CanWrite: b.CanWrite(), CanUndo: b.CanWrite() && len(b.Hist) > 0,
	}
	p.CanSend = p.CanWrite && p.Unsent > 0
	r := b.Report
	switch {
	case b.Store == nil:
		p.Message = MsgNoSession
		return p
	case b.NoSession:
		p.Message = MsgNoSuchSession(b.Store.Session)
		return p
	case r.Missing:
		p.Message = MsgNoReport
		return p
	}
	p.Err, p.ReadErr = r.Err, r.ReadErr
	if r.Err == "" && r.ReadErr == "" && len(r.Nodes) == 0 && len(r.Probs) == 0 {
		p.Message = MsgEmpty
		return p
	}
	for _, pr := range r.Probs {
		p.Problems = append(p.Problems, ProblemLine(pr))
	}
	p.Partial = report.Partial(r.Probs)
	if len(p.Problems) > 0 {
		p.ProblemHead = MsgProblems
		if p.Partial {
			p.ProblemHead += MsgPartial
		}
	}
	p.Waiting, p.Unread = b.Counts(showHidden)
	p.Summary = b.Summary(showHidden)
	p.Hidden = b.HiddenCount()
	comments := map[int]*Comment{}
	for _, t := range b.Pending() {
		c := b.comment(t)
		comments[t.N] = c
		p.Comments = append(p.Comments, c)
	}
	byNode := map[*report.Node]*Thing{}
	var build func(ns []*report.Node) []*Thing
	build = func(ns []*report.Node) []*Thing {
		var out []*Thing
		for _, n := range ns {
			if !showHidden && (b.IsHidden(n) || (n.Kind == report.KindSection && b.allHidden(n.Kids))) {
				continue
			}
			t := b.thing(n, comments)
			byNode[n] = t
			t.Kids = build(n.Kids)
			if EmptySection(n) {
				t.None = MsgNone
			}
			if t.Kind == KindAsk {
				for _, o := range t.Kids {
					o.Chosen = t.Right.Reaction != "" && o.OptionKey == t.Right.Reaction
				}
			}
			out = append(out, t)
		}
		return out
	}
	for _, t := range build(r.Nodes) {
		if t.Kind == KindNotice {
			p.Notices = append(p.Notices, t)
		} else {
			p.Things = append(p.Things, t)
		}
	}
	for _, n := range b.Jumps(showHidden) {
		if t := byNode[n]; t != nil {
			p.Jumps = append(p.Jumps, t)
			t.Jump = len(p.Jumps)
			t.JumpKey = JumpLetter(t.Jump - 1)
			p.JumpTo = append(p.JumpTo, firstStop(t))
		}
	}
	for _, l := range b.Lines(showHidden) {
		if l.Mark == MarkNone && !l.You {
			continue
		}
		nx := Next{Key: l.Node.Key}
		for _, f := range l.Folds {
			nx.Folds = append(nx.Folds, f.Key)
		}
		p.Next = append(p.Next, nx)
	}
	return p
}

// EmptySection says whether n is a Section that holds nothing: every
// screen shows MsgNone, faint, under its heading (rather than a bare
// heading, which reads as if something failed to show). A Section whose
// lines are all hidden is not shown at all, so it is not one of these.
func EmptySection(n *report.Node) bool {
	if n.Kind != report.KindSection {
		return false
	}
	for _, k := range n.Kids {
		if k.Kind != report.KindNotice {
			return false
		}
	}
	return true
}

var kinds = map[report.Kind]Kind{
	report.KindNotice: KindNotice, report.KindTitle: KindTitle, report.KindSection: KindSection,
	report.KindTodo: KindItem, report.KindDoing: KindItem, report.KindDone: KindItem,
	report.KindQuestion: KindAsk, report.KindChoice: KindOption, report.KindLink: KindLink,
	report.KindDetails: KindFold, report.KindPara: KindPara, report.KindPlain: KindList,
	report.KindCode: KindCode,
}

// thing is n as shown (not what it holds: Kids are the caller's).
func (b *Board) thing(n *report.Node, comments map[int]*Comment) *Thing {
	t := &Thing{
		Kind: kinds[n.Kind], Node: n, Key: n.Key, Target: report.Target(n),
		Title: Spans(n.Text), Note: n.Reason, Path: n.Path, URL: n.URL, At: n.At,
		OptionKey: n.Label, Num: n.Num, Level: n.Level, Head: n.Head, Gap: n.Gap,
		Lang: n.Lang, Code: n.Code, Depth: n.Depth, Hidden: b.IsHidden(n),
	}
	switch n.Kind {
	case report.KindTodo:
		t.State = "todo"
	case report.KindDoing:
		t.State = "doing"
	case report.KindDone:
		t.State = "done"
	}
	for _, c := range n.Body {
		t.Body = append(t.Body, b.thing(c, comments))
	}
	for _, c := range n.Subs {
		t.Subs = append(t.Subs, b.thing(c, comments))
	}
	if !Stops(n) {
		return t
	}
	t.Stop = true
	t.Seen = SeenOf(n)
	t.Token = t.Seen.Token()
	t.Open = StartsOpen(n)
	t.CanOpen = b.CanOpen(n)
	t.Actions = b.ActionsOf(n)
	t.Mark = b.MarkOf(n)
	t.You = n.Wait == "you" && !t.Hidden
	t.Right = b.RightOf(n)
	if !t.Right.At.IsZero() {
		t.Right.AtText = AtWord(t.Right.At, Clock())
	}
	t.Sign, t.OpenSign = SignOf(n, t.CanOpen, false, t.Hidden), SignOf(n, t.CanOpen, true, t.Hidden)
	if t.Target != "" {
		for _, th := range b.State.On(t.Target) {
			if c := comments[th.N]; c != nil {
				t.Comments = append(t.Comments, c)
			}
		}
	}
	return t
}

func (b *Board) comment(t *board.Thought) *Comment {
	c := &Comment{N: t.N, Text: strings.TrimSpace(t.Text), Target: t.ID, About: b.About(t),
		Word: b.ThoughtWord(t), Finished: t.Finished()}
	if t.ID != "" {
		if n := report.ByID(b.Report.Nodes, t.ID); n != nil && t.Title != "" && t.Title != n.Title() {
			c.TitleThen = t.Title
		}
	}
	if t.SettledState != "" {
		c.Settled, c.SettledHead = t.Settled, SettleHead(t.SettledState)
	} else {
		c.Note = b.ThoughtNote(t)
	}
	for _, m := range board.ReImageRef.FindAllStringSubmatch(t.Text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			c.Images = append(c.Images, n)
		}
	}
	return c
}

var reInline = regexp.MustCompile("`([^`]+)`|\\[([^\\]]+)\\]\\(([^)]*)\\)|\\*\\*([^*]+)\\*\\*")

// Spans is inline Markdown as runs of styled text: code spans (without
// backticks), links (their title, and the URL) and **bold**.
func Spans(s string) []Span {
	var out []Span
	add := func(text string, st Style, url string) {
		if text != "" {
			out = append(out, Span{Text: text, Style: st, URL: url})
		}
	}
	for {
		m := reInline.FindStringSubmatchIndex(s)
		if m == nil {
			add(s, StylePlain, "")
			return out
		}
		add(s[:m[0]], StylePlain, "")
		switch {
		case m[2] >= 0:
			add(s[m[2]:m[3]], StyleCode, "")
		case m[4] >= 0:
			add(s[m[4]:m[5]], StyleLink, s[m[6]:m[7]])
		default:
			add(s[m[8]:m[9]], StyleBold, "")
		}
		s = s[m[1]:]
	}
}
