package issueops

import (
	"testing"

	"agent-harness/internal/core/preflight"
)

// S4 (P0/P1 closure): the read-side fallbacks (hook guards resolving the
// expected worktree from the session binding) already exist, but nothing
// ever WROTE the binding — link-worktree must bind and done must unbind so
// the fallback survives session restarts with real data.
func TestLinkWorktreeBindsSessionAndDoneUnbinds(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := initIssueOpsRepo(t)
	branch := "99-session-binding"
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

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	if record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/99"); err != nil {
		t.Fatal(err)
	}
	if record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main", LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}

	binding, err := ReadIssueOpsSession(repo)
	if err != nil {
		t.Fatalf("link-worktree must persist the session binding: %v", err)
	}
	if binding.CycleID != record.ID || binding.ExpectedWorktree != worktree {
		t.Fatalf("binding mismatch: got %+v want cycle=%s worktree=%s", binding, record.ID, worktree)
	}

	if _, err := ForceReleaseIssueOps(stateRoot, record.ID, "test closure"); err != nil {
		t.Fatal(err)
	}
	if after, err := ReadIssueOpsSession(repo); err == nil && after.CycleID == record.ID {
		t.Fatalf("cycle release must unbind the session binding, still got %+v", after)
	}
}
