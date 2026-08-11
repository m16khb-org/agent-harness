package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	contractissueops "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

type rolloverOwnerInspector struct {
	inventory port.ExecutionOrcaOwnerInventory
	requests  []port.ExecutionOrcaOwnerInventoryRequest
}

func (inspector *rolloverOwnerInspector) InspectOwner(_ context.Context, request port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	inspector.requests = append(inspector.requests, request)
	if inspector.inventory.RuntimeID != "" && inspector.inventory.RuntimeID != "runtime-sealed" && !request.AllowRuntimeRollover {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("changed runtime inventory requested without rollover authority")
	}
	return inspector.inventory, nil
}

func TestExecutionReplacementRecoversDeadOwnerAfterOrcaRuntimeRollover(t *testing.T) {
	stateRoot, record := rolloverExecutionFixture(t)
	requester := executionActor("codex", "replacement-owner")
	inspector := &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
		RuntimeID: "runtime-current", TaskLive: true,
		TaskStatus: "dispatched", DispatchStatus: "dispatched",
	}}
	dependencies := ExecutionReplaceDependencies{
		OrcaOwner: inspector,
		ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			return port.ExecutionIssueSnapshot{
				URL:   request.URL,
				Body:  "## 완료 기준\n\n- AC-01\n- AC-02\n- AC-03\n\n## 검증\n\n```bash\ngo test ./internal/core/issueops -count=1\n```",
				State: "open", Source: "test",
			}, nil
		},
	}

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatalf("dead owner rollover preview: %v", err)
	}
	if preview.InventoryFingerprint == "" {
		t.Fatal("dead owner rollover preview did not return an inventory fingerprint")
	}
	if len(inspector.requests) != 1 || !inspector.requests[0].AllowRuntimeRollover {
		t.Fatalf("dead owner did not enable bounded runtime rollover inventory: %#v", inspector.requests)
	}

	revoked, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "old Orca runtime rolled over",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies)
	if err != nil {
		t.Fatalf("revoke dead owner rollover lease: %v", err)
	}
	if revoked.Execution.Lease.Generation != 2 || revoked.Execution.Lease.Status != contractissueops.LeaseStatusRevoking {
		t.Fatalf("revoke rotated the lease incorrectly: %#v", revoked.Execution.Lease)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "duplicate revoke",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies); err == nil || !strings.Contains(err.Error(), "stale lease generation") {
		t.Fatalf("duplicate revoke did not fail generation CAS: %v", err)
	}

	finalizePreview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatalf("preview dead owner rollover finalization: %v", err)
	}
	if finalizePreview.QuiescenceFingerprint == "" {
		t.Fatal("dead owner rollover finalization did not return a quiescence fingerprint")
	}

	finalized, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: finalizePreview.QuiescenceFingerprint,
		Actor:                 requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies)
	if err != nil {
		t.Fatalf("finalize dead owner rollover lease: %v", err)
	}
	if finalized.Execution.Lease.Generation != 2 || finalized.Execution.Lease.Status != contractissueops.LeaseStatusClaimable || finalized.Execution.Lease.Holder != nil {
		t.Fatalf("finalized rollover lease is not claimable: %#v", finalized.Execution.Lease)
	}
	if finalized.Execution.Orca.LeaseGeneration != finalized.Execution.Lease.Generation ||
		finalized.Execution.Orca.ArtifactIdentityVersion != contractissueops.OrcaArtifactIdentityVersion ||
		finalized.Execution.Orca.IssueBodySHA256 != finalized.IssueBodySHA256 ||
		finalized.Execution.Orca.ContextPacketSHA256 != finalized.ContextPacketSHA256 ||
		finalized.Execution.Orca.OwnerPromptSHA256 != finalized.OwnerPromptSHA256 {
		t.Fatalf("finalize did not persist its generation-specific Orca identity: binding=%#v result=%#v", finalized.Execution.Orca, finalized)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Orca.LeaseGeneration != finalized.Execution.Lease.Generation ||
		persisted.Execution.Orca.ArtifactIdentityVersion != contractissueops.OrcaArtifactIdentityVersion ||
		persisted.Execution.Orca.IssueBodySHA256 != finalized.IssueBodySHA256 ||
		persisted.Execution.Orca.ContextPacketSHA256 != finalized.ContextPacketSHA256 ||
		persisted.Execution.Orca.OwnerPromptSHA256 != finalized.OwnerPromptSHA256 {
		t.Fatalf("durable finalize identity=%#v result=%#v", persisted.Execution.Orca, finalized)
	}
}

