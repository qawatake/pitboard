package page

// Untouched is what of a works on a line a screen put the cursor on by
// itself (at start, or when stops appear on a page that had none), before
// the person pointed at it. Hiding it, saying it is read and writing a
// comment on it work: they show on the screen at once, wait in the record
// until sent, and u takes them back. y / n and an option answer the AI, so
// they wait until the person points at the line.
func (a Actions) Untouched() Actions {
	a.YN, a.Options = false, nil
	return a
}
