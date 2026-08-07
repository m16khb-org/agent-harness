package harnessapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestIssueOpsPreparationPlanArtifactFailureStopsBeforeOwnerMutations(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "262-plan-readiness"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/262"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	orca := &planReadinessOrcaFake{}
	remoteReads := 0
	operationIDs := 0
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Orca: orca,
		ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			remoteReads++
			return port.ExecutionIssueSnapshot{}, nil
		},
		NewOperationID: func() (string, error) {
			operationIDs++
			return "must-not-be-created", nil
		},
	})

	result, err := handler(context.Background(), stateRoot, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", Actor: claimWiringActor(t), CWD: repo,
		OwnerHost: "codex", Confirm: true,
	}, issueops.ExecutionPrepareInvocation{})
	if err == nil || result.OK {
		t.Fatalf("result=%#v err=%v, want plan readiness failure", result, err)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "orca_plan_artifact_required" {
		t.Fatalf("error=%T %v want orca_plan_artifact_required", err, err)
	}
	if orca.probeCalls != 1 || orca.inspectCalls != 0 || orca.invokeCalls != 0 {
		t.Fatalf("Orca probe=%d inspect=%d invoke=%d, want 1/0/0", orca.probeCalls, orca.inspectCalls, orca.invokeCalls)
	}
	if remoteReads != 0 || operationIDs != 0 {
		t.Fatalf("remote reads=%d operation IDs=%d, want 0/0", remoteReads, operationIDs)
	}
	persisted, readErr := issueops.ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution != nil || persisted.WorktreePath != "" {
		t.Fatalf("readiness failure persisted begin intent or lease: %#v", persisted)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", record.Branch)
	if _, statErr := os.Lstat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("readiness failure materialized workspace %q: %v", worktree, statErr)
	}
}

type planReadinessOrcaFake struct {
	probeCalls   int
	inspectCalls int
	invokeCalls  int
}

func (fake *planReadinessOrcaFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	fake.probeCalls++
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true}, nil
}

func (fake *planReadinessOrcaFake) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	fake.inspectCalls++
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (fake *planReadinessOrcaFake) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invokeCalls++
	return port.ExecutionOrcaIntentReceipt{}, nil
}

func TestIssueOpsPrepareWiringRunsRealDirectPreviewWithoutPersistence(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "config", "user.name", "IssueOps Test")
	claimWiringGit(t, repo, "config", "user.email", "issueops@example.invalid")
	claimWiringGit(t, repo, "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-wiring"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	direct := &preparationWiringDirectFake{}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Direct: direct, Now: func() time.Time { return time.Date(2026, 8, 2, 4, 5, 6, 7, time.UTC) },
		NewOperationID: func() (string, error) { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil },
	})
	process := &issueopscontract.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T00:00:00Z", Executable: "/usr/local/bin/codex"}

	result, err := handler(context.Background(), stateRoot, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", Actor: issueopscontract.NativeActor{Host: "codex", SessionID: "session", SessionProcess: process},
		CWD: repo, Confirm: false,
	}, issueops.ExecutionPrepareInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Preview || result.ResolvedMode != "direct" || direct.calls != 1 {
		t.Fatalf("result=%#v direct calls=%d", result, direct.calls)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution != nil || persisted.WorktreePath != "" {
		t.Fatalf("preview persisted execution: %#v", persisted)
	}
}

func TestIssueOpsPrepareWiringUsesRequestScopedIssueSnapshot(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "gitlab", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte("# Request-scoped snapshot plan\n")); err != nil {
		t.Fatal(err)
	}
	fallbackCalls := 0
	fallback := func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		fallbackCalls++
		return port.ExecutionIssueSnapshot{}, context.DeadlineExceeded
	}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Orca: &reconcileProvisionerFake{}, ReadIssue: fallback,
	})

	raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionPrepare, ID: record.ID, Mode: "orca",
		Actor: claimWiringActor(t), CWD: repo, OwnerHost: "codex", Confirm: true,
		IssueSnapshot: &port.ExecutionIssueSnapshotEvidence{
			Provider: "gitlab", Source: "glab_mcp",
			WebURL: "https://gitlab.example.com/acme/repo/-/issues/199",
			Body:   claimWiringIssueBody(), State: "opened",
		},
	}, issueops.ExecutionActionDependencies{Prepare: handler, ReadIssue: fallback})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(issueops.ExecutionPrepareResult)
	if !ok || !result.OK || result.ResolvedMode != "orca" || result.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("result=%#v", raw)
	}
	if fallbackCalls != 0 {
		t.Fatalf("validated request snapshot called provider fallback %d times", fallbackCalls)
	}
}

type preparationWiringDirectFake struct{ calls int }

func (fake *preparationWiringDirectFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	fake.calls++
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git",
	}, nil
}
