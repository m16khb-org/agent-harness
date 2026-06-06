package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunHookPreToolUseEnforcesLinkedIssueOpsWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookIntentForTest(t, record.ID)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/12",
		Branch:       "12-issue-worktree",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(source), "agent-harness.worktrees", "12-issue-worktree")
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookDesignForTest(t, record.ID)
	writeHookFixtureFile(t, worktree, "plans/issue-worktree.md", "plan\n")
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), record.ID, filepath.Join(worktree, "plans", "issue-worktree.md")); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "apply_patch",
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
	if obj["decision"] != "block" {
		t.Fatalf("expected linked IssueOps worktree guard to block source checkout edit, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "linked IssueOps worktree") {
		t.Fatalf("expected linked worktree reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksSourceCheckoutWhenLinkedCycleExists(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookIntentForTest(t, record.ID)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/12",
		Branch:       "12-issue-worktree",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(source), "agent-harness.worktrees", "12-issue-worktree")
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookDesignForTest(t, record.ID)
	planPath := filepath.Join(worktree, "plans", "demo.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), record.ID, planPath); err != nil {
		t.Fatal(err)
	}
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
	if obj["decision"] != "block" {
		t.Fatalf("expected linked IssueOps worktree from another branch to block source checkout edit, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "linked IssueOps worktree") {
		t.Fatalf("expected linked worktree reason, got %q", reason)
	}

	payload, err = json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path": filepath.Join(worktree, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj = runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected linked IssueOps worktree edit to be allowed, got %+v", obj)
	}
}

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
	if obj["decision"] != "block" {
		t.Fatalf("expected source checkout edit to be blocked, got %+v", obj)
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
	if obj["decision"] != "block" {
		t.Fatalf("expected source checkout CodeGraph projectPath to block from linked worktree cwd, got %+v", obj)
	}

	payload, err = json.Marshal(map[string]any{
		"cwd":       target.path,
		"tool_name": "mcp__codegraph__codegraph_search",
		"tool_input": map[string]any{
			"projectPath": target.path,
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
		t.Fatalf("expected linked worktree CodeGraph projectPath to be allowed from linked worktree cwd, got %+v", obj)
	}
}
