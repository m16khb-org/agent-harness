package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

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
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookIntentForTest(t, record.ID)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
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

func TestRunHookPreToolUseDoesNotFenceUnpreparedLinkedCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsHookIntentForTest(t, record.ID)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
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

func TestRunHookPreToolUseKeepsSourceFileEditIndependentFromCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "2519-test-quality-comprehensive")
	activateIssueOpsHookExecution(t, cycle.id)
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
	if obj["decision"] != "allow" {
		t.Fatalf("source checkout mutation must not be claimed by the active cycle, got %+v", obj)
	}

	claude := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-worktree"})
	})
	if len(claude) != 0 {
		t.Fatalf("Claude source edit allow must remain a no-op hook response, got %+v", claude)
	}

	codex := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree"})
	})
	if len(codex) != 0 {
		t.Fatalf("Codex source edit allow must remain a no-op hook response, got %+v", codex)
	}
	_ = cycle
}

func TestRunHookPreToolUseAllowsHolderCanonicalPatchFromCodexSourceCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "72-cycle-target")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	payload, err := json.Marshal(map[string]any{
		"cwd": source, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "apply_patch",
		"tool_input": map[string]any{
			"file_path": filepath.Join(cycle.path, "internal", "fixed.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree"})
	})
	if len(got) != 0 {
		t.Fatalf("Codex source cwd must not override an explicit canonical target owned by the holder: %+v", got)
	}
}

func assertIssueOpsDenyJSON(t *testing.T, raw any, id, root string, generation int, code string) {
	t.Helper()
	encoded, ok := raw.(string)
	if !ok {
		t.Fatalf("structured deny reason must be a JSON string, got %#v", raw)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(encoded), &fields); err != nil {
		t.Fatalf("structured deny reason is not JSON: %q: %v", encoded, err)
	}
	assertIssueOpsDenyFields(t, fields, id, root, generation, code)
}

func assertIssueOpsDenyFields(t *testing.T, raw any, id, root string, generation int, code string) {
	t.Helper()
	fields, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("structured deny fields missing: %#v", raw)
	}
	if fields["code"] != code || fields["lifecycle_id"] != id || fields["expected_root"] != root ||
		fields["current_generation"] != float64(generation) || !strings.Contains(fields["next_command"].(string), "--id "+id) {
		t.Fatalf("unexpected structured deny fields: %+v", fields)
	}
	// reason은 모든 deny에 실린다(이슈 #154). 구조화된 deny가 host hook 출력에서
	// result.Reason을 대체하므로, 여기에 없으면 사용자에게는 코드만 남고 왜 막혔는지
	// 알 길이 사라진다 — 이 헬퍼가 예전에 세던 5개는 그 상태를 고정하고 있었다.
	if reason, ok := fields["reason"].(string); !ok || strings.TrimSpace(reason) == "" {
		t.Fatalf("deny must carry the blocking reason: %+v", fields)
	}
	// identity 진단 필드는 holder 불일치 deny에서만 나타난다. 진단을 더한다고
	// 아무 필드나 늘리지 않는다는 계약은 그대로다.
	wantIdentityEcho := code == "holder_identity_mismatch"
	_, hasAxis := fields["identity_mismatch"]
	_, hasObserved := fields["observed_actor"]
	if wantLen := 6; wantIdentityEcho {
		wantLen = 8
		if len(fields) != wantLen || !hasAxis || !hasObserved {
			t.Fatalf("identity mismatch deny must echo the observed actor: %+v", fields)
		}
	} else if len(fields) != wantLen || hasAxis || hasObserved {
		t.Fatalf("non-identity deny must stay compact: %+v", fields)
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
	actor := activateIssueOpsHookExecution(t, cycle.id)
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
	activateIssueOpsHookExecution(t, cycle.id)
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
	for name, tc := range map[string]struct {
		root string
		want string
	}{
		"source":    {source, "allow"},
		"canonical": {cycle.path, "block"},
		"sibling":   {sibling, "allow"},
	} {
		t.Run(name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"cwd": unrelated, "tool_name": "mcp__filesystem__move_file",
				"tool_input": map[string]any{
					"source": filepath.Join(tc.root, "before.txt"), "destination": filepath.Join(tc.root, "after.txt"),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
			})
			if got["decision"] != tc.want {
				t.Fatalf("filesystem alias target in %s decision=%v, want %s: %+v", name, got["decision"], tc.want, got)
			}
		})
	}
}
