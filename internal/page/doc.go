// Package page is what a pitboard screen shows and what the person's
// actions on it do, whatever the screen is: the terminal viewer
// (internal/viewer) and a browser alike.
//
// It reads the report (Report) and the record (Board), and says, for each
// line, what it is, what the person can do with it, which marks it gets and
// what words go at its right end (Page builds all of it as a tree of
// Things). The person's actions (React, Hide, Comment, Read, Undo, Send,
// ...) are recorded through it, and each returns the words to tell the
// person. The words a screen shows (未送信, 送りました, ...) are here, so
// that every screen says the same.
//
// What belongs to one kind of screen stays with it: what is open, the
// cursor, scrolling, the editor, keys, drawing and the actual colors (a
// Tone says what a color means, not which it is).
package page