func TestExecutionReplacementRuntimeRolloverSafetyBoundaries(t *testing.T) {
	t.Run("live owner", func(t *testing.T) {
		stateRoot, record := rolloverExecutionFixture(t)
		liveOwner := executionActor("codex", "live-owner")
		record.Execution.Lease.Holder = &liveOwner
		if _, err := writeIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		inspector := &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current"}}

		_, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
			ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
			Actor: executionActor("codex", "replacement-owner"), CWD: record.Execution.Workspace.Root,
		}, ExecutionReplaceDependencies{OrcaOwner: inspector})
		if err == nil {
			t.Fatal("live owner was allowed to preview runtime rollover replacement")
		}
		if len(inspector.requests) != 1 || inspector.requests[0].AllowRuntimeRollover {
			t.Fatalf("live owner requested rollover inventory: %#v", inspector.requests)
		}
	})

	t.Run("matching ghost terminal", func(t *testing.T) {
		stateRoot, record := rolloverExecutionFixture(t)
		inspector := &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
			RuntimeID: "runtime-current", TerminalID: "pty-old", TerminalLive: false,
			TaskStatus: "completed", DispatchStatus: "completed",
		}}

		_, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
			ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
			Actor: executionActor("codex", "replacement-owner"), CWD: record.Execution.Workspace.Root,
		}, ExecutionReplaceDependencies{OrcaOwner: inspector})
		if err == nil {
			t.Fatal("matching ghost terminal was treated as authoritative absence")
		}
	})

	t.Run("same runtime task remains live", func(t *testing.T) {
		stateRoot, record := rolloverExecutionFixture(t)
		inspector := &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
			RuntimeID: "runtime-sealed", TaskLive: true,
			TaskStatus: "dispatched", DispatchStatus: "dispatched",
		}}
		dependencies := ExecutionReplaceDependencies{OrcaOwner: inspector}
		requester := executionActor("codex", "replacement-owner")
		preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
			ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
			Actor: requester, CWD: record.Execution.Workspace.Root,
		}, dependencies)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
			ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
			InventoryFingerprint: preview.InventoryFingerprint, Reason: "same runtime task check",
			Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
		}, dependencies); err != nil {
			t.Fatal(err)
		}

		_, err = ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
			ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
			Actor: requester, CWD: record.Execution.Workspace.Root,
		}, dependencies)
		if err == nil || !strings.Contains(err.Error(), "Orca owner is not quiescent") {
			t.Fatalf("same-runtime task liveness did not block finalization: %v", err)
		}
	})
}

func rolloverExecutionFixture(t *testing.T) (string, contractissueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "270-runtime-rollover")
	record := fixture.record
	record.IssueURL = "https://github.com/example/agent-harness/issues/270"
	record.BranchPrepare.IssueURL = record.IssueURL
	record.Execution.Mode = contractissueops.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &contractissueops.OrcaBinding{
		RuntimeID: "runtime-sealed", RepoID: "repo", WorktreeID: "worktree",
		WorktreeInstanceID: "instance", RunID: "run", TaskID: "task",
		DispatchID: "dispatch", TerminalPTYID: "pty-old", LeaseGeneration: 1,
		OwnerHost: "codex", OwnerModel: "gpt-5.6-sol", OwnerEffort: "high",
	}
	// Orca 실행은 sealed plan readiness를 요구한다. 실제 수명주기에서 plan은 owner가
	// 활성화되기 전 released generation에서 link-plan + artifact stage로 봉인되므로,
	// 픽스처도 durable plan path와 staged artifact를 같은 순서로 갖춘다.
	const planBody = "# Plan\n"
	record.PlanPath = filepath.Join(record.WorktreePath, "plan.md")
	writePlanArtifactTestFile(t, record.PlanPath, planBody)
	record.Execution.Lease = contractissueops.WriteLease{
		Generation: 1, Status: contractissueops.LeaseStatusReleased,
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte(planBody)); err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease = contractissueops.WriteLease{
		Generation: 1, Status: contractissueops.LeaseStatusActive,
		Holder: &contractissueops.NativeActor{
			Host: "codex", SessionID: "dead-owner",
			SessionProcess: &contractissueops.NativeProcessReceipt{
				PID: 999999, StartedAt: "2026-08-03T00:00:00Z", Executable: "/usr/bin/codex",
			},
		},
		ClaimedAt: "2026-08-03T00:00:00Z",
	}
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}
