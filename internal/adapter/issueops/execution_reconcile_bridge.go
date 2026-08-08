package issueops

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// ExecutionReconcileIntentState는 새 reconcile vertical이 기존 durable CAS
// primitive를 호출할 때 필요한 최소 호환 상태다.
type ExecutionReconcileIntentState struct {
	Record             issueops.IssueOpsRecord
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              port.ExecutionOrcaIntentStage
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

func CanonicalizeExecutionReconcileIntent(stateRoot, id string, snapshot *issueops.IssueOpsRecord) (ExecutionReconcileIntentState, error) {
	var record issueops.IssueOpsRecord
	if snapshot == nil {
		var err error
		record, err = ReadIssueOps(stateRoot, id)
		if err != nil {
			return ExecutionReconcileIntentState{Record: issueops.IssueOpsRecord{ID: id}}, err
		}
	} else {
		record = *snapshot
		if record.ID != id {
			return ExecutionReconcileIntentState{Record: record}, fmt.Errorf("reconcile snapshot ID changed before canonicalization")
		}
	}
	persisted, payload, err := reconcileCanonicalOrcaIntent(stateRoot, record)
	if err != nil {
		return ExecutionReconcileIntentState{Record: persisted}, err
	}
	return executionReconcileIntentStateFromPayload(stateRoot, persisted, payload)
}

func ExecutionReconcileIntentRequest(expected ExecutionReconcileIntentState) (port.ExecutionOrcaIntentRequest, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentExpectedRecord(expected.Record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return executionOrcaIntentRequest(expected.Record, payload)
}

func MarkExecutionReconcileIntentInvoking(stateRoot string, expected ExecutionReconcileIntentState) (ExecutionReconcileIntentState, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	if _, err := markOrcaIntentInvokingFromRawState(stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw); err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	return readExecutionReconcileIntent(stateRoot, expected.Record.ID, expected.OperationID)
}

func RecordExecutionReconcileIntentFailure(stateRoot string, expected ExecutionReconcileIntentState, invocationState string, cause error, now func() time.Time) error {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return err
	}
	return recordOrcaIntentFailureFromRawState(stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, invocationState, cause, now)
}

func ApplyExecutionReconcileIntentReceipt(ctx context.Context, stateRoot string, expected ExecutionReconcileIntentState, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFunc, now func() time.Time) (ExecutionReconcileIntentState, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	persisted, nextPayload, err := advanceOrcaIntentReceiptWithExpectedRaw(ctx, stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, receipt, readIssue, now)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	if persisted.Execution == nil || persisted.Execution.Pending == nil {
		raw, err := readExecutionResumeRecordRawOnly(stateRoot, persisted.ID)
		if err != nil {
			return ExecutionReconcileIntentState{}, err
		}
		return ExecutionReconcileIntentState{Record: persisted, RecordRaw: raw, OperationID: expected.OperationID}, nil
	}
	return executionReconcileIntentStateFromPayload(stateRoot, persisted, nextPayload)
}

func ReadExecutionReconcileRecord(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	return ReadIssueOps(stateRoot, id)
}

func readExecutionReconcileIntent(stateRoot, id, operationID string) (ExecutionReconcileIntentState, error) {
	record, recordRaw, err := readExecutionResumeRecordRaw(stateRoot, id)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	payload, intentRaw, err := readExecutionResumeIntentRaw(stateRoot, operationID)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	return executionReconcileIntentState(record, recordRaw, payload, intentRaw), nil
}

func executionReconcileIntentStateFromPayload(stateRoot string, record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (ExecutionReconcileIntentState, error) {
	partial := executionReconcileIntentState(record, nil, payload, nil)
	currentRecord, recordRaw, err := readExecutionResumeRecordRaw(stateRoot, record.ID)
	if err != nil {
		return partial, err
	}
	if !reflect.DeepEqual(currentRecord, record) {
		return partial, fmt.Errorf("IssueOps record snapshot changed before reconcile raw capture")
	}
	currentPayload, intentRaw, err := readExecutionResumeIntentRaw(stateRoot, payload.OperationID)
	if err != nil {
		return partial, err
	}
	if !reflect.DeepEqual(currentPayload, payload) {
		return partial, fmt.Errorf("Orca intent snapshot changed before reconcile raw capture")
	}
	return executionReconcileIntentState(record, recordRaw, payload, intentRaw), nil
}

func executionReconcileIntentState(record issueops.IssueOpsRecord, recordRaw []byte, payload externalOrcaIntentPayload, intentRaw []byte) ExecutionReconcileIntentState {
	return ExecutionReconcileIntentState{
		Record: record, RecordRaw: append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...),
		OperationID: payload.OperationID, Stage: intentPortStage(payload.Stage), InvocationState: payload.InvocationState,
		InvocationAttempts: payload.InvocationAttempts, Pending: record.Execution != nil && record.Execution.Pending != nil,
	}
}

func executionReconcileIntentPayload(expected ExecutionReconcileIntentState) (externalOrcaIntentPayload, error) {
	return executionResumeIntentPayload(ExecutionResumeIntentState{
		Record: expected.Record, RecordRaw: expected.RecordRaw, IntentRaw: expected.IntentRaw,
		OperationID: expected.OperationID, Stage: expected.Stage, InvocationState: expected.InvocationState,
		InvocationAttempts: expected.InvocationAttempts, Pending: expected.Pending,
	})
}

// ClearExecutionReconcileIntent는 authoritative zero로 확인된 reconcile intent를
// 제거한다(#280). 재시도가 아니라 기록 정리이므로 stage를 전진시키지 않는다.
func ClearExecutionReconcileIntent(stateRoot string, expected ExecutionReconcileIntentState, cause error, now func() time.Time) (ExecutionReconcileIntentState, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	record, err := ClearOrcaIntentWithNoResource(stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, cause, now)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	cleared := expected
	cleared.Record = record
	cleared.RecordRaw, cleared.IntentRaw = nil, nil
	cleared.Pending = false
	return cleared, nil
}
