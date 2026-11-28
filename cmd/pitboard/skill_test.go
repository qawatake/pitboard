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
	// The skill says which topic to read when.
	for _, tp := range topics {
		if !strings.Contains(s, "pitboard help "+tp.name) {
			t.Errorf("skill does not point at pitboard help %s", tp.name)
		}
	}
	out.Reset()
	run([]string{"-h"}, &out)
	for _, w := range []string{"pitboard hook post-tool", "settle", "scratchpad", "→ now:", "AI の番", "old_string", "Topics (pitboard help TOPIC):"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("-h lacks %q", w)
		}
	}
	// What -h says is not repeated in the skill.
	if strings.Contains(s, "AI の番") {
		t.Error("the skill repeats the AI の番 paragraph of -h")
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
