package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestRunHookPreToolUseAllowsAnyLinkedIssueOpsWorktreeForRepo(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := createLinkedIssueOpsWorktree(t, source, "95-tier-matrix-scan-allowance-policy")
	second := createLinkedIssueOpsWorktree(t, source, "96-integrate-public-seo-rendering")
	third := createLinkedIssueOpsWorktree(t, source, "97-parallel-worktree-fixture")
	fixtures := []linkedIssueOpsWorktree{first, second, third}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].id < fixtures[j].id
	})
	target := fixtures[len(fixtures)-1]

	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path": filepath.Join(source, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected source checkout edit to be allowed when no active cycle on current branch, got %+v", obj)
	}

	payload, err = json.Marshal(map[string]any{
		"cwd":       target.path,
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path": filepath.Join(target.path, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj = runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected linked IssueOps worktree edit to be allowed while first exists at %s, got %+v", fixtures[0].path, obj)
	}

	payload, err = json.Marshal(map[string]any{
		"cwd":       target.path,
		"tool_name": "mcp__filesystem__read_file",
		"tool_input": map[string]any{
			"path": filepath.Join(source, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj = runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected read-only filesystem MCP observation across linked worktrees to be allowed, got %+v", obj)
	}

	// External code-intelligence MCP tools are no longer special-cased by the
	// worktree guard; they pass through regardless of projectPath.
	payload, err = json.Marshal(map[string]any{
		"cwd":       target.path,
		"tool_name": "mcp__codegraph__codegraph_search",
		"tool_input": map[string]any{
			"projectPath": source,
			"query":       "BuildLifecyclePreToolUseDecision",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj = runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected external search MCP tool to pass through the worktree guard, got %+v", obj)
	}
}
