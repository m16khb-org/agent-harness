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

func TestDelegatedLinkWorktreeBindsScopedSessionWithoutClobberingPrimary(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	parent := createDelegationReadyParentForTest(t, stateRoot)
	primary, err := ReadIssueOpsSession(parent.Repo)
	if err != nil {
		t.Fatalf("parent link-worktree must bind primary session: %v", err)
	}
	if primary.CycleID != parent.ID {
		t.Fatalf("expected parent primary binding, got %+v", primary)
	}

	started, err := StartIssueOpsChild(stateRoot, IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "124-child-session",
		Title:              "child session binding",
		TaskScope:          "scoped session binding",
		AcceptanceCriteria: []string{"child worktree link binds scoped session"},
		ParentPlanPath:     parent.PlanPath,
		ChildIssueURL:      "https://github.com/example/repo/issues/124",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := LinkIssueOpsIssue(stateRoot, started.Child.ID, "https://github.com/example/repo/issues/124")
	if err != nil {
		t.Fatal(err)
	}
	child, err = PrepareIssueOpsBranch(stateRoot, child.ID, IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: child.IssueURL, Branch: child.Branch, BaseBranch: parent.Branch, LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	childWorktree := makeIssueOpsWorktreeDirForTest(t, parent.Repo, started.Child.Branch)
	child, err = LinkIssueOpsWorktree(stateRoot, child.ID, childWorktree)
	if err != nil {
		t.Fatal(err)
	}

	afterPrimary, err := ReadIssueOpsSession(parent.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if afterPrimary.CycleID != parent.ID || afterPrimary.ExpectedWorktree != primary.ExpectedWorktree {
		t.Fatalf("delegated link-worktree clobbered primary binding: got %+v want %+v", afterPrimary, primary)
	}
	scoped, err := ReadScopedIssueOpsSession(parent.Repo, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.CycleID != child.ID || scoped.ExpectedWorktree != childWorktree {
		t.Fatalf("expected child scoped binding, got %+v", scoped)
	}

	if _, err := ForceReleaseIssueOps(stateRoot, child.ID, "test child scoped cleanup"); err != nil {
		t.Fatal(err)
	}
	if scoped, err := ReadScopedIssueOpsSession(parent.Repo, child.ID); err != nil {
		t.Fatal(err)
	} else if scoped.CycleID != "" {
		t.Fatalf("child force release must unbind child scoped session, got %+v", scoped)
	}
	if afterRelease, err := ReadIssueOpsSession(parent.Repo); err != nil {
		t.Fatal(err)
	} else if afterRelease.CycleID != parent.ID {
		t.Fatalf("child force release must not unbind parent primary session, got %+v", afterRelease)
	}
}
