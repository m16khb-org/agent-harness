package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	writeHookFixtureFile(t, worktree, "internal/core/issueops.go", "package core\n")
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
	if obj["decision"] != "allow" {
		t.Fatalf("expected linked IssueOps worktree guard to allow source checkout edit, got %+v", obj)
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
	if obj["decision"] != "allow" {
		t.Fatalf("expected source checkout edit to be allowed when no active cycle on current branch (cycle-scoped guard), got %+v", obj)
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

func TestRunHookPreToolUseAsksForSessionBoundMirrorFileEdit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "2519-test-quality-comprehensive")
	activateIssueOpsHookExecutionV1(t, cycle.id)
	writeHookFixtureFile(t, cycle.path, "src/a.ts", "export const a = 1;\n")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "apply_patch",
		"tool_input": map[string]any{
			"file_path": filepath.Join(source, "src", "a.ts"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected source mutation to be blocked outside the active lease worktree, got %+v", obj)
	}
	assertIssueOpsV1DenyFields(t, obj["deny"], cycle.id, cycle.path, 1, "write_lease_required")

	claude := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-worktree"})
	})
	hso, exists := claude["hookSpecificOutput"].(map[string]any)
	if !exists {
		t.Fatalf("Claude block must emit a native decision, got %+v", claude)
	}
	assertIssueOpsV1DenyJSON(t, hso["permissionDecisionReason"], cycle.id, cycle.path, 1, "write_lease_required")

	codex := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree"})
	})
	if codex["decision"] != "block" {
		t.Fatalf("Codex block must emit a native decision, got %+v", codex)
	}
	assertIssueOpsV1DenyJSON(t, codex["reason"], cycle.id, cycle.path, 1, "write_lease_required")
}

func assertIssueOpsV1DenyJSON(t *testing.T, raw any, id, root string, generation int, code string) {
	t.Helper()
	encoded, ok := raw.(string)
	if !ok {
		t.Fatalf("structured deny reason must be a JSON string, got %#v", raw)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(encoded), &fields); err != nil {
		t.Fatalf("structured deny reason is not JSON: %q: %v", encoded, err)
	}
	assertIssueOpsV1DenyFields(t, fields, id, root, generation, code)
}

func assertIssueOpsV1DenyFields(t *testing.T, raw any, id, root string, generation int, code string) {
	t.Helper()
	fields, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("structured deny fields missing: %#v", raw)
	}
	if len(fields) != 5 || fields["code"] != code || fields["lifecycle_id"] != id || fields["expected_root"] != root ||
		fields["current_generation"] != float64(generation) || !strings.Contains(fields["next_command"].(string), "--id "+id) {
		t.Fatalf("unexpected structured deny fields: %+v", fields)
	}
}

func TestRunHookPreToolUseBindsLeaseHolderToLocalProcessAncestry(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "69-process-bound")
	actor := activateIssueOpsHookExecutionV1(t, cycle.id)
	path := filepath.Join(cycle.path, "src", "holder.go")
	payload, err := json.Marshal(map[string]any{
		"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "apply_patch", "tool_input": map[string]any{"file_path": path},
	})
	if err != nil {
		t.Fatal(err)
	}
	allowed := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if allowed["decision"] != "allow" {
		t.Fatalf("locally observed holder process ancestry must allow canonical mutation, got %+v", allowed)
	}

	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), cycle.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.Holder.SessionProcess.StartedAt = "1970-01-01T00:00:00Z"
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	blocked := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if blocked["decision"] != "block" {
		t.Fatalf("same session payload with a mismatched process receipt must be blocked, got %+v", blocked)
	}
}

func TestRunHookPreToolUseBlocksFilesystemAliasTargetsFromUnrelatedCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "69-filesystem-alias")
	activateIssueOpsHookExecutionV1(t, cycle.id)
	sibling := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "foreign-alias")
	gitDir := filepath.Join(source, ".git", "worktrees", "foreign-alias")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unrelated := t.TempDir()
	for name, root := range map[string]string{"source": source, "canonical": cycle.path, "sibling": sibling} {
		t.Run(name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"cwd": unrelated, "tool_name": "mcp__filesystem__move_file",
				"tool_input": map[string]any{
					"source": filepath.Join(root, "before.txt"), "destination": filepath.Join(root, "after.txt"),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
			})
			if got["decision"] != "block" {
				t.Fatalf("filesystem alias target in %s escaped authority selection: %+v", name, got)
			}
		})
	}
}
