package hookcli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunHookPreToolUseAllowsIssueBasedGitLabMRWithCombinedGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab mr for 2385 --with-labels --assignee 100`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected issue-based GitLab MR with labels and numeric assignee to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredRelatedIssueMRRequiresNumericAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"related_issue":     "2385",
				"copy_issue_labels": true,
				"assignee":          "m16khb",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured related-issue MR with username assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "numeric assignee") {
		t.Fatalf("expected numeric assignee reason, got %q", reason)
	}
}
