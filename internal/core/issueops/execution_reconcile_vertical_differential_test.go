package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestReconcileVerticalDifferentialPreservesPublicAndRawContracts(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{successfulExecutionOrcaIntentReceipt(t, request)}}, nil
	}
	if _, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("codex", "differential-seed"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader, Now: reconcileDifferentialClock}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		inventory port.ExecutionOrcaIntentInventory
	}{
		{name: "exact candidate", inventory: port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TerminalPTYID: "pty-differential"}}}},
		{name: "multiple candidates", inventory: port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TerminalPTYID: "pty-a"}, {TerminalPTYID: "pty-b"}}}},
		{name: "non authoritative zero", inventory: port.ExecutionOrcaIntentInventory{}},
		{name: "unknown authoritative zero", inventory: port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyRoot := cloneReconcileState(t, stateRoot, record.ID)
			verticalRoot := cloneReconcileState(t, stateRoot, record.ID)
			legacy := observeLegacyReconcile(t, legacyRoot, record.ID, record.Repo, test.inventory)
			vertical := observeVerticalReconcile(t, verticalRoot, record.ID, test.inventory)
			if !bytes.Equal(legacy.resultJSON, vertical.resultJSON) || legacy.err != vertical.err ||
				!bytes.Equal(legacy.recordRaw, vertical.recordRaw) || !bytes.Equal(legacy.intentRaw, vertical.intentRaw) {
				t.Fatalf("differential mismatch\nlegacy result=%s err=%q record=%s intent=%s\nvertical result=%s err=%q record=%s intent=%s",
					legacy.resultJSON, legacy.err, legacy.recordRaw, legacy.intentRaw,
					vertical.resultJSON, vertical.err, vertical.recordRaw, vertical.intentRaw)
			}
		})
	}
}

func TestReconcileVerticalPreservesLegacyMigrationDisclosure(t *testing.T) {
	stateRoot, record, payload := legacyResumeIntentFixture(t, "github", 16)
	record, _ = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, nil)
	inspectCalls := 0
	fake := &executionOrcaFake{inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		inspectCalls++
		return port.ExecutionOrcaIntentInventory{}, errors.New("transport")
	}}
	effects := &reconcileDifferentialEffects{stateRoot: stateRoot, provisioner: fake}
	result, err := leaseapp.NewReconcileService(
		leaseoutbound.NewReconcileRepository(effects),
		leaseoutbound.NewReconcileStageExecutor(effects.inspect, effects.invoke),
	).Reconcile(context.Background(), leaseapp.ReconcileRequest{ID: record.ID})
	if err == nil || !result.IntentMigrated || !result.ExternalStateInspected || result.Code != "orca_reconcile_ambiguous" || inspectCalls != 1 {
		t.Fatalf("result=%#v inspect=%d err=%v", result, inspectCalls, err)
	}
}

func TestReconcileVerticalRejectsUnsafeLegacyMarkerBeforeInspection(t *testing.T) {
	stateRoot, record, payload := legacyPrepareIntentFixture(t)
	record, _ = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, func(_ *IssueOpsRecord, payload *externalOrcaIntentPayload) {
		payload.InvocationState = orcaIntentUnknown
	})
	inspectCalls := 0
	fake := &executionOrcaFake{inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		inspectCalls++
		return port.ExecutionOrcaIntentInventory{}, nil
	}}
	effects := &reconcileDifferentialEffects{stateRoot: stateRoot, provisioner: fake}
	result, err := leaseapp.NewReconcileService(
		leaseoutbound.NewReconcileRepository(effects),
		leaseoutbound.NewReconcileStageExecutor(effects.inspect, effects.invoke),
	).Reconcile(context.Background(), leaseapp.ReconcileRequest{ID: record.ID})
	if err == nil || result.Code != "legacy_intent_upgrade_unsafe" || result.ExternalStateInspected || inspectCalls != 0 {
		t.Fatalf("result=%#v inspect=%d err=%v", result, inspectCalls, err)
	}
}

type reconcileDifferentialObservation struct {
	resultJSON []byte
	err        string
	recordRaw  []byte
	intentRaw  []byte
}

