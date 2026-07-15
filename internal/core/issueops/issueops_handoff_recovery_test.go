package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestHandoffCancelWritesTombstoneBeforeReleasingExternalLease(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	updated, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != handoff.StateRecoveryRequired || updated.Disposition != "" || persisted.ExecutionHandoff.Cancellation == nil || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "cancellation_requested" {
		t.Fatalf("cancel released an externally provisioned lease before quiescence: result=%#v handoff=%#v", updated, persisted.ExecutionHandoff)
	}
}

func TestHandoffCancelRejectsClaimedWithoutStaleEvidence(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err == nil {
		t.Fatal("claimed handoff cancel must require explicit stale or force evidence")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateClaimed || persisted.ExecutionHandoff.ClosedDisposition != "" {
		t.Fatalf("rejected claimed cancel mutated the handoff: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffCancelRejectsOversizedReasonWithoutMutation(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(record)); err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: strings.Repeat("x", 4097),
	}, nil, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "cancellation reason exceeds 4096 bytes") {
		t.Fatalf("oversized cancellation reason error = %v", err)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("rejected oversized cancellation reason mutated the lease")
	}
}

func TestHandoffCancelTombstonePreservesAmbiguousOperation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.ClosedDisposition != "" || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.Cancellation == nil || persisted.ExecutionHandoff.Failure.Code != "cancellation_requested" {
		t.Fatalf("cancel tombstone did not preserve the ambiguous journal: %#v", persisted.ExecutionHandoff)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "must-not-be-used", nil
		},
	}); err == nil {
		t.Fatal("retry must reject a cancelled attempt with an unresolved journal")
	}
}

func TestHandoffForcedClaimedCancelPersistsEvidenceAndFencesWorker(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "worker heartbeat lost after coordinator investigation",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.ClosedDisposition != "" || persisted.ExecutionHandoff.Cancellation == nil || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "cancellation_requested" || persisted.ExecutionHandoff.Failure.Message == "" {
		t.Fatalf("forced cancellation tombstone not persisted: %#v", persisted.ExecutionHandoff)
	}
	if _, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, IssueOpsHeartbeatRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID,
	}); err == nil {
		t.Fatal("cancellation tombstone must fence worker heartbeat")
	}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}); err == nil {
		t.Fatal("cancellation tombstone must fence worker finish")
	}
}

func TestHandoffSubmittedTerminalProjectionCanForceCancelAndFinalize(t *testing.T) {
	stateRoot, record, _, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	projector := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-terminal", Sequence: 101}}
	submitted, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, projector)
	if err != nil {
		t.Fatal(err)
	}
	if submitted.ExecutionHandoff.WorkerDoneProjection == nil || submitted.ExecutionHandoff.WorkerDoneProjection.State != "sent" {
		t.Fatalf("terminal projection was not persisted: %#v", submitted.ExecutionHandoff.WorkerDoneProjection)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "coordinator rejected the submitted result",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatalf("force-cancel submitted terminal projection: %v", err)
	}
	finalizeCancelledHandoffForTest(t, stateRoot, record.ID)
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateClosed || persisted.ExecutionHandoff.ClosedDisposition != handoff.DispositionCancelled || persisted.ExecutionHandoff.WorkerDoneProjection == nil || persisted.ExecutionHandoff.WorkerDoneProjection.MessageID != "msg-terminal" {
		t.Fatalf("submitted projection cancellation lost terminal evidence: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffCancelClosesOnlyTrulyPreMutationPreparation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca = &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo"}
	record.WorktreePath = ""
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled {
		t.Fatalf("pre-mutation preparation did not close directly: %#v", got)
	}
}

