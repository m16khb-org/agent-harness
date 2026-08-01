package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestResumeIntentSpikeUsesFixedOperationID(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatal(err)
	}

	const operationID = "11111111111111111111111111111111"
	persisted, payload, err := beginOrcaExecutionResumeIntentWithID(
		stateRoot,
		record,
		artifacts,
		record.Execution.Orca.RuntimeID,
		"",
		operationID,
		func() time.Time { return time.Date(2026, time.July, 31, 2, 20, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if payload.OperationID != operationID || persisted.Execution.Pending == nil ||
		!strings.Contains(persisted.Execution.Pending.OperationID, operationID) {
		t.Fatalf("fixed resume intent = record=%#v payload=%#v", persisted.Execution.Pending, payload)
	}
}

func TestResumeIntentSpikeBridgeMatchesLegacyBeginRawBytes(t *testing.T) {
	legacyRoot, record, _ := reseededOrcaCycle(t)
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsRow(t, legacyRoot, record.ID)
	verticalRoot := t.TempDir()
	verticalDB, err := sqlstore.Open(verticalRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := verticalDB.Apply(context.Background(), []sqlstore.Mutation{{Bucket: issueOpsBucket, ID: record.ID, Data: before}}); err != nil {
		t.Fatal(err)
	}
	const operationID = "22222222222222222222222222222222"
	clock := fixedResumeIntentClock
	if _, _, err := beginOrcaExecutionResumeIntentWithID(legacyRoot, record, artifacts, record.Execution.Orca.RuntimeID, "", operationID, clock); err != nil {
		t.Fatal(err)
	}
	verticalRecord, err := decodeIssueOpsRecord(record.ID, before)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BeginExecutionResumeIntent(verticalRoot, verticalRecord, before, executionResumeArtifactsReceipt(artifacts), record.Execution.Orca.RuntimeID, "", operationID, clock); err != nil {
		t.Fatal(err)
	}
	if got, want := rawIssueOpsRow(t, verticalRoot, record.ID), rawIssueOpsRow(t, legacyRoot, record.ID); !bytes.Equal(got, want) {
		t.Fatalf("record bytes drifted\nlegacy=%s\nvertical=%s", want, got)
	}
	if got, want := rawExternalIntentRow(t, verticalRoot, operationID), rawExternalIntentRow(t, legacyRoot, operationID); !bytes.Equal(got, want) {
		t.Fatalf("external intent bytes drifted")
	}
}

func TestResumeIntentSpikeBridgePendingIsConsumedByLegacyReconcile(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	artifacts, err := ReadExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatal(err)
	}
	beforeLease := record.Execution.Lease
	state := beginResumeBridgeIntent(t, stateRoot, record, artifacts, strings.Repeat("e", 32))
	state = advanceResumeBridgeStage(t, stateRoot, state, port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-resume"})
	state = advanceResumeBridgeStage(t, stateRoot, state, port.ExecutionOrcaIntentReceipt{RunID: "run-resume"})
	state = advanceResumeBridgeStage(t, stateRoot, state, port.ExecutionOrcaIntentReceipt{RunID: "run-resume", RunBound: true})
	state = advanceResumeBridgeStage(t, stateRoot, state, port.ExecutionOrcaIntentReceipt{TaskID: "task-resume"})
	state, err = MarkExecutionResumeIntentInvoking(stateRoot, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordExecutionResumeIntentFailure(stateRoot, state, orcaIntentUnknown, errors.New("transport"), fixedResumeIntentClock); err != nil {
		t.Fatal(err)
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution.Pending == nil || pending.Execution.Pending.Kind != "dispatch" || pending.Execution.Lease != beforeLease {
		t.Fatalf("pending=%#v", pending.Execution)
	}
	fake := &executionOrcaFake{inspect: func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		if request.Stage != port.ExecutionOrcaIntentDispatch {
			t.Fatalf("reconcile stage=%s", request.Stage)
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TaskID: request.TaskID, DispatchID: "dispatch-resume"}}}, nil
	}}
	reconciled, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{ID: record.ID, Confirm: true, Actor: executionActor("codex", "resume-reconciler"), CWD: record.Execution.Workspace.Root}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Execution.Pending != nil || reconciled.Execution.Lease != beforeLease || reconciled.Execution.Orca.DispatchID != "dispatch-resume" || reconciled.Execution.Orca.LeaseGeneration != beforeLease.Generation {
		t.Fatalf("reconciled=%#v", reconciled.Execution)
	}
}

func TestResumeIntentBridgeCASRejectsBeginMarkAndReceiptDriftWithoutMutation(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, stateRoot string, record IssueOpsRecord, artifacts ExecutionResumeArtifactsReceipt)
	}{
		{
			name: "begin",
			run: func(t *testing.T, stateRoot string, record IssueOpsRecord, artifacts ExecutionResumeArtifactsReceipt) {
				raw := rawIssueOpsRow(t, stateRoot, record.ID)
				writeResumeBridgeDrift(t, stateRoot, record)
				beforeRecord, beforeIntent := rawIssueOpsRow(t, stateRoot, record.ID), []byte(nil)
				_, err := BeginExecutionResumeIntent(stateRoot, record, raw, artifacts, record.Execution.Orca.RuntimeID, "", strings.Repeat("a", 32), fixedResumeIntentClock)
				if err == nil || !strings.Contains(err.Error(), "stale raw record snapshot") {
					t.Fatalf("begin error=%v", err)
				}
				assertResumeBridgeRows(t, stateRoot, record.ID, strings.Repeat("a", 32), beforeRecord, beforeIntent)
			},
		},
		{
			name: "mark",
			run: func(t *testing.T, stateRoot string, record IssueOpsRecord, artifacts ExecutionResumeArtifactsReceipt) {
				intent := beginResumeBridgeIntent(t, stateRoot, record, artifacts, strings.Repeat("b", 32))
				writeResumeBridgeIntentDrift(t, stateRoot, intent.OperationID)
				beforeRecord, beforeIntent := rawIssueOpsRow(t, stateRoot, record.ID), rawExternalIntentRow(t, stateRoot, intent.OperationID)
				_, err := MarkExecutionResumeIntentInvoking(stateRoot, intent)
				if err == nil || !strings.Contains(err.Error(), "stale raw intent snapshot") {
					t.Fatalf("mark error=%v", err)
				}
				assertResumeBridgeRows(t, stateRoot, record.ID, intent.OperationID, beforeRecord, beforeIntent)
			},
		},
		{
			name: "receipt",
			run: func(t *testing.T, stateRoot string, record IssueOpsRecord, artifacts ExecutionResumeArtifactsReceipt) {
				intent := beginResumeBridgeIntent(t, stateRoot, record, artifacts, strings.Repeat("c", 32))
				writeResumeBridgeDrift(t, stateRoot, record)
				beforeRecord, beforeIntent := rawIssueOpsRow(t, stateRoot, record.ID), rawExternalIntentRow(t, stateRoot, intent.OperationID)
				_, err := ApplyExecutionResumeIntentReceipt(context.Background(), stateRoot, intent, port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-resume"}, fixedResumeIntentClock)
				if err == nil || !strings.Contains(err.Error(), "stale raw record snapshot") {
					t.Fatalf("receipt error=%v", err)
				}
				assertResumeBridgeRows(t, stateRoot, record.ID, intent.OperationID, beforeRecord, beforeIntent)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := reseededOrcaCycle(t)
			artifacts, err := ReadExecutionResumeArtifacts(record)
			if err != nil {
				t.Fatal(err)
			}
			tt.run(t, stateRoot, record, artifacts)
		})
	}
}

var fixedResumeIntentClock = func() time.Time { return time.Date(2026, time.July, 31, 2, 20, 0, 0, time.UTC) }

func beginResumeBridgeIntent(t *testing.T, stateRoot string, record IssueOpsRecord, artifacts ExecutionResumeArtifactsReceipt, operationID string) ExecutionResumeIntentState {
	t.Helper()
	state, err := BeginExecutionResumeIntent(stateRoot, record, rawIssueOpsRow(t, stateRoot, record.ID), artifacts, record.Execution.Orca.RuntimeID, "", operationID, fixedResumeIntentClock)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func advanceResumeBridgeStage(t *testing.T, stateRoot string, state ExecutionResumeIntentState, receipt port.ExecutionOrcaIntentReceipt) ExecutionResumeIntentState {
	t.Helper()
	state, err := MarkExecutionResumeIntentInvoking(stateRoot, state)
	if err != nil {
		t.Fatal(err)
	}
	next, err := ApplyExecutionResumeIntentReceipt(context.Background(), stateRoot, state, receipt, fixedResumeIntentClock)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func writeResumeBridgeDrift(t *testing.T, stateRoot string, record IssueOpsRecord) {
	t.Helper()
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.UpdatedAt = current.UpdatedAt + "-drift"
	if _, err := writeIssueOps(stateRoot, current); err != nil {
		t.Fatal(err)
	}
}

func writeResumeBridgeIntentDrift(t *testing.T, stateRoot, operationID string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil || !ok {
		t.Fatalf("read pending intent ok=%t err=%v", ok, err)
	}
	var payload externalOrcaIntentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	payload.InvocationState, payload.InvocationAttempts = orcaIntentUnknown, 1
	changed, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: operationID, Data: changed}}); err != nil {
		t.Fatal(err)
	}
}

func assertResumeBridgeRows(t *testing.T, stateRoot, id, operationID string, wantRecord, wantIntent []byte) {
	t.Helper()
	if got := rawIssueOpsRow(t, stateRoot, id); !bytes.Equal(got, wantRecord) {
		t.Fatalf("record bytes changed after rejected CAS")
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if wantIntent == nil {
		if ok {
			t.Fatal("rejected begin created an external intent")
		}
		return
	}
	if !ok || !bytes.Equal(got, wantIntent) {
		t.Fatal("intent bytes changed after rejected CAS")
	}
}
