package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	issueopscontract "issueops/internal/contract/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

func issueOpsReconcileHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReconcileRequest, deps issueops.ExecutionReconcileDependencies) (issueops.ExecutionReconcileResult, error) {
	service := newIssueOpsReconcileService(stateRoot, deps.Orca, deps.ReadIssue, request.Snapshot, deps.Now)
	return leaseinbound.NewReconcileHandler(service)(ctx, stateRoot, request, deps)
}

func newIssueOpsReconcileService(stateRoot string, provisioner port.ExecutionOrcaProvisioner, readIssue issueops.ExecutionIssueSnapshotReadFunc, snapshot *issueopscontract.IssueOpsRecord, now func() time.Time) *leaseapp.ReconcileService {
	effects := &coreReconcileEffects{stateRoot: stateRoot, provisioner: provisioner, readIssue: readIssue, snapshot: snapshot, now: now}
	return leaseapp.NewReconcileService(
		leaseoutbound.NewReconcileRepository(effects),
		leaseoutbound.NewReconcileStageExecutor(effects.inspectStage, effects.invokeStage),
	)
}

type coreReconcileEffects struct {
	stateRoot   string
	provisioner port.ExecutionOrcaProvisioner
	readIssue   issueops.ExecutionIssueSnapshotReadFunc
	snapshot    *issueopscontract.IssueOpsRecord
	now         func() time.Time
}

func (e *coreReconcileEffects) Canonicalize(_ context.Context, id string) (leaseoutbound.ReconcileEffectState, error) {
	state, err := issueops.CanonicalizeExecutionReconcileIntent(e.stateRoot, id, e.snapshot)
	converted, convertErr := reconcileEffectStateFromCore(state)
	if convertErr != nil {
		return leaseoutbound.ReconcileEffectState{}, convertErr
	}
	return converted, err
}

func (e *coreReconcileEffects) MarkInvoking(_ context.Context, state leaseoutbound.ReconcileEffectState) (leaseoutbound.ReconcileEffectState, error) {
	coreState, err := reconcileCoreIntentState(state)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	next, err := issueops.MarkExecutionReconcileIntentInvoking(e.stateRoot, coreState)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return reconcileEffectStateFromCore(next)
}

func (e *coreReconcileEffects) RecordFailure(_ context.Context, state leaseoutbound.ReconcileEffectState, invocation string, cause error) error {
	coreState, err := reconcileCoreIntentState(state)
	if err != nil {
		return err
	}
	return issueops.RecordExecutionReconcileIntentFailure(e.stateRoot, coreState, invocation, cause, e.now)
}

func (e *coreReconcileEffects) ApplyReceipt(ctx context.Context, state leaseoutbound.ReconcileEffectState, receipt leasecontract.ReconcileStageReceipt) (leaseoutbound.ReconcileEffectState, error) {
	coreState, err := reconcileCoreIntentState(state)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	portReceipt, err := reconcilePortReceipt(receipt)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	next, err := issueops.ApplyExecutionReconcileIntentReceipt(ctx, e.stateRoot, coreState, portReceipt, e.readIssue, e.now)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return reconcileEffectStateFromCore(next)
}

func (e *coreReconcileEffects) ClearIntent(_ context.Context, state leaseoutbound.ReconcileEffectState, cause error) (leaseoutbound.ReconcileEffectState, error) {
	coreState, err := reconcileCoreIntentState(state)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	next, err := issueops.ClearExecutionReconcileIntent(e.stateRoot, coreState, cause, e.now)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return reconcileEffectStateFromCore(next)
}

func (e *coreReconcileEffects) Latest(_ context.Context, id string) (leasecontract.Record, error) {
	record, err := issueops.ReadExecutionReconcileRecord(e.stateRoot, id)
	if err != nil {
		return leasecontract.Record{}, err
	}
	return reconcileContractRecord(record)
}

func (e *coreReconcileEffects) inspectStage(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
	if e.provisioner == nil {
		return leasecontract.ReconcileStageInventory{}, false, fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := e.reconcileRequest(intent)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, false, err
	}
	inventory, err := e.provisioner.InspectIntent(ctx, request)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, true, err
	}
	result := leasecontract.ReconcileStageInventory{AuthoritativeZero: inventory.AuthoritativeZero}
	for _, candidate := range inventory.Candidates {
		converted, err := reconcileContractReceipt(candidate)
		if err != nil {
			return leasecontract.ReconcileStageInventory{}, true, err
		}
		result.Candidates = append(result.Candidates, converted)
	}
	return result, true, nil
}

func (e *coreReconcileEffects) invokeStage(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageReceipt, string, error) {
	if e.provisioner == nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := e.reconcileRequest(intent)
	if err != nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", err
	}
	receipt, err := e.provisioner.InvokeIntent(ctx, request)
	if err != nil {
		invocation := "unknown"
		if typed, ok := errors.AsType[*port.OrcaError](err); ok && !typed.Invoked {
			invocation = "not_invoked_proven"
		}
		return leasecontract.ReconcileStageReceipt{}, invocation, err
	}
	converted, err := reconcileContractReceipt(receipt)
	return converted, "", err
}

func (e *coreReconcileEffects) reconcileRequest(intent leaseapp.ReconcileIntentState) (port.ExecutionOrcaIntentRequest, error) {
	state, err := reconcileCoreIntentState(leaseoutbound.ReconcileEffectState{
		Record: intent.Progress.Record, RecordRaw: intent.RecordRaw, IntentRaw: intent.IntentRaw,
		OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState,
		InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending,
	})
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return issueops.ExecutionReconcileIntentRequest(state)
}

func reconcileEffectStateFromCore(state issueops.ExecutionReconcileIntentState) (leaseoutbound.ReconcileEffectState, error) {
	record, err := reconcileContractRecord(state.Record)
	if err != nil {
		return leaseoutbound.ReconcileEffectState{}, err
	}
	return leaseoutbound.ReconcileEffectState{
		Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Stage: string(state.Stage), InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, Pending: state.Pending,
	}, nil
}

func reconcileCoreIntentState(state leaseoutbound.ReconcileEffectState) (issueops.ExecutionReconcileIntentState, error) {
	record, err := resumeCoreRecord(state.Record)
	if err != nil {
		return issueops.ExecutionReconcileIntentState{}, err
	}
	return issueops.ExecutionReconcileIntentState{
		Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Stage: port.ExecutionOrcaIntentStage(state.Stage), InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, Pending: state.Pending,
	}, nil
}

func reconcileContractRecord(record issueopscontract.IssueOpsRecord) (leasecontract.Record, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return leasecontract.Record{}, err
	}
	return leasecontract.Decode(record.ID, data)
}

func reconcileContractReceipt(receipt port.ExecutionOrcaIntentReceipt) (leasecontract.ReconcileStageReceipt, error) {
	return convertReconcileReceipt[leasecontract.ReconcileStageReceipt](receipt)
}

func reconcilePortReceipt(receipt leasecontract.ReconcileStageReceipt) (port.ExecutionOrcaIntentReceipt, error) {
	return convertReconcileReceipt[port.ExecutionOrcaIntentReceipt](receipt)
}

func convertReconcileReceipt[T any](source any) (T, error) {
	var target T
	data, err := json.Marshal(source)
	if err != nil {
		return target, err
	}
	return target, json.Unmarshal(data, &target)
}
