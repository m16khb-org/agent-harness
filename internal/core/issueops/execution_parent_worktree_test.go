package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestExecutionWorkspaceRequestBindsDelegatedChildToUmbrellaWorktree(t *testing.T) {
	repo := t.TempDir()
	record := IssueOpsRecord{
		ID:     "io-child",
		Repo:   repo,
		Branch: "190-child",
		BranchPrepare: &IssueOpsBranchPrepare{
			BaseBranch: "117-umbrella",
			BaseSHA:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Delegation: &IssueOpsDelegationContract{ParentCycleID: "io-parent"},
	}

	got, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	wantParent := filepath.Join(repo+".worktrees", "117-umbrella")
	if got.ParentWorktree != wantParent {
		t.Fatalf("parent worktree = %q, want %q", got.ParentWorktree, wantParent)
	}

	receipt := port.ExecutionWorkspaceReceipt{
		SourceRoot: got.SourceRoot, Root: got.Root, Branch: got.Branch,
		BaseHead: got.BaseHead, ParentWorktree: got.ParentWorktree, Driver: "orca",
	}
	if persisted := workspaceFromReceipt(receipt, "2026-07-27T00:00:00Z"); persisted.ParentWorktree != wantParent {
		t.Fatalf("persisted parent worktree = %q, want %q", persisted.ParentWorktree, wantParent)
	}
}

func TestExecutionWorkspaceRequestBindsExplicitUmbrellaParentWithoutDelegation(t *testing.T) {
	repo := t.TempDir()
	wantParent := filepath.Join(repo+".worktrees", "117-umbrella")
	record := IssueOpsRecord{
		ID:     "io-provider-child",
		Repo:   repo,
		Branch: "196-provider-child",
		BranchPrepare: &IssueOpsBranchPrepare{
			BaseBranch:     "117-umbrella",
			BaseSHA:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ParentWorktree: wantParent,
		},
	}

	got, err := executionWorkspaceRequest(record, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentWorktree != wantParent {
		t.Fatalf("parent worktree = %q, want %q", got.ParentWorktree, wantParent)
	}
}

func TestExecutionWorkspaceRequestRejectsNonCanonicalExplicitParent(t *testing.T) {
	repo := t.TempDir()
	record := IssueOpsRecord{
		ID:     "io-provider-child",
		Repo:   repo,
		Branch: "196-provider-child",
		BranchPrepare: &IssueOpsBranchPrepare{
			BaseBranch:     "117-umbrella",
			BaseSHA:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ParentWorktree: filepath.Join(repo+".worktrees", "wrong-parent"),
		},
	}

	_, err := executionWorkspaceRequest(record, false)
	if err == nil || !strings.Contains(err.Error(), "canonical parent worktree") {
		t.Fatalf("non-canonical explicit parent must fail closed: %v", err)
	}
}

func TestExecutionWorkspaceRequestKeepsIndependentWorktreeTopLevel(t *testing.T) {
	record := IssueOpsRecord{
		ID:     "io-independent",
		Repo:   t.TempDir(),
		Branch: "190-independent",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			BaseBranch: "main",
			BaseSHA:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	got, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentWorktree != "" {
		t.Fatalf("independent worktree unexpectedly inherited parent %q", got.ParentWorktree)
	}
}