func TestHandoffCancelTombstonesEveryProvisionedState(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (string, IssueOpsRecord)
		force bool
	}{
		{name: "coordinator preparing", setup: func(t *testing.T) (string, IssueOpsRecord) { return handoffDispatchRecord(t) }},
		{name: "dispatched", setup: func(t *testing.T) (string, IssueOpsRecord) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			return stateRoot, record
		}},
		{name: "claimed", force: true, setup: func(t *testing.T) (string, IssueOpsRecord) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			claimed, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(record))
			if err != nil {
				t.Fatal(err)
			}
			return stateRoot, claimed
		}},
		{name: "submitted", force: true, setup: func(t *testing.T) (string, IssueOpsRecord) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			record.ExecutionHandoff.State = handoff.StateSubmitted
			record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "worker-1"}
			record.ExecutionHandoff.Result = validCompletedHandoffResultForTest(record)
			persisted, err := WriteIssueOps(stateRoot, record)
			if err != nil {
				t.Fatal(err)
			}
			return stateRoot, persisted
		}},
		{name: "ambiguous recovery", setup: func(t *testing.T) (string, IssueOpsRecord) {
			stateRoot, record := handoffDispatchRecord(t)
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"})
			persisted, err := WriteIssueOps(stateRoot, record)
			if err != nil {
				t.Fatal(err)
			}
			return stateRoot, persisted
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := tt.setup(t)
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "cancel", Confirm: true, Force: tt.force, Reason: "verified cancellation request",
			}, nil, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != handoff.StateRecoveryRequired || got.Disposition != "" || persisted.ExecutionHandoff.Cancellation == nil || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "cancellation_requested" {
				t.Fatalf("provisioned %s lease was released before quiescence: result=%#v handoff=%#v", tt.name, got, persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffFinalizeCancelRequiresExactQuiescence(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	claimed, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	claimed.ExecutionHandoff.LastHeartbeatAt = "2026-07-11T00:50:00Z"
	if _, err := WriteIssueOps(stateRoot, claimed); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "worker stopped responding",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: record.ExecutionHandoff.Orca.WorkerMailboxHandle, PTYID: record.ExecutionHandoff.Orca.WorkerPTYID,
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.WorktreePath, Connected: true, Writable: true,
	}}
	client.dispatch.Status = "dispatched"
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("active terminal and dispatched task must prevent cancellation finalize")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Cancellation == nil {
		t.Fatalf("failed finalize released cancellation tombstone: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffFinalizeCancelRejectsConnectedRuntimeReissuedWorker(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	claimed, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	claimed.ExecutionHandoff.LastHeartbeatAt = "2026-07-11T00:50:00Z"
	if _, err := WriteIssueOps(stateRoot, claimed); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "runtime restarted while worker was active",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-reissued", PTYID: "pty-reissued",
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Title:     "dynamic Codex title",
		Connected: true, Writable: true,
	}}
	client.dispatch.Status = "failed"
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "possible writer") {
		t.Fatalf("runtime-reissued connected worker must keep cancellation tombstone: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Cancellation == nil {
		t.Fatalf("runtime restart released cancellation guard: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffFinalizeCancelRejectsSiblingPossibleWriterAndDispatchedAssignment(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{name: "writable sibling", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = []port.OrcaTerminal{{Handle: "term-sibling", PTYID: "pty-sibling", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.WorktreePath, Writable: true}}
		}},
		{name: "dispatched assignment", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = nil
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-sibling", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-sibling": {ID: "dispatch-sibling", TaskID: "task-sibling", AssigneeHandle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, Status: "dispatched"}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock()); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.dispatch.Status = "failed"
			tt.setup(record, client)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "finalize-cancel", Confirm: true}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "possible writer") {
				t.Fatalf("possible writer finalized cancellation: %v", err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Cancellation == nil {
				t.Fatalf("possible writer released cancellation tombstone: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffFinalizeCancelClosesAfterStaleDisconnectedFailedEvidence(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	claimed, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	claimed.ExecutionHandoff.LastHeartbeatAt = "2026-07-11T00:50:00Z"
	if _, err := WriteIssueOps(stateRoot, claimed); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "verified stale worker",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: record.ExecutionHandoff.Orca.WorkerMailboxHandle, PTYID: record.ExecutionHandoff.Orca.WorkerPTYID,
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.WorktreePath, Connected: false, Writable: false,
	}}
	client.dispatch.Status = "failed"
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled {
		t.Fatalf("verified quiescent cancellation did not close: %#v", got)
	}
}

func TestHandoffFinalizeCancelRejectsMalformedQuiescenceInventory(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{}}
	client.dispatch.Status = "failed"
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "terminal inventory contains missing or duplicate stable identity") {
		t.Fatalf("malformed terminal inventory must not prove quiescence: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Cancellation == nil {
		t.Fatalf("malformed inventory released cancellation tombstone: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffFinalizeCancelRequiresPendingDispatchTaskToLeaveReadyInventory(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	h := record.ExecutionHandoff
	h.State = handoff.StateRecoveryRequired
	h.Orca.DispatchID = ""
	h.Orca.WorkerMailboxHandle = ""
	h.DeliveryMode = ""
	h.DispatchedAt = ""
	h.PendingOperation = &IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationDispatch, StartedAt: "2026-07-11T00:50:00Z", ExpectedAssigneeHandle: h.Orca.WorkerTerminalHandle, DeliveryMode: "inject",
	}
	h.Failure = &IssueOpsExecutionHandoffFailure{Code: "dispatch_ambiguous", Message: "dispatch result is unknown", At: "2026-07-11T00:50:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: h.Orca.WorkerTerminalHandle, PTYID: h.Orca.WorkerPTYID, WorktreeID: h.Orca.WorktreeID,
		WorktreePath: h.WorkerRoot, Connected: false, Writable: false,
	}}
	client.dispatchShowErr = &port.OrcaError{Code: "not_found", Detail: "dispatch absent", Invoked: true}
	client.tasks = []port.OrcaTask{{ID: h.Orca.TaskID, Title: "task", DisplayName: "cycle", Status: "ready"}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "exact worker task is still ready") {
		t.Fatalf("ready task must keep pending-dispatch cancellation open: %v", err)
	}
	client.tasks = nil
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil || got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled {
		t.Fatalf("absent dispatch plus non-ready task did not finalize: result=%#v err=%v", got, err)
	}
}

