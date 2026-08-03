package issueops

import (
	"context"
	"fmt"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

type ExecutionResumeArtifactsReceipt struct {
	ClaimTokenPath      string
	IssueBodySHA256     string
	ContextPacketPath   string
	ContextPacketSHA256 string
	OwnerPromptPath     string
	OwnerPromptSHA256   string
}

type ExecutionResumeIntentState struct {
	Record             issueops.IssueOpsRecord
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              port.ExecutionOrcaIntentStage
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

func ReadExecutionResumeArtifacts(record issueops.IssueOpsRecord) (ExecutionResumeArtifactsReceipt, error) {
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		return ExecutionResumeArtifactsReceipt{}, err
	}
	return executionResumeArtifactsReceipt(artifacts), nil
}

func NewExecutionResumeOperationID() (string, error) { return newExecutionOperationID() }

func ValidateExecutionResumeOwner(record issueops.IssueOpsRecord, inventory port.ExecutionOrcaOwnerInventory) error {
	return validateExecutionRuntimeRollover(record, inventory)
}

func ExecutionResumeIntentRequest(expected ExecutionResumeIntentState) (port.ExecutionOrcaIntentRequest, error) {
	payload, err := executionResumeIntentPayload(expected)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentExpectedRecord(expected.Record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return executionOrcaIntentRequest(expected.Record, payload)
}

func BeginExecutionResumeIntent(stateRoot string, record issueops.IssueOpsRecord, expectedRecordRaw []byte, artifacts ExecutionResumeArtifactsReceipt, runtimeID, reusedTerminalPTYID, operationID string, now func() time.Time) (ExecutionResumeIntentState, error) {
	persisted, payload, err := beginOrcaExecutionResumeIntentWithExpectedRaw(stateRoot, record, expectedRecordRaw, executionResumeArtifactsFromReceipt(artifacts), runtimeID, reusedTerminalPTYID, operationID, now)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	return executionResumeIntentStateFromPayload(stateRoot, persisted, payload)
}

func ReadExecutionResumeIntent(stateRoot, id, operationID string) (ExecutionResumeIntentState, error) {
	record, raw, err := readExecutionResumeRecordRaw(stateRoot, id)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	payload, intentRaw, err := readExecutionResumeIntentRaw(stateRoot, operationID)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	return executionResumeIntentState(record, raw, payload, intentRaw), nil
}

func MarkExecutionResumeIntentInvoking(stateRoot string, expected ExecutionResumeIntentState) (ExecutionResumeIntentState, error) {
	payload, err := executionResumeIntentPayload(expected)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	if _, err := markOrcaIntentInvokingFromRawState(stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw); err != nil {
		return ExecutionResumeIntentState{}, err
	}
	return ReadExecutionResumeIntent(stateRoot, expected.Record.ID, expected.OperationID)
}

func RecordExecutionResumeIntentFailure(stateRoot string, expected ExecutionResumeIntentState, invocationState string, cause error, now func() time.Time) error {
	payload, err := executionResumeIntentPayload(expected)
	if err != nil {
		return err
	}
	return recordOrcaIntentFailureFromRawState(stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, invocationState, cause, now)
}

func ApplyExecutionResumeIntentReceipt(ctx context.Context, stateRoot string, expected ExecutionResumeIntentState, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) (ExecutionResumeIntentState, error) {
	payload, err := executionResumeIntentPayload(expected)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	persisted, next, err := advanceOrcaIntentReceiptWithExpectedRaw(ctx, stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, receipt, nil, now)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	if persisted.Execution == nil || persisted.Execution.Pending == nil {
		raw, err := readExecutionResumeRecordRawOnly(stateRoot, persisted.ID)
		if err != nil {
			return ExecutionResumeIntentState{}, err
		}
		return ExecutionResumeIntentState{Record: persisted, RecordRaw: raw, OperationID: expected.OperationID}, nil
	}
	return executionResumeIntentStateFromPayload(stateRoot, persisted, next)
}

func executionResumeArtifactsReceipt(artifacts executionResumeArtifacts) ExecutionResumeArtifactsReceipt {
	return ExecutionResumeArtifactsReceipt{ClaimTokenPath: artifacts.claimTokenPath, IssueBodySHA256: artifacts.issueBodySHA256, ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256, OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256}
}

func executionResumeArtifactsFromReceipt(artifacts ExecutionResumeArtifactsReceipt) executionResumeArtifacts {
	return executionResumeArtifacts{claimTokenPath: artifacts.ClaimTokenPath, issueBodySHA256: artifacts.IssueBodySHA256, packetPath: artifacts.ContextPacketPath, packetSHA256: artifacts.ContextPacketSHA256, promptPath: artifacts.OwnerPromptPath, promptSHA256: artifacts.OwnerPromptSHA256}
}

func executionResumeIntentStateFromPayload(stateRoot string, record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (ExecutionResumeIntentState, error) {
	raw, err := readExecutionResumeRecordRawOnly(stateRoot, record.ID)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	intentRaw, err := preparationIntentCodec.Encode(payload)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	return executionResumeIntentState(record, raw, payload, intentRaw), nil
}

func executionResumeIntentState(record issueops.IssueOpsRecord, recordRaw []byte, payload externalOrcaIntentPayload, intentRaw []byte) ExecutionResumeIntentState {
	return ExecutionResumeIntentState{Record: record, RecordRaw: append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...), OperationID: payload.OperationID, Stage: intentPortStage(payload.Stage), InvocationState: payload.InvocationState, InvocationAttempts: payload.InvocationAttempts, Pending: record.Execution != nil && record.Execution.Pending != nil}
}

func executionResumeIntentPayload(expected ExecutionResumeIntentState) (externalOrcaIntentPayload, error) {
	return preparationIntentCodec.Decode(expected.OperationID, expected.IntentRaw)
}

func readExecutionResumeRecordRaw(stateRoot, id string) (issueops.IssueOpsRecord, []byte, error) {
	raw, err := readExecutionResumeRecordRawOnly(stateRoot, id)
	if err != nil {
		return issueops.IssueOpsRecord{}, nil, err
	}
	record, err := decodeIssueOpsRecord(id, raw)
	if err != nil {
		return issueops.IssueOpsRecord{}, nil, err
	}
	if err := validateIssueOpsRecord(record); err != nil {
		return issueops.IssueOpsRecord{}, nil, err
	}
	return record, raw, nil
}

func readExecutionResumeRecordRawOnly(stateRoot, id string) ([]byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	raw, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("issueops record %s not found", id)
	}
	return raw, nil
}

func readExecutionResumeIntentRaw(stateRoot, operationID string) (externalOrcaIntentPayload, []byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalOrcaIntentPayload{}, nil, err
	}
	raw, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return externalOrcaIntentPayload{}, nil, err
	}
	if !ok {
		return externalOrcaIntentPayload{}, nil, fmt.Errorf("Orca external intent payload is missing")
	}
	payload, err := preparationIntentCodec.Decode(operationID, raw)
	if err != nil {
		return externalOrcaIntentPayload{}, nil, err
	}
	return payload, raw, nil
}