func observeLegacyReconcile(t *testing.T, stateRoot, id, cwd string, inventory port.ExecutionOrcaIntentInventory) reconcileDifferentialObservation {
	t.Helper()
	fake := &executionOrcaFake{inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return inventory, nil
	}}
	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: id, Confirm: true, Actor: executionActor("codex", "differential-legacy"), CWD: cwd,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader, Now: reconcileDifferentialClock})
	return reconcileDifferentialObservationAt(t, stateRoot, id, result, err)
}

func observeVerticalReconcile(t *testing.T, stateRoot, id string, inventory port.ExecutionOrcaIntentInventory) reconcileDifferentialObservation {
	t.Helper()
	fake := &executionOrcaFake{inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return inventory, nil
	}}
	effects := &reconcileDifferentialEffects{stateRoot: stateRoot, provisioner: fake}
	service := leaseapp.NewReconcileService(
		leaseoutbound.NewReconcileRepository(effects),
		leaseoutbound.NewReconcileStageExecutor(effects.inspect, effects.invoke),
	)
	appResult, err := service.Reconcile(context.Background(), leaseapp.ReconcileRequest{ID: id})
	coreRecord, convertErr := resumeDifferentialCoreRecord(appResult.Record)
	if convertErr != nil {
		t.Fatal(convertErr)
	}
	result := executionReconcileResult(coreRecord, false, appResult.Code)
	result.OK = appResult.OK
	result.Reconciled = appResult.Reconciled
	result.ExternalStateInspected = appResult.ExternalStateInspected
	if appResult.IntentMigrated {
		result.IntentMigrationCode = "legacy_intent_upgraded"
	}
	return reconcileDifferentialObservationAt(t, stateRoot, id, result, err)
}

func reconcileDifferentialObservationAt(t *testing.T, stateRoot, id string, result ExecutionReconcileResult, err error) reconcileDifferentialObservation {
	t.Helper()
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	record, readErr := ReadIssueOps(stateRoot, id)
	if readErr != nil {
		t.Fatal(readErr)
	}
	observation := reconcileDifferentialObservation{resultJSON: resultJSON, recordRaw: rawIssueOpsRow(t, stateRoot, id)}
	if err != nil {
		observation.err = err.Error()
	}
	if record.Execution != nil && record.Execution.Pending != nil {
		observation.intentRaw = rawExternalIntentRow(t, stateRoot, record.Execution.Pending.OperationID)
	}
	return observation
}

type reconcileDifferentialEffects struct {
	stateRoot   string
	provisioner port.ExecutionOrcaProvisioner
}

func (e *reconcileDifferentialEffects) Canonicalize(_ context.Context, id string) (leaseoutbound.ReconcileEffectState, error) {
	state, err := CanonicalizeExecutionReconcileIntent(e.stateRoot, id, nil)
	converted, convertErr := reconcileDifferentialEffectState(state)
	if convertErr != nil {
		return leaseoutbound.ReconcileEffectState{}, convertErr
	}
	return converted, err
}

func (e *reconcileDifferentialEffects) MarkInvoking(_ context.Context, state leaseoutbound.ReconcileEffectState) (leaseoutbound.ReconcileEffectState, error) {
	coreState, err := reconcileDifferentialCoreState(state)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	next, err := MarkExecutionReconcileIntentInvoking(e.stateRoot, coreState)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return reconcileDifferentialEffectState(next)
}

func (e *reconcileDifferentialEffects) RecordFailure(_ context.Context, state leaseoutbound.ReconcileEffectState, invocation string, cause error) error {
	coreState, err := reconcileDifferentialCoreState(state)
	if err != nil {
		return err
	}
	return RecordExecutionReconcileIntentFailure(e.stateRoot, coreState, invocation, cause, reconcileDifferentialClock)
}

func (e *reconcileDifferentialEffects) ApplyReceipt(ctx context.Context, state leaseoutbound.ReconcileEffectState, receipt leasecontract.ReconcileStageReceipt) (leaseoutbound.ReconcileEffectState, error) {
	coreState, err := reconcileDifferentialCoreState(state)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	portReceipt, err := reconcileDifferentialPortReceipt(receipt)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	next, err := ApplyExecutionReconcileIntentReceipt(ctx, e.stateRoot, coreState, portReceipt, executionIssueSnapshotReader, reconcileDifferentialClock)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return reconcileDifferentialEffectState(next)
}