func TestHandoffForcedCancelRedactsReasonBeforeStateAndProjection(t *testing.T) {
	tests := []struct {
		name, value, reason string
	}{
		{name: "authorization bearer", value: "opaque-bearer-value-7F3A", reason: "Authorization: Bearer opaque-bearer-value-7F3A"},
		{name: "api key", value: "opaque-api-value-91C2", reason: "api_key=opaque-api-value-91C2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			claim := handoffClaimRequest(record)
			if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
				t.Fatal(err)
			}
			result, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: tt.reason,
			}, nil, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, encoded := range []string{string(rawIssueOpsBytesForTest(t, stateRoot, record.ID)), fmt.Sprintf("%#v", persisted), fmt.Sprintf("%#v", result)} {
				if strings.Contains(encoded, tt.value) {
					t.Fatalf("forced-cancel state/projection leaked %s value: %s", tt.name, encoded)
				}
			}
		})
	}
}

func TestHandoffRetryUsesNewAttemptAndEpoch(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	finalizeCancelledHandoffForTest(t, stateRoot, record.ID)
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "retry", Confirm: true}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.State != handoff.StateCoordinatorPreparing {
		t.Fatalf("retry did not create new fenced attempt: %#v", got)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff.OwnershipEpoch != "epoch-2" {
		t.Fatalf("epoch not replaced: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffRetryAllowsWorkerFailedDisposition(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
	record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
	record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "epoch-worker-failed", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.State != handoff.StateCoordinatorPreparing {
		t.Fatalf("worker_failed retry did not create a new fenced attempt: %#v", got)
	}
}

func TestHandoffRetryPreservesBoundedPriorAttemptAudit(t *testing.T) {
	t.Run("worker failed result and cleanup", func(t *testing.T) {
		stateRoot, record, _ := dispatchedHandoffRecord(t)
		record.ExecutionHandoff.State = handoff.StateClosed
		record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
		record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-old", AgentID: "agent-old"}
		record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs = []string{"pty-baseline"}
		record.ExecutionHandoff.Orca.WorkerPTYID = "pty-old"
		record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-old"
		record.ExecutionHandoff.Orca.WorkerTabID = "tab-old"
		record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-old"
		record.ExecutionHandoff.Orca.TaskID = "task-old"
		record.ExecutionHandoff.Orca.DispatchID = "dispatch-old"
		record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
		record.ExecutionHandoff.Result.CleanupReceipts = []string{"old worker resources stopped"}
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "retry", Confirm: true,
		}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-new", nil }}); err != nil {
			t.Fatal(err)
		}
		assertPriorAttemptJSON(t, stateRoot, record.ID, handoff.DispositionWorkerFailed, record.ExecutionHandoff.Orca, "pty-old", "term-old", "task-old", "dispatch-old", "old worker resources stopped", "")
	})

	t.Run("forced cancel failure", func(t *testing.T) {
		stateRoot, record, _ := dispatchedHandoffRecord(t)
		record.ExecutionHandoff.Orca.WorkerTabID = "tab-1"
		record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-1"
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(record)); err != nil {
			t.Fatal(err)
		}
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "worker lease proved stale",
		}, nil, handoffPrepareTestClock()); err != nil {
			t.Fatal(err)
		}
		finalizeCancelledHandoffForTest(t, stateRoot, record.ID)
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "retry", Confirm: true,
		}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-new", nil }}); err != nil {
			t.Fatal(err)
		}
		assertPriorAttemptJSON(t, stateRoot, record.ID, handoff.DispositionCancelled, record.ExecutionHandoff.Orca, "pty-1", "term-1", "task-1", "dispatch-1", "", "cancellation_finalized")
	})
}

func finalizeCancelledHandoffForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	h := record.ExecutionHandoff
	if h == nil || h.Cancellation == nil || h.Orca == nil {
		t.Fatalf("missing cancellation tombstone identity: %#v", h)
	}
	if h.WorkerSession != nil {
		h.LastHeartbeatAt = "2026-07-11T00:50:00Z"
		record, err = WriteIssueOps(stateRoot, record)
		if err != nil {
			t.Fatal(err)
		}
		h = record.ExecutionHandoff
	}
	client := handoffDispatchFake(record)
	if h.Orca.WorkerPTYID != "" || h.Orca.WorkerMailboxHandle != "" {
		client.terminals = []port.OrcaTerminal{{
			Handle: h.Orca.WorkerMailboxHandle, PTYID: h.Orca.WorkerPTYID, WorktreeID: h.Orca.WorktreeID,
			WorktreePath: h.WorkerRoot, Connected: false, Writable: false,
		}}
	}
	if h.Orca.TaskID != "" || h.Orca.DispatchID != "" {
		client.dispatch = port.OrcaDispatch{
			ID: h.Orca.DispatchID, TaskID: h.Orca.TaskID, AssigneeHandle: h.Orca.WorkerMailboxHandle, Status: "failed",
		}
	}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: id, Action: "finalize-cancel", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled {
		t.Fatalf("quiescent cancellation did not finalize: %#v", got)
	}
}

