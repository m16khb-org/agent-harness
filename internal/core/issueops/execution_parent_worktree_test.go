package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestPrepareExecutionAllowsDelegatedChildFromSealedParentWorktree(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	record.BranchPrepare.BaseBranch = "117-umbrella"
	record.Delegation = &IssueOpsDelegationContract{ParentCycleID: "io-parent"}
	record, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}

	parentWorktree := filepath.Join(record.Repo+".worktrees", record.BranchPrepare.BaseBranch)
	if err := os.MkdirAll(parentWorktree, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if workspace.ParentWorktree != parentWorktree {
			t.Fatalf("parent worktree = %q, want %q", workspace.ParentWorktree, parentWorktree)
		}
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		receipt := executionOrcaWorkspaceReceipt(workspace)
		receipt.Workspace.ParentWorktree = workspace.ParentWorktree
		return receipt, nil
	}

	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: parentWorktree, Confirm: true,
		Actor: executionActor("codex", "parent-worktree-session"), OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution == nil || fake.prepareCalls != 1 {
		t.Fatalf("sealed parent worktree에서 Orca 준비가 완료되지 않았다: result=%#v calls=%d", got, fake.prepareCalls)
	}
}

func TestPrepareExecutionRejectsForeignCWDForDelegatedChild(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	record.BranchPrepare.BaseBranch = "117-umbrella"
	record.Delegation = &IssueOpsDelegationContract{ParentCycleID: "io-parent"}
	record, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	fake := readyOrcaFake()

	_, err = PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: t.TempDir(), Confirm: true,
		Actor: executionActor("codex", "foreign-worktree-session"), OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err == nil || !strings.Contains(err.Error(), "sealed parent worktree") {
		t.Fatalf("임의의 제3 경로가 Orca 준비에 허용됐다: %v", err)
	}
	if fake.prepareCalls != 0 {
		t.Fatalf("CWD 검증 실패 뒤 Orca mutation이 실행됐다: calls=%d", fake.prepareCalls)
	}
}

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