func (e *reconcileDifferentialEffects) Latest(_ context.Context, id string) (leasecontract.Record, error) {
	record, err := ReadExecutionReconcileRecord(e.stateRoot, id)
	if err != nil {
		return leasecontract.Record{}, err
	}
	return reconcileDifferentialContractRecord(record)
}

func (e *reconcileDifferentialEffects) inspect(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
	request, err := e.request(intent)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, false, err
	}
	inventory, err := e.provisioner.InspectIntent(ctx, request)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, true, err
	}
	var result leasecontract.ReconcileStageInventory
	if err := reconcileDifferentialConvert(inventory, &result); err != nil {
		return leasecontract.ReconcileStageInventory{}, true, err
	}
	return result, true, nil
}

func (e *reconcileDifferentialEffects) invoke(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageReceipt, string, error) {
	request, err := e.request(intent)
	if err != nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", err
	}
	receipt, err := e.provisioner.InvokeIntent(ctx, request)
	if err != nil {
		state := "unknown"
		var typed *port.OrcaError
		if errors.As(err, &typed) && !typed.Invoked {
			state = "not_invoked_proven"
		}
		return leasecontract.ReconcileStageReceipt{}, state, err
	}
	var result leasecontract.ReconcileStageReceipt
	if err := reconcileDifferentialConvert(receipt, &result); err != nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", err
	}
	return result, "", nil
}

func (e *reconcileDifferentialEffects) request(intent leaseapp.ReconcileIntentState) (port.ExecutionOrcaIntentRequest, error) {
	state, err := reconcileDifferentialCoreState(leaseoutbound.ReconcileEffectState{
		Record: intent.Progress.Record, RecordRaw: intent.RecordRaw, IntentRaw: intent.IntentRaw,
		OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState,
		InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending, Migrated: intent.Migrated,
	})
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return ExecutionReconcileIntentRequest(state)
}

func reconcileDifferentialEffectState(state ExecutionReconcileIntentState) (leaseoutbound.ReconcileEffectState, error) {
	record, err := reconcileDifferentialContractRecord(state.Record)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return leaseoutbound.ReconcileEffectState{
		Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Stage: string(state.Stage), InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, Pending: state.Pending, Migrated: state.Migrated,
	}, nil
}

func reconcileDifferentialCoreState(state leaseoutbound.ReconcileEffectState) (ExecutionReconcileIntentState, error) {
	record, err := resumeDifferentialCoreRecord(state.Record)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	return ExecutionReconcileIntentState{
		Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Stage: port.ExecutionOrcaIntentStage(state.Stage), InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, Pending: state.Pending, Migrated: state.Migrated,
	}, nil
}

func reconcileDifferentialContractRecord(record IssueOpsRecord) (leasecontract.Record, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return leasecontract.Record{}, err
	}
	return leasecontract.Decode(record.ID, data)
}

func reconcileDifferentialPortReceipt(receipt leasecontract.ReconcileStageReceipt) (port.ExecutionOrcaIntentReceipt, error) {
	var result port.ExecutionOrcaIntentReceipt
	return result, reconcileDifferentialConvert(receipt, &result)
}

func reconcileDifferentialConvert(source, target any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func cloneReconcileState(t *testing.T, stateRoot, id string) string {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution == nil || record.Execution.Pending == nil {
		t.Fatal("pending intent is required")
	}
	source, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	recordRaw, ok, err := source.Get(issueOpsBucket, id)
	if err != nil || !ok {
		t.Fatalf("read record ok=%t err=%v", ok, err)
	}
	intentRaw, ok, err := source.Get(externalIntentBucket, record.Execution.Pending.OperationID)
	if err != nil || !ok {
		t.Fatalf("read intent ok=%t err=%v", ok, err)
	}
	destinationRoot := t.TempDir()
	destination, err := sqlstore.Open(destinationRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.Apply(context.Background(), []sqlstore.Mutation{
		{Bucket: issueOpsBucket, ID: id, Data: recordRaw},
		{Bucket: externalIntentBucket, ID: record.Execution.Pending.OperationID, Data: intentRaw},
	}); err != nil {
		t.Fatal(err)
	}
	return destinationRoot
}

func reconcileDifferentialClock() time.Time {
	return time.Date(2026, time.August, 1, 4, 0, 0, 0, time.UTC)
}