func quiescentRetryClient(t *testing.T, stateRoot, id string) *dispatchOrcaFake {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		t.Fatalf("retry fixture lacks Orca identity: %#v", h)
	}
	if h.Orca.WorkerPTYID != "" || h.Orca.WorkerTerminalHandle != "" {
		client.terminals = []port.OrcaTerminal{{
			Handle: h.Orca.WorkerTerminalHandle, PTYID: h.Orca.WorkerPTYID, WorktreeID: h.Orca.WorktreeID,
			WorktreePath: h.WorkerRoot,
		}}
	}
	if h.Orca.TaskID != "" || h.Orca.DispatchID != "" {
		client.dispatch = port.OrcaDispatch{
			ID: h.Orca.DispatchID, TaskID: h.Orca.TaskID, AssigneeHandle: h.Orca.WorkerMailboxHandle, Status: "failed",
		}
	}
	if h.Cleanup == nil {
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: id, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "retry", Reason: "reuse the quiescent worktree for the next fenced attempt",
		}, client, handoffPrepareTestClock()); err != nil {
			t.Fatal(err)
		}
		for _, step := range []string{"task_terminal", "terminal_quiescent"} {
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: id, Action: "record-cleanup", Confirm: true, CleanupStep: step,
			}, client, handoffPrepareTestClock()); err != nil {
				t.Fatal(err)
			}
		}
	}
	return client
}

func assertPriorAttemptJSON(t *testing.T, stateRoot, id, disposition string, oldOrca *IssueOpsOrcaIdentity, pty, mailbox, task, dispatch, cleanup, failure string) {
	t.Helper()
	var raw struct {
		ExecutionHandoff struct {
			Attempt       int `json:"attempt"`
			PriorAttempts []struct {
				Attempt           int    `json:"attempt"`
				ClosedDisposition string `json:"closed_disposition"`
				Orca              struct {
					WorktreeID          string `json:"worktree_id"`
					WorktreeInstanceID  string `json:"worktree_instance_id"`
					WorktreePath        string `json:"worktree_path"`
					WorkerPTYID         string `json:"worker_pty_id"`
					WorkerMailboxHandle string `json:"worker_mailbox_handle"`
					WorkerTabID         string `json:"worker_tab_id"`
					WorkerLeafID        string `json:"worker_leaf_id"`
					TaskID              string `json:"task_id"`
					DispatchID          string `json:"dispatch_id"`
				} `json:"orca"`
				Result *struct {
					CleanupReceipts []string `json:"cleanup_receipts"`
				} `json:"result"`
				Failure *struct {
					Code string `json:"code"`
				} `json:"failure"`
			} `json:"prior_attempts"`
		} `json:"execution_handoff"`
	}
	if err := json.Unmarshal(rawIssueOpsBytesForTest(t, stateRoot, id), &raw); err != nil {
		t.Fatal(err)
	}
	if raw.ExecutionHandoff.Attempt != 2 || len(raw.ExecutionHandoff.PriorAttempts) != 1 {
		t.Fatalf("retry must retain one bounded prior attempt: %#v", raw.ExecutionHandoff)
	}
	prior := raw.ExecutionHandoff.PriorAttempts[0]
	if prior.Attempt != 1 || prior.ClosedDisposition != disposition || prior.Orca.WorktreeID != oldOrca.WorktreeID || prior.Orca.WorktreeInstanceID != oldOrca.WorktreeInstanceID || prior.Orca.WorktreePath != oldOrca.WorktreePath || prior.Orca.WorkerPTYID != pty || prior.Orca.WorkerMailboxHandle != mailbox || prior.Orca.WorkerTabID != oldOrca.WorkerTabID || prior.Orca.WorkerLeafID != oldOrca.WorkerLeafID || prior.Orca.TaskID != task || prior.Orca.DispatchID != dispatch {
		t.Fatalf("prior attempt lost external identity: %#v", prior)
	}
	if cleanup != "" && (prior.Result == nil || len(prior.Result.CleanupReceipts) != 1 || prior.Result.CleanupReceipts[0] != cleanup) {
		t.Fatalf("prior attempt lost cleanup evidence: %#v", prior.Result)
	}
	if failure != "" && (prior.Failure == nil || prior.Failure.Code != failure) {
		t.Fatalf("prior attempt lost failure evidence: %#v", prior.Failure)
	}
}

