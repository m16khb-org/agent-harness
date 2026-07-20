package hookcli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesIssueOpsWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	obj := runHookCapture(t, `{"cwd":"`+source+`","tool_name":"apply_patch","tool_input":{"file_path":"`+source+`/.agent-harness/OPERATIONS.md"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", worktree, "--source-checkout", source, "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected source checkout edit to remain available, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsApplyPatchInsideExpectedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "api-servers")
	worktree := filepath.Join(filepath.Dir(source), "api-servers.worktrees", "2193-demo")
	patch := "*** Begin Patch\n*** Add File: " + filepath.Join(worktree, "SMOKE.go") + "\n+package smoke\n*** End Patch\n"
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "apply_patch",
		"tool_input": map[string]any{
			"patch": patch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", worktree, "--source-checkout", source, "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected apply_patch target inside expected worktree to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsSourceApplyPatchOutsideExpectedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "api-servers")
	worktree := filepath.Join(filepath.Dir(source), "api-servers.worktrees", "2193-demo")
	patch := "*** Begin Patch\n*** Add File: " + filepath.Join(source, "SMOKE.go") + "\n+package smoke\n*** End Patch\n"
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "apply_patch",
		"tool_input": map[string]any{
			"patch": patch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", worktree, "--source-checkout", source, "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected source apply_patch to remain available, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksIssueBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git checkout -b 2387-fix-grpc-ai-dmm-tag-replication-lag",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected issue-number branch creation without source ref to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "source ref") || !strings.Contains(reason, "2387-fix-grpc-ai-dmm-tag-replication-lag") {
		t.Fatalf("expected source-ref guidance, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksIssueBranchCreationWithSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git switch -c 2386-remove-dmm-ranking-ranktype origin/main",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected issue branch creation with source ref to be blocked without IssueOps state, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "started through IssueOps") || !strings.Contains(reason, "2386-remove-dmm-ranking-ranktype") {
		t.Fatalf("expected IssueOps bootstrap guidance, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git switch -c 2386-remove-dmm-ranking-ranktype",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected branch creation without source ref to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "source ref") || !strings.Contains(reason, "ask the user") {
		t.Fatalf("expected source-ref guidance, got %q", reason)
	}
}
