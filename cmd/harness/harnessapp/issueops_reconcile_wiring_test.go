package harnessapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

type reconcileProvisionerFake struct {
	inspectCalls int
	invokeCalls  int
	adopt        bool
	failStage    port.ExecutionOrcaIntentStage
}

func (*reconcileProvisionerFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true}, nil
}

func (f *reconcileProvisionerFake) InspectIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	f.inspectCalls++
	if f.adopt {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{reconcileSuccessfulReceipt(request)}}, nil
	}
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (f *reconcileProvisionerFake) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	f.invokeCalls++
	if request.Stage == port.ExecutionOrcaIntentWorktree {
		if err := os.MkdirAll(request.Workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		receipt := port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{SourceRoot: request.Workspace.SourceRoot, Root: request.Workspace.Root, Branch: request.Workspace.Branch, BaseHead: request.Workspace.BaseHead, ParentWorktree: request.Workspace.ParentWorktree, Driver: "orca", Exists: true},
			RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
		}}
		if f.failStage == request.Stage {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
		}
		return receipt, nil
	}
	if request.Stage == f.failStage {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
	}
	return reconcileSuccessfulReceipt(request), nil
}

func TestIssueOpsReconcileVerticalAdoptsExactlyOneStage(t *testing.T) {
	stateRoot, record, fake := reconcilePendingFixture(t, port.ExecutionOrcaIntentTerminal)
	fake.adopt = true
	fake.inspectCalls = 0
	fake.invokeCalls = 0
	raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReconcile, ID: record.ID, Confirm: true,
		Actor: claimWiringActor(t), CWD: record.Execution.Workspace.SourceRoot,
	}, issueops.ExecutionActionDependencies{
		Orca: fake, Reconcile: issueOpsReconcileHandler, RemoteReconcile: issueOpsPublicationReconcileHandler,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(issueops.ExecutionReconcileResult)
	if !ok {
		t.Fatalf("result type=%T", raw)
	}
	if !result.OK || !result.Reconciled || !result.ExternalStateInspected || result.Code != "orca_reconcile_advanced_run_create" {
		t.Fatalf("result=%#v", result)
	}
	if fake.inspectCalls != 1 || fake.invokeCalls != 0 || result.Pending == nil || result.Pending.Kind != "owner_launch" {
		t.Fatalf("inspect=%d invoke=%d pending=%#v", fake.inspectCalls, fake.invokeCalls, result.Pending)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Pending == nil || persisted.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("persisted execution=%#v", persisted.Execution)
	}
}

func TestIssueOpsReconcileVerticalUsesRequestScopedReaderForWorktreeReceipt(t *testing.T) {
	stateRoot, record, fake := reconcilePendingFixture(t, port.ExecutionOrcaIntentWorktree)
	fake.adopt = true
	fake.inspectCalls = 0
	fake.invokeCalls = 0
	reads := 0
	raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReconcile, ID: record.ID, Confirm: true,
		Actor: claimWiringActor(t), CWD: record.Execution.Workspace.SourceRoot,
	}, issueops.ExecutionActionDependencies{
		Orca: fake, Reconcile: issueOpsReconcileHandler,
		ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			reads++
			return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := raw.(issueops.ExecutionReconcileResult)
	if result.Code != "orca_reconcile_advanced_terminal_create" || reads != 1 || fake.inspectCalls != 1 || fake.invokeCalls != 0 {
		t.Fatalf("result=%#v reads=%d inspect=%d invoke=%d", result, reads, fake.inspectCalls, fake.invokeCalls)
	}
}

func TestIssueOpsReconcileVerticalAdvancesRemainingStagesOneCallAtATime(t *testing.T) {
	stateRoot, record, fake := reconcilePendingFixture(t, port.ExecutionOrcaIntentTerminal)
	fake.adopt = true
	wantCodes := []string{
		"orca_reconcile_advanced_run_create",
		"orca_reconcile_advanced_run_bind",
		"orca_reconcile_advanced_task_create",
		"orca_reconcile_advanced_dispatch",
		"orca_reconcile_completed",
	}
	for index, wantCode := range wantCodes {
		beforeInspects := fake.inspectCalls
		raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
			Action: issueops.ExecutionActionReconcile, ID: record.ID, Confirm: true,
			Actor: claimWiringActor(t), CWD: record.Execution.Workspace.SourceRoot,
		}, issueops.ExecutionActionDependencies{Orca: fake, Reconcile: issueOpsReconcileHandler})
		if err != nil {
			t.Fatalf("stage %d: %v", index, err)
		}
		result := raw.(issueops.ExecutionReconcileResult)
		if result.Code != wantCode || fake.inspectCalls != beforeInspects+1 {
			t.Fatalf("stage %d result=%#v inspect=%d", index, result, fake.inspectCalls-beforeInspects)
		}
	}
}

