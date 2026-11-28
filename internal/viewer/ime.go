package viewer

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// A terminal program cannot turn the input method off. When the editor
// closes, the keys that follow are the viewer's (s, j, x...), and an input
// method left in Japanese would compose them instead. So the viewer runs the
// command the person gives in $PITBOARD_IME_OFF (with sh -c), which switches
// the input method to ASCII, e.g. 'macism com.apple.keylayout.ABC'. Unset:
// nothing is run.

// imeTimeout is how long the command may run before it is cut off.
const imeTimeout = 2 * time.Second

// imeRan is told when the command has ended (tests wait on it).
var imeRan func()

// imeOff runs $PITBOARD_IME_OFF in the background, without the terminal:
// its output is discarded and a failure is ignored (the viewer goes on as
// it would without it).
func imeOff() {
	sh := os.Getenv("PITBOARD_IME_OFF")
	if sh == "" {
		return
	}
	done := imeRan
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), imeTimeout)
		defer cancel()
		c := exec.CommandContext(ctx, "sh", "-c", sh)
		c.WaitDelay = 500 * time.Millisecond // a child left holding the pipes does not keep it running
		_ = c.Run()                          // stdin, stdout, stderr: the null device
		if done != nil {
			done()
		}
	}()
}