func TestHandoffForceAbandonRequiresConfirmedCompleteAbsentInventoryAndNeverRetries(t *testing.T) {
	tests := []struct {
		name    string
		pending IssueOpsExecutionHandoffPendingOperation
		prepare func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{
			name: "worktree create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationWorktreeCreate, StartedAt: "2026-07-11T00:50:00Z", BaselineWorktreeIDs: []string{"wt-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
			},
		},
		{
			name: "terminal create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationTerminalCreate, StartedAt: "2026-07-11T00:50:00Z", BaselinePTYIDs: []string{"pty-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}}
			},
		},
		{
			name: "task create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationTaskCreate, StartedAt: "2026-07-11T00:50:00Z", BaselineTaskIDs: []string{"task-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}}
			},
		},
		{
			name: "dispatch",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationDispatch, StartedAt: "2026-07-11T00:50:00Z", ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject",
			},
			prepare: func(record IssueOpsRecord, client *dispatchOrcaFake) {
				record.ExecutionHandoff.Orca.TaskID = "task-1"
				client.dispatchShowErr = &port.OrcaError{Code: "not_found", Invoked: true}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			if tt.pending.Kind == handoff.OperationDispatch {
				record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
				record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
				record.ExecutionHandoff.Orca.TaskID = "task-1"
			}
			setRecoveryRequiredForTest(&record, tt.pending)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			tt.prepare(record, client)
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "complete inventory proves the invoked operation created no artifact",
			}, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled || got.RecoveryCode != "force_abandoned_absent_operation" {
				t.Fatalf("force abandon did not close the absent operation: %#v", got)
			}
			before := append([]string(nil), client.trace...)
			if started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err != nil || started.State != handoff.StateClosed {
				t.Fatalf("abandoned attempt start must be an inert terminal projection: %#v err=%v", started, err)
			}
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "retry", Confirm: true,
			}, client, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "must-not-be-used", nil }}); err == nil {
				t.Fatal("force-abandoned ambiguous operation must require a fresh cycle")
			}
			if strings.Join(client.trace, "\n") != strings.Join(before, "\n") || client.terminalCreates != 0 || client.dispatchCalls != 0 {
				t.Fatalf("abandoned operation was retried: trace before=%v after=%v creates=%d dispatch=%d", before, client.trace, client.terminalCreates, client.dispatchCalls)
			}
		})
	}
}

func TestHandoffForceAbandonRejectsReadOnlyRuntimeRefreshByteEquivalently(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationRuntimeRefresh, StartedAt: "2026-07-11T00:50:00Z",
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	client := handoffDispatchFake(record)
	_, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "runtime identity requires exact reconciliation",
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "runtime_refresh is a read-only identity reconciliation and cannot be force-abandoned") {
		t.Fatalf("runtime refresh abandon error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("runtime refresh abandon inspected Orca: %v", client.trace)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(before, after) {
		t.Fatal("runtime refresh abandon changed the durable lease")
	}
}

func TestHandoffForceAbandonRejectsMissingAuthorityAndYoungJournal(t *testing.T) {
	tests := []struct {
		name    string
		request IssueOpsHandoffRecoverRequest
		started string
		want    string
	}{
		{name: "confirmation", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Force: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:50:00Z", want: "abandon requires --confirm"},
		{name: "force", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:50:00Z", want: "abandon requires --force"},
		{name: "reason", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true}, started: "2026-07-11T00:50:00Z", want: "abandon requires a nonempty --reason"},
		{name: "reason bound", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true, Reason: strings.Repeat("x", 4097)}, started: "2026-07-11T00:50:00Z", want: "abandon reason exceeds 4096 bytes"},
		{name: "minimum age", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:57:04Z", want: "pending operation must be at least 5m0s old"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationWorktreeCreate, StartedAt: tt.started, BaselineWorktreeIDs: []string{"wt-old"},
			})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			client := handoffDispatchFake(record)
			client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
			tt.request.ID = record.ID
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, tt.request, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("force abandon error = %v, want %q", err, tt.want)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if string(after) != string(before) || len(client.trace) != 0 {
				t.Fatalf("rejected force abandon changed state or inspected Orca: trace=%v", client.trace)
			}
		})
	}
}

