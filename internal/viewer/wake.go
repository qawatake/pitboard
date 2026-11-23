package viewer

import (
	"strings"
	"time"
)

// wake types a line into the AI's terminal telling it that the board has
// something it has not read. What was sent is not marked as delivered: that
// happens when the AI runs 'changes'.
func (a *app) wake() {
	if !a.canWrite() {
		return
	}
	// Waking may take seconds: say so, and drop the keys pressed meanwhile
	// (they were pressed at a screen that said nothing yet, and would wipe
	// out the result).
	waited := false
	msg, err := a.b.Send(func(m string) {
		a.msg, waited = m, true
		a.render()
	})
	if waited {
		a.dropBefore = time.Now()
	}
	if err != nil {
		a.msg = err.Error()
		return
	}
	a.msg = msg
	if strings.Contains(msg, "\n") {
		a.msgLines = 2
	}
}
