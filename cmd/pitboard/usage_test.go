package main

import (
	"strings"
	"testing"

	"github.com/qawatake/pitboard/internal/report"
)

func TestHelpListsViewerKeys(t *testing.T) {
	if !strings.Contains(usage, "the viewer's own keys "+report.ViewerKeyList()+")") || strings.Contains(usage, "{viewerKeys}") {
		t.Error("-h does not list viewerKeys")
	}
	if !strings.Contains(report.ViewerKeys, "u") {
		t.Error("u (undo) is a viewer key")
	}
}
