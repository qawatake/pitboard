package main

import (
	"os"
	"strings"
	"testing"
)

func TestSkillIsThin(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"--skill"}, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.HasPrefix(s, "---\nname: pitboard\ndescription: ") || !strings.Contains(s, "pitboard -h") || strings.Contains(s, "hook post-tool") {
		t.Errorf("skill:\n%s", s)
	}
	out.Reset()
	run([]string{"-h"}, &out)
	for _, w := range []string{"pitboard hook post-tool", "settle", "scratchpad", "PITBOARD_SEND", "herdr agent prompt", "→ now:"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("-h lacks %q", w)
		}
	}
}

// The skill printed is skills/pitboard/SKILL.md, the file gh skill install
// reads.
func TestSkillMatchesSKILLmd(t *testing.T) {
	md, err := os.ReadFile("../../skills/pitboard/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if skill != string(md) {
		t.Error("cmd/pitboard/skill.go differs from skills/pitboard/SKILL.md: change skill.go to match SKILL.md")
	}
}