func TestIssueOpsReconcileVerticalDoesNotClaimInspectionWithoutProvisioner(t *testing.T) {
	stateRoot, record, _ := reconcilePendingFixture(t, port.ExecutionOrcaIntentTerminal)
	raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReconcile, ID: record.ID, Confirm: true,
		Actor: claimWiringActor(t), CWD: record.Execution.Workspace.SourceRoot,
	}, issueops.ExecutionActionDependencies{Reconcile: issueOpsReconcileHandler})
	if err == nil {
		t.Fatal("missing provisioner must fail")
	}
	result := raw.(issueops.ExecutionReconcileResult)
	if result.Code != "orca_reconcile_ambiguous" || result.ExternalStateInspected {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestIssueOpsReconcileVerticalUsesInjectedClockForFailureReceipt(t *testing.T) {
	stateRoot, record, fake := reconcilePendingFixture(t, port.ExecutionOrcaIntentTerminal)
	want := time.Date(2026, 8, 1, 11, 12, 13, 14, time.UTC)
	_, err := issueops.ReconcileExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: claimWiringActor(t), CWD: record.Execution.Workspace.SourceRoot,
	}, issueops.ExecutionReconcileDependencies{
		Orca: fake, Handler: issueOpsReconcileHandler, Now: func() time.Time { return want },
	})
	if err == nil {
		t.Fatal("ambiguous invocation must retain the intent")
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Failure == nil || persisted.Execution.Failure.At != want.Format(time.RFC3339Nano) {
		t.Fatalf("failure=%#v", persisted.Execution.Failure)
	}
}

func reconcilePendingFixture(t *testing.T, failStage port.ExecutionOrcaIntentStage) (string, issueopscontract.IssueOpsRecord, *reconcileProvisionerFake) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "source")
	claimWiringGit(t, "", "init", "-q", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# reconcile fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimWiringGit(t, repo, "add", "README.md")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: reconcile fixture")
	baseHead := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	const branch = "194-reconcile"
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsCurrentSchemaVersion, ID: issueops.NewIssueOpsID(repo, branch),
		Repo: repo, Branch: branch, Phase: issueops.IssueOpsPhasePlan, IssueURL: "https://github.com/acme/repo/issues/194",
		DesignReview:  &issueopscontract.IssueOpsDesignReview{Approved: true, ReviewedAt: "2026-08-01T00:00:00Z"},
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/194", Branch: branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true, CreatedAt: "2026-08-01T00:00:00Z"},
		CreatedAt:     "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z",
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte("# Reconcile plan\n")); err != nil {
		t.Fatal(err)
	}
	fake := &reconcileProvisionerFake{failStage: failStage}
	prepare := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Orca: fake, ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
		},
	})
	_, err := prepare(context.Background(), stateRoot, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", Actor: claimWiringActor(t), CWD: repo, OwnerHost: "codex", OwnerModel: "model", Confirm: true,
	}, issueops.ExecutionPrepareInvocation{})
	if err == nil {
		t.Fatal("fixture must stop on an ambiguous terminal mutation")
	}
	pending, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantKind := "owner_launch"
	if failStage == port.ExecutionOrcaIntentWorktree {
		wantKind = "worktree_create"
	}
	if pending.Execution == nil || pending.Execution.Pending == nil || pending.Execution.Pending.Kind != wantKind || pending.Execution.Mode != issueopscontract.ExecutionModeOrca {
		t.Fatalf("pending fixture=%#v", pending.Execution)
	}
	return stateRoot, pending, fake
}

func reconcileSuccessfulReceipt(request port.ExecutionOrcaIntentRequest) port.ExecutionOrcaIntentReceipt {
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		return port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{SourceRoot: request.Workspace.SourceRoot, Root: request.Workspace.Root, Branch: request.Workspace.Branch, BaseHead: request.Workspace.BaseHead, ParentWorktree: request.Workspace.ParentWorktree, Driver: "orca", Exists: true},
			RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
		}}
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-reconciled", TerminalHandle: "transient"}
	case port.ExecutionOrcaIntentRun:
		return port.ExecutionOrcaIntentReceipt{RunID: "run-reconciled"}
	case port.ExecutionOrcaIntentRunBind:
		return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-reconciled"}
	case port.ExecutionOrcaIntentDispatch:
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-reconciled"}
	default:
		return port.ExecutionOrcaIntentReceipt{}
	}
}

func TestReconcileReceiptRoundTripPreservesWorktreeIdentity(t *testing.T) {
	want := port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{SourceRoot: "/source", Root: "/worktree", Branch: "194", BaseHead: "abc", ParentWorktree: "/parent", Driver: "orca", Exists: true},
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
	}}
	contractReceipt, err := reconcileContractReceipt(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reconcilePortReceipt(contractReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace == nil || *got.Workspace != *want.Workspace {
		t.Fatalf("receipt=%#v", got)
	}
}
