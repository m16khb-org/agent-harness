package lifecycle

import (
	"strings"
	"testing"
)

func TestRecordLifecycleToolUseSkipsReadOnlyBashOutputPaths(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}

	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:    repo,
		Tool:    "Bash",
		Command: "rg -n \"PostCompact|OPEN_API_SPEC\" .",
		Paths:   []string{"cmd/harness/hook_user_prompt.go", ".agent-harness/OPEN_API_SPEC.md"},
		Source:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Recorded {
		t.Fatalf("read-only Bash should not queue doc upkeep: %+v", result)
	}

	pre := BuildLifecyclePreCompactCapsule(repo)
	if pre.Recorded || pre.PendingCount != 0 {
		t.Fatalf("read-only Bash should leave no compaction capsule work: %+v", pre)
	}
}

func TestRecordLifecycleToolUseSkipsQuotedRedirectInReadOnlyBash(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}

	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:    repo,
		Tool:    "Bash",
		Command: "rg -n 'a > b' internal/core/hook_prompt.go",
		Paths:   []string{"internal/core/hook_prompt.go"},
		Source:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Recorded {
		t.Fatalf("quoted redirect in read-only Bash should not queue doc upkeep: %+v", result)
	}
}

func TestRecordLifecycleToolUseAllowsMutatingBashCommand(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}

	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:    repo,
		Tool:    "Bash",
		Command: "gofmt -w internal/core/lifecycle_state.go",
		Source:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Recorded || !containsString(result.Event.TargetDocs, "OPERATIONS.md") {
		t.Fatalf("mutating Bash should queue lifecycle doc upkeep: %+v", result)
	}
}

func TestRecordLifecycleToolUseQueuesRelevantDocUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:   repo,
		Tool:   "apply_patch",
		Paths:  []string{"internal/core/hook_prompt.go", "internal/core/hook_prompt_test.go"},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Recorded || result.Event.ID == "" {
		t.Fatalf("expected recorded upkeep event: %+v", result)
	}
	if !containsString(result.Event.TargetDocs, "OPERATIONS.md") || !containsString(result.Event.TargetDocs, "TESTING.md") {
		t.Fatalf("expected operations/testing targets: %+v", result.Event.TargetDocs)
	}
}

func TestBuildLifecycleStopReminderIncludesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{Kind: "code_change", TargetDocs: []string{"OPERATIONS.md"}, Summary: "Hook behavior changed.", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	reminder := BuildLifecycleStopReminder(repo)
	if !reminder.ShouldInject || !strings.Contains(reminder.AdditionalContext, "Pending .agent-harness doc upkeep") || !strings.Contains(reminder.AdditionalContext, "OPERATIONS.md") {
		t.Fatalf("unexpected reminder: %+v", reminder)
	}
}

func TestLifecycleCompactReminderDeduplicatesRepeatedUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	event := DocUpkeepEvent{
		Kind:       "code_change",
		TargetDocs: []string{"OPEN_API_SPEC.md"},
		Summary:    "Bash touched harness lifecycle-relevant files; shared project docs may need review.",
		Source:     "test",
	}
	if _, err := AppendDocUpkeepEvent(repo, event); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(repo, event); err != nil {
		t.Fatal(err)
	}

	pre := BuildLifecyclePreCompactCapsule(repo)
	if !pre.Recorded || pre.PendingCount != 2 {
		t.Fatalf("pre-compact should preserve both queued events before rendering: %+v", pre)
	}
	post := BuildLifecyclePostCompactReminder(repo)
	if !post.ShouldInject || strings.Count(post.AdditionalContext, "OPEN_API_SPEC.md") != 2 {
		t.Fatalf("post-compact context should keep compact target-doc routing: %s", post.AdditionalContext)
	}
	if strings.Contains(post.AdditionalContext, event.Summary) {
		t.Fatalf("post-compact context should defer detailed upkeep rows to UserPromptSubmit: %s", post.AdditionalContext)
	}
}
