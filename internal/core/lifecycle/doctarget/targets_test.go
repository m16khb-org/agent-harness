package doctarget

import (
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestForToolUseSkipsReadOnlyBashOutputPaths(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "rg -n \"PostCompact|OPEN_API_SPEC\" .",
		Paths:   []string{"cmd/harness/hook_user_prompt.go", ".agent-harness/OPEN_API_SPEC.md"},
		Source:  "test",
	})
	if len(targets) != 0 {
		t.Fatalf("read-only Bash should not queue doc upkeep targets: %+v", targets)
	}
}

func TestForToolUseSkipsQuotedRedirectInReadOnlyBash(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "rg -n 'a > b' internal/core/hook_prompt.go",
		Paths:   []string{"internal/core/hook_prompt.go"},
		Source:  "test",
	})
	if len(targets) != 0 {
		t.Fatalf("quoted redirect in read-only Bash should not queue doc upkeep targets: %+v", targets)
	}
}

func TestForToolUseAllowsMutatingBashCommand(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "gofmt -w internal/core/lifecycle_state.go",
		Source:  "test",
	})
	if !containsString(targets, "OPERATIONS.md") {
		t.Fatalf("mutating Bash should queue lifecycle doc upkeep targets: %+v", targets)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
