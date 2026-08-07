package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/contract/issueops"
)

func TestIssueOpsAISlopCleanRejectsUntrackedPlanWithoutImplementation(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "99-plan-only"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/99")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/new-demo.md", "plan\n")
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans", "new-demo.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean)); err == nil || !strings.Contains(err.Error(), "implementation_changes") {
		t.Fatalf("ai-slop-clean should reject plan-only untracked directories, got %v", err)
	}
}
