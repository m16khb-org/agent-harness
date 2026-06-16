package hookinput

import "testing"

func TestSkillFromHookInput(t *testing.T) {
	if got := SkillFromHookInput([]byte(`{"tool_name":"Skill","tool_input":{"skill":"codd"}}`)); got != "codd" {
		t.Fatalf("skill = %q, want codd", got)
	}
	if got := SkillFromHookInput([]byte(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)); got != "" {
		t.Fatalf("non-skill tool should yield empty, got %q", got)
	}
	if got := SkillFromHookInput([]byte(`{}`)); got != "" {
		t.Fatalf("missing tool_input should yield empty, got %q", got)
	}
}