func TestHandoffForceAbandonRequiresAuthoritativeAbsenceAndExactFence(t *testing.T) {
	tests := []struct {
		name    string
		pending IssueOpsExecutionHandoffPendingOperation
		prepare func(string, IssueOpsRecord, *dispatchOrcaFake)
		want    string
	}{
		{
			name: "worktree exact candidate", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, record IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {
					ID: "wt-new", RepoID: record.ExecutionHandoff.Orca.RepoID, BaseRef: record.ExecutionHandoff.Orca.BaseRef,
					Path: record.ExecutionHandoff.WorkerRoot, Comment: issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
				}}
			},
			want: "worktree inventory contains 1 exact post-baseline candidate",
		},
		{
			name: "worktree truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktreeListErr = fmt.Errorf("Orca worktree list is incomplete")
			},
			want: "Orca worktree list is incomplete",
		},
		{
			name: "worktree missing stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {}}
			},
			want: "worktree inventory contains missing or duplicate stable identity",
		},
		{
			name: "worktree duplicate stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, record IssueOpsRecord, client *dispatchOrcaFake) {
				row := port.OrcaWorktree{ID: "wt-new", RepoID: record.ExecutionHandoff.Orca.RepoID, BaseRef: record.ExecutionHandoff.Orca.BaseRef, Path: record.ExecutionHandoff.WorkerRoot, Comment: "another cycle"}
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, row, row}
			},
			want: "worktree inventory contains missing or duplicate stable identity",
		},
		{
			name: "worktree missing classification fields", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {ID: "wt-new"}}
			},
			want: "worktree inventory row is missing classification fields",
		},
		{
			name: "terminal exact candidate", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, record IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {
					PTYID: "pty-new", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
					Title: issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
				}}
			},
			want: "terminal inventory contains 1 exact post-baseline candidate",
		},
		{
			name: "terminal truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminalListErr = fmt.Errorf("Orca terminal list is incomplete")
			},
			want: "Orca terminal list is incomplete",
		},
		{
			name: "terminal missing stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {}}
			},
			want: "terminal inventory contains missing or duplicate stable identity",
		},
		{
			name: "terminal duplicate stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, record IssueOpsRecord, client *dispatchOrcaFake) {
				row := port.OrcaTerminal{PTYID: "pty-new", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, Title: "another terminal"}
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, row, row}
			},
			want: "terminal inventory contains missing or duplicate stable identity",
		},
		{
			name: "terminal missing classification fields", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {PTYID: "pty-new"}}
			},
			want: "terminal inventory row is missing classification fields",
		},
		{
			name: "task exact candidate", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, record IssueOpsRecord, client *dispatchOrcaFake) {
				title, display, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
				if err != nil {
					t.Fatal(err)
				}
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {ID: "task-new", Status: "ready", Title: title, DisplayName: display}}
			},
			want: "task inventory contains 1 exact post-baseline candidate",
		},
		{
			name: "task truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.taskListErr = fmt.Errorf("Orca task list is incomplete")
			},
			want: "Orca task list is incomplete",
		},
		{
			name: "task missing stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {Status: "ready"}}
			},
			want: "task inventory contains missing or duplicate stable identity",
		},
		{
			name: "task duplicate stable identity", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				row := port.OrcaTask{ID: "task-new", Status: "ready", Title: "another task", DisplayName: "another cycle"}
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, row, row}
			},
			want: "task inventory contains missing or duplicate stable identity",
		},
		{
			name: "task missing classification fields", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {ID: "task-new", Status: "ready"}}
			},
			want: "task inventory row is missing classification fields",
		},
		{
			name: "dispatch exists", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) { client.dispatchShowErr = nil },
			want:    "dispatch still exists",
		},
		{
			name: "dispatch unknown error", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.dispatchShowErr = fmt.Errorf("transport unavailable")
			},
			want: "inspect dispatch absence",
		},
		{
			name: "record changed during inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(stateRoot string, record IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
				client.beforeWorktreeList = func() {
					current, err := ReadIssueOps(stateRoot, record.ID)
					if err != nil {
						t.Fatal(err)
					}
					current.UpdatedAt = "2026-07-11T01:01:00Z"
					if _, err := WriteIssueOps(stateRoot, current); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: "handoff changed during force-abandon inventory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			tt.pending.StartedAt = "2026-07-11T00:50:00Z"
			if tt.pending.Kind == handoff.OperationDispatch {
				record.ExecutionHandoff.Orca.TaskID = "task-1"
			}
			setRecoveryRequiredForTest(&record, tt.pending)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			tt.prepare(stateRoot, record, client)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "inventory cannot authoritatively prove absence",
			}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("force abandon error = %v, want %q", err, tt.want)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil {
				t.Fatalf("rejected force abandon changed the journal: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffForceAbandonIgnoresUnrelatedRuntimeArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		pending IssueOpsExecutionHandoffPendingOperation
		prepare func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{
			name: "worktree", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(record IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {
					ID: "wt-unrelated", RepoID: record.ExecutionHandoff.Orca.RepoID, BaseRef: record.ExecutionHandoff.Orca.BaseRef,
					Path: record.ExecutionHandoff.WorkerRoot, Comment: "another cycle",
				}}
			},
		},
		{
			name: "terminal", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(record IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {
					PTYID: "pty-unrelated", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, Title: "another terminal",
				}}
			},
		},
		{
			name: "task", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {ID: "task-unrelated", Status: "ready", Title: "another task", DisplayName: "another cycle"}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			tt.pending.StartedAt = "2026-07-11T00:50:00Z"
			setRecoveryRequiredForTest(&record, tt.pending)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			tt.prepare(record, client)
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "exact operation candidate is absent",
			}, client, handoffPrepareTestClock())
			if err != nil || got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled {
				t.Fatalf("unrelated runtime activity blocked exact abandon: result=%#v err=%v", got, err)
			}
		})
	}
}

func TestHandoffRetryPinsCleanPartialCommitAsNewAttemptBase(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/partial.go", "package internal\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: partial worker result"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	partialHead := preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD")
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		Verification: []string{"focused test failed after partial commit"}, CleanupReceipts: []string{"worker resources stopped"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }}); err != nil {
		t.Fatal(err)
	}
	retried, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.ExecutionHandoff.AttemptBaseHead != partialHead {
		t.Fatalf("retry attempt base = %q, want partial HEAD %q", retried.ExecutionHandoff.AttemptBaseHead, partialHead)
	}
	client := handoffDispatchFake(retried)
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := ReadIssueOps(stateRoot, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(dispatched)); err != nil {
		t.Fatalf("new worker claim at the clean retry checkpoint failed: %v", err)
	}
}

