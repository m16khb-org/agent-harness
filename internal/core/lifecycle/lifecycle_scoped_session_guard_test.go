package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestLifecycleGuardPrefersScopedBindingOnBranchMatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-parent-scoped-guard", IssueOpsPhaseImplement)
	parentID := newIssueOpsID(repo, "1-parent-scoped-guard")
	parentWorktree := makeIssueOpsGuardWorktreeForTest(t, repo, "1-parent-scoped-guard")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-parent-scoped-guard")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), parentID, parentWorktree); err != nil {
		t.Fatal(err)
	}

	child := createLifecycleDelegatedChildForScopedGuard(t, repo, parentID, "1-child-scoped-guard")
	childWorktree := makeIssueOpsGuardWorktreeForTest(t, repo, child.Branch)
	if err := issueops.BindScopedIssueOpsSession(repo, child.ID, child.Branch, childWorktree); err != nil {
		t.Fatal(err)
	}

	writeIssueOpsGuardFileForTest(t, childWorktree, "internal/x.go", "package child\n")
	if err := writeLifecycleRepoHead(t, repo, child.Branch); err != nil {
		t.Fatal(err)
	}
	childBlocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "mcp__filesystem__read_file", EnforceWorktree: true,
	})
	if childBlocked.Decision != "block" || !strings.Contains(childBlocked.Reason, childWorktree) {
		t.Fatalf("child branch should prefer scoped binding worktree, got %+v", childBlocked)
	}

	if err := writeLifecycleRepoHead(t, repo, "1-parent-scoped-guard"); err != nil {
		t.Fatal(err)
	}
	parentBlocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "mcp__filesystem__read_file", EnforceWorktree: true,
	})
	if parentBlocked.Decision != "block" || !strings.Contains(parentBlocked.Reason, parentWorktree) {
		t.Fatalf("parent branch should use primary binding worktree, got %+v", parentBlocked)
	}
}

func TestLifecycleGuardEnvBeatsScopedBinding(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-parent-env-guard", IssueOpsPhaseImplement)
	parentID := newIssueOpsID(repo, "1-parent-env-guard")
	child := createLifecycleDelegatedChildForScopedGuard(t, repo, parentID, "1-child-env-guard")
	childWorktree := makeIssueOpsGuardWorktreeForTest(t, repo, child.Branch)
	envWorktree := makeIssueOpsGuardWorktreeForTest(t, repo, "125-env-wins")
	if err := issueops.BindScopedIssueOpsSession(repo, child.ID, child.Branch, childWorktree); err != nil {
		t.Fatal(err)
	}
	if err := writeLifecycleRepoHead(t, repo, child.Branch); err != nil {
		t.Fatal(err)
	}

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "mcp__filesystem__read_file", ExpectedWorktree: envWorktree, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, envWorktree) {
		t.Fatalf("ExpectedWorktree env should beat scoped binding, got %+v", blocked)
	}
}

func createLifecycleDelegatedChildForScopedGuard(t *testing.T, repo, parentID, branch string) issueops.IssueOpsRecord {
	t.Helper()
	child, err := issueops.StartIssueOps(IssueOpsStateRoot(), issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	child.Delegation = &issueops.IssueOpsDelegationContract{
		ParentCycleID:      parentID,
		TaskScope:          "scoped lifecycle guard",
		AcceptanceCriteria: []string{"guard chooses scoped binding"},
		DelegatedAt:        "2026-07-07T00:00:00Z",
	}
	child.Phase = IssueOpsPhasePlan
	child, err = writeIssueOps(IssueOpsStateRoot(), child)
	if err != nil {
		t.Fatal(err)
	}
	return child
}

func writeLifecycleRepoHead(t *testing.T, repo, branch string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644)
}
