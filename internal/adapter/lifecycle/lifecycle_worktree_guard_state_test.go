package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestActiveIssueOpsCycleForBranchIsDeterministicAndReleasesOnDone(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "1-main"); ok {
		t.Fatalf("no cycle yet")
	}
	first, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "1-main"})
	if err != nil {
		t.Fatal(err)
	}
	// Re-starting the same (repo, branch) must resume the same record, not duplicate.
	second, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "1-main"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("start must be idempotent per (repo, branch): %s != %s", first.ID, second.ID)
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "1-main"); !ok {
		t.Fatalf("active cycle should be found")
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "other"); ok {
		t.Fatalf("a different branch must not match")
	}
	markIssueOpsPRPhaseForTest(t, repo, "1-main")
	record, err := ReadIssueOps(IssueOpsStateRoot(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = IssueOpsPhaseDone
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "1-main"); ok {
		t.Fatalf("done cycle must not be reported active")
	}
}

func TestGitBranchFromHeadResolvesRelativeLinkedWorktreeGitdir(t *testing.T) {
	base := t.TempDir()
	// Simulate a linked worktree: <base>/wt/.git is a file pointing to a relative
	// gitdir, and HEAD lives under that resolved gitdir.
	wt := filepath.Join(base, "repo.worktrees", "feat-x")
	gitdir := filepath.Join(base, "repo", ".git", "worktrees", "feat-x")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wt, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/1-x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := gitBranchFromHead(wt); got != "1-x" {
		t.Fatalf("expected branch 1-x from relative linked-worktree gitdir, got %q", got)
	}
}