func TestHandoffRetryReattestsLegacyCodexBypassWithoutChangingSealedOptions(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
	record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
	record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
	record.ExecutionHandoff.ContextOptions = &model.IssueOpsExecutionHandoffContextOptions{
		WorkerScope: "preserve this exact worker scope", RequiredDocs: []string{"AGENTS.md"}, AllowCodexHookTrustBypass: false,
	}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, quiescentRetryClient(t, stateRoot, record.ID), IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }}); err != nil {
		t.Fatal(err)
	}
	retried, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	options := retried.ExecutionHandoff.ContextOptions
	if options == nil || options.AllowCodexHookTrustBypass || options.WorkerScope != "preserve this exact worker scope" || len(options.RequiredDocs) != 1 || options.RequiredDocs[0] != "AGENTS.md" {
		t.Fatalf("retry must preserve delivery options but clear per-attempt attestation: %#v", options)
	}
	client := handoffDispatchFake(retried)
	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(retried, IssueOpsHandoffStartRequest{
		ID: retried.ID, CoordinatorRecipient: testCoordinatorRecipient, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true},
	}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(retried, IssueOpsHandoffStartRequest{
		ID: retried.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: preview.ContextSHA256, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true},
	}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || !client.terminalRequests[0].AllowCodexHookTrustBypass {
		t.Fatalf("legacy retry attestation did not authorize exactly one Codex launch: result=%#v terminal=%#v", started, client.terminalRequests)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.ContextVersion != 1 || persisted.ExecutionHandoff.ContextVersion != 1 {
		t.Fatalf("optional attestation must remain context version 1 for legacy retry compatibility: constant=%d persisted=%d", handoff.ContextVersion, persisted.ExecutionHandoff.ContextVersion)
	}
	if persisted.ExecutionHandoff.ContextOptions == nil || !persisted.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass || persisted.ExecutionHandoff.ContextOptions.WorkerScope != "preserve this exact worker scope" {
		t.Fatalf("reattestation changed the sealed delivery contract: %#v", persisted.ExecutionHandoff.ContextOptions)
	}
}

func TestHandoffRetryRejectsUnsafeWorktreeCheckpointBeforeNewAttempt(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, IssueOpsRecord)
	}{
		{name: "dirty worktree", mutate: func(t *testing.T, record IssueOpsRecord) {
			writeIssueOpsFile(t, record.WorktreePath, "dirty.txt", "dirty\n")
		}},
		{name: "branch mismatch", mutate: func(t *testing.T, record IssueOpsRecord) {
			if code, _, stderr := preflight.GitCmd(record.WorktreePath, "switch", "-q", "-c", "other-branch"); code != 0 {
				t.Fatalf("git switch failed: %s", stderr)
			}
		}},
		{name: "head lookup failure", mutate: func(t *testing.T, record IssueOpsRecord) {
			ref := filepath.Join(record.WorktreePath, ".git", "refs", "heads", record.Branch)
			if err := os.Remove(ref); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := gitBackedDispatchedHandoff(t)
			record.ExecutionHandoff.State = handoff.StateClosed
			record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
			record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
			record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, record)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "retry", Confirm: true,
			}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "must-not-be-used", nil }}); err == nil {
				t.Fatal("unsafe retry checkpoint must fail before a new attempt")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateClosed || persisted.ExecutionHandoff.Attempt != 1 {
				t.Fatalf("rejected retry mutated the attempt: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffRetryRejectsAcceptedDisposition(t *testing.T) {
	stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	if _, err := AcceptIssueOpsHandoff(stateRoot, coordinatorAcceptRequest(record, IssueOpsHandoffAcceptRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
		ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
	})); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "must-not-be-used", nil
		},
	}); err == nil {
		t.Fatal("accepted handoff must remain terminal and reject retry")
	}

	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := persisted.ExecutionHandoff
	if got.State != handoff.StateClosed || got.ClosedDisposition != handoff.DispositionAccepted || got.Attempt != 1 || got.OwnershipEpoch != record.ExecutionHandoff.OwnershipEpoch {
		t.Fatalf("rejected accepted retry mutated terminal handoff: %#v", got)
	}
}

