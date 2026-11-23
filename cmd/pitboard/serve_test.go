package main

import (
	"io"
	"strings"
	"testing"
)

// serve refuses what it does not know before listening.
func TestServeArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	for args, want := range map[string]string{
		"serve --bogus":       "unknown argument",
		"serve --port":        "--port needs",
		"serve --port x":      "give a port number",
		"serve --port=70000":  "give a port number",
		"serve --no-open=yes": "takes no value",
	} {
		err := run(append([]string{"-s", "abc"}, strings.Fields(args)...), io.Discard)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", args, err)
		}
	}
}