func TestHandoffRecoverExactOneOnlyAndNeverAdvances(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero"},
		{name: "one", rows: []port.OrcaTerminal{{Handle: "term-new", PTYID: "pty-new", WorktreeID: "wt-1", WorktreePath: "WORKER_ROOT", Connected: true, Writable: true}}, ok: true},
		{name: "multiple", rows: []port.OrcaTerminal{{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1"}, {Handle: "term-2", PTYID: "pty-2", WorktreeID: "wt-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"old"}})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			for i := range tt.rows {
				if tt.rows[i].WorktreePath == "WORKER_ROOT" {
					tt.rows[i].WorktreePath = record.WorktreePath
				}
			}
			client := handoffDispatchFake(record)
			client.terminals = tt.rows
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
			if tt.ok && (err != nil || got.State != handoff.StateCoordinatorPreparing || got.NextCommand == "") {
				t.Fatalf("expected reconciled status only, got %#v err=%v", got, err)
			}
			if tt.ok && (client.taskCreates != 0 || client.dispatchCalls != 0) {
				t.Fatal("reconcile advanced to a next operation")
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

func TestHandoffRecoverDispatchRequiresDurableDeliveryIdentityAndDispatchedStatus(t *testing.T) {
	tests := []struct {
		name, expectedAssignee, deliveryMode, returnedAssignee, status, dispatchID string
	}{
		{name: "assignee mismatch", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-other", status: "dispatched", dispatchID: "dispatch-1"},
		{name: "failed status", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "failed", dispatchID: "dispatch-1"},
		{name: "completed status", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "completed", dispatchID: "dispatch-1"},
		{name: "missing dispatch id", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "dispatched"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.CoordinatorMailboxHandle = testCoordinatorRecipient
			record.ExecutionHandoff.Orca.TaskID = "task-1"
			record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: tt.expectedAssignee, DeliveryMode: tt.deliveryMode,
			})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.dispatch.ID = tt.dispatchID
			client.dispatch.AssigneeHandle = tt.returnedAssignee
			client.dispatch.Status = tt.status
			client.dispatch.Injected = false
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "reconcile",
			}, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("dispatch reconciliation must match its durable delivery identity and exact dispatched status")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil {
				t.Fatalf("failed dispatch reconciliation advanced state: %#v", persisted.ExecutionHandoff)
			}
		})
	}
	client := handoffDispatchFake()
	for _, tt := range []struct{ name, expectedAssignee, deliveryMode string }{
		{name: "missing expected assignee", deliveryMode: "inject"},
		{name: "missing delivery mode", expectedAssignee: "term-1"},
		{name: "unsupported delivery mode", expectedAssignee: "term-1", deliveryMode: "terminal_send"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", tt.expectedAssignee, tt.deliveryMode, client, testCoordinatorRecipient); err == nil {
				t.Fatal("invalid durable delivery journal was accepted")
			}
		})
	}
}

func TestHandoffRecoverDispatchNotFoundReturnsToCoordinatorPreparing(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.CoordinatorMailboxHandle = testCoordinatorRecipient
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
	record.ExecutionHandoff.Orca.TaskID = "task-1"
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject",
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.dispatchShowErr = &port.OrcaError{Code: "not_found", Invoked: true}

	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "reconcile",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateCoordinatorPreparing || got.NextCommand == "" {
		t.Fatalf("absent dispatch recovery = %#v, want coordinator preparation retry", got)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	h := persisted.ExecutionHandoff
	if h.PendingOperation != nil || h.Failure != nil || h.DeliveryMode != "" || h.DispatchedAt != "" {
		t.Fatalf("absent dispatch recovery retained terminal state: %#v", h)
	}
	if h.Orca.TaskID != "task-1" || h.Orca.WorkerTerminalHandle != "term-1" || h.Orca.DispatchID != "" || h.Orca.WorkerMailboxHandle != "" {
		t.Fatalf("absent dispatch recovery changed retained identity: %#v", h.Orca)
	}
}

func TestHandoffRecoverDispatchRejectsInconsistentV4MailboxAuthority(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	h := record.ExecutionHandoff
	h.CoordinatorMailboxHandle = testCoordinatorRecipient
	h.Orca.WorkerTerminalHandle = "term-live"
	h.Orca.WorkerMailboxHandle = "term-stale-sealed"
	h.Orca.TaskID = "task-1"
	h.Orca.DispatchID = ""
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-live", DeliveryMode: "inject",
	})
	putRawIssueOpsRecordForTest(t, stateRoot, record)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	client := handoffDispatchFake(record)
	client.dispatch = port.OrcaDispatch{ID: "dispatch-1", TaskID: "task-1", AssigneeHandle: "term-live", Status: "dispatched"}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "reconcile",
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "dispatch id and worker mailbox") {
		t.Fatalf("dispatch recovery error = %v, want unpaired sealed worker mailbox rejection", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("invalid v4 authority reached Orca: %v", client.trace)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("rejected inconsistent v4 recovery authority was overwritten")
	}
}

func TestHandoffRecoverDispatchUsesDurableRefreshedAssigneeWithoutInjectedField(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.TaskID = "task-1"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.refreshedTerminal = port.OrcaTerminal{
		Handle: "term-live", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true,
	}
	client.dispatchErr = &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true}
	client.dispatch.AssigneeHandle = "term-live"
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("expected ambiguous dispatch")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	operation := pending.ExecutionHandoff.PendingOperation
	if operation == nil || operation.Kind != handoff.OperationDispatch || operation.ExpectedAssigneeHandle != "term-live" || operation.DeliveryMode != "inject" {
		t.Fatalf("dispatch journal did not seal refreshed delivery identity: %#v", operation)
	}
	client.dispatchErr = nil
	client.dispatch = port.OrcaDispatch{ID: "dispatch-live", TaskID: "task-1", AssigneeHandle: "term-live", Status: "dispatched"}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.DispatchID != "dispatch-live" || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("installed-shape dispatch was not reconciled: %#v", got)
	}
}
