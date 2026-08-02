package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const (
	orcaIntentNotInvoked  = "not_invoked_proven"
	orcaIntentUnknown     = "unknown"
	maxOrcaIntentAttempts = 2

	orcaIntentPurposePrepare = "prepare"
	orcaIntentPurposeResume  = "resume"
)

type externalOrcaLaunchIdentity = preparationcontract.LaunchIdentity
type externalOrcaIntentPayload = preparationcontract.Intent

var preparationIntentCodec preparationcontract.IntentCodec

func beginOrcaExecutionIntent(stateRoot string, record issueops.IssueOpsRecord, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, req ExecutionPrepareRequest, snapshot executionOwnerSnapshot, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	return beginOrcaExecutionIntentWithID(stateRoot, record, workspace, probe, req, snapshot, "", now)
}

func beginOrcaExecutionIntentWithID(stateRoot string, record issueops.IssueOpsRecord, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, req ExecutionPrepareRequest, snapshot executionOwnerSnapshot, operationID string, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	var err error
	if strings.TrimSpace(operationID) == "" {
		operationID, err = newExecutionOperationID()
		if err != nil {
			return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
		}
	}
	startedAt := executionNow(now)
	payload := externalOrcaIntentPayload{
		SchemaVersion: issueops.IssueOpsSchemaVersion, OperationID: operationID, LifecycleID: record.ID,
		Generation: 1, Stage: preparationcontract.IntentStageWorktree, StartedAt: startedAt,
		Purpose: orcaIntentPurposePrepare, InvocationState: orcaIntentNotInvoked,
		Workspace: intentContractWorkspaceRequest(workspace), Probe: intentContractProbeRequest(probe),
		IssueBodySHA256: snapshot.issue.BodySHA256,
	}
	if strings.ToLower(strings.TrimSpace(req.OwnerHost)) != probe.Host || strings.TrimSpace(req.OwnerModel) != probe.Model || strings.TrimSpace(req.OwnerEffort) != probe.Effort {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, fmt.Errorf("owner profile changed before Orca intent persistence")
	}
	payload, err = sealExternalOrcaPrepareIntentPayload(record, payload)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	data, err := preparationIntentCodec.Encode(payload)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	var persisted issueops.IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return err
		}
		if current.Execution != nil {
			return fmt.Errorf("IssueOps execution already exists; reconcile or inspect its current state")
		}
		if err := validateOrcaIntentIssueIdentity(current, payload); err != nil {
			return err
		}
		// 위 검사는 자기 ID의 Execution만 본다 — 다른 레코드가 같은 canonical
		// root를 주장하는 레코드 수준 레이스는 이 임계구역 재검사로만 봉합된다.
		if err := ensureExecutionRootUnclaimed(stateRoot, current.ID, workspace.Root); err != nil {
			return err
		}
		current.Execution = &issueops.Execution{
			Mode: issueops.ExecutionModeOrca,
			Workspace: issueops.Workspace{
				SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
				BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree,
				Driver: "orca", LinkedAt: startedAt,
			},
			Lease: issueops.WriteLease{Generation: payload.Generation, Status: issueops.LeaseStatusReleased},
			Pending: &issueops.ExternalIntent{
				OperationID: operationID, Kind: string(port.ExecutionOrcaIntentWorktree), Marker: payload.Marker, StartedAt: startedAt,
			},
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{
			Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func executeOrcaIntentStage(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, payload externalOrcaIntentPayload, orca port.ExecutionOrcaProvisioner, readIssue ExecutionIssueSnapshotReadFunc, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	if orca == nil {
		return record, payload, fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := executionOrcaIntentRequest(record, payload)
	if err != nil {
		return record, payload, err
	}
	inventory, err := orca.InspectIntent(ctx, request)
	if err != nil {
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
	}
	if len(inventory.Candidates) > 1 {
		err := fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if len(inventory.Candidates) == 1 {
		updated, next, advanceErr := advanceOrcaIntentReceipt(ctx, stateRoot, record, payload, inventory.Candidates[0], readIssue, now)
		if advanceErr != nil {
			_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, orcaIntentUnknown, advanceErr, now)
		}
		return updated, next, advanceErr
	}
	if !inventory.AuthoritativeZero {
		err := fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if payload.InvocationState != orcaIntentNotInvoked && payload.Stage != preparationcontract.IntentStageRunBind {
		err := fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if payload.InvocationAttempts >= maxOrcaIntentAttempts {
		err := fmt.Errorf("Orca intent retry is exhausted; intent retained")
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	payload, err = markOrcaIntentInvoking(stateRoot, record.ID, payload)
	if err != nil {
		return record, payload, err
	}
	receipt, invokeErr := orca.InvokeIntent(ctx, request)
	if invokeErr != nil {
		invocation := orcaIntentUnknown
		var typed *port.OrcaError
		if errors.As(invokeErr, &typed) && !typed.Invoked {
			invocation = orcaIntentNotInvoked
		}
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, invocation, invokeErr, now)
		return record, payload, fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", invokeErr)
	}
	updated, next, advanceErr := advanceOrcaIntentReceipt(ctx, stateRoot, record, payload, receipt, readIssue, now)
	if advanceErr != nil {
		_ = recordOrcaIntentFailure(stateRoot, record.ID, payload, orcaIntentUnknown, advanceErr, now)
	}
	return updated, next, advanceErr
}

func markOrcaIntentInvoking(stateRoot, id string, expected externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	return markOrcaIntentInvokingWithExpectedRaw(stateRoot, id, expected, nil, nil)
}

func markOrcaIntentInvokingWithExpectedRaw(stateRoot, id string, expected externalOrcaIntentPayload, expectedRecordRaw, expectedIntentRaw []byte) (externalOrcaIntentPayload, error) {
	updated := expected
	updated.InvocationState = orcaIntentUnknown
	updated.InvocationAttempts++
	data, err := preparationIntentCodec.Encode(updated)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, stored, err := readAndMatchOrcaIntent(stateRoot, id, expected)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("Orca intent payload changed before invocation CAS")
		}
		_, err = persistOrcaIntentTransition(stateRoot, record, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func markOrcaIntentInvokingFromRawState(stateRoot string, record issueops.IssueOpsRecord, expected externalOrcaIntentPayload, expectedRecordRaw, expectedIntentRaw []byte) (externalOrcaIntentPayload, error) {
	updated := expected
	updated.InvocationState = orcaIntentUnknown
	updated.InvocationAttempts++
	data, err := preparationIntentCodec.Encode(updated)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		if err := validateOrcaIntentExpectedRecord(record, expected); err != nil {
			return err
		}
		_, err := persistOrcaIntentTransition(stateRoot, record, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func recordOrcaIntentFailure(stateRoot, id string, expected externalOrcaIntentPayload, invocation string, cause error, now func() time.Time) error {
	return recordOrcaIntentFailureWithExpectedRaw(stateRoot, id, expected, nil, nil, invocation, cause, now)
}

func recordOrcaIntentFailureWithExpectedRaw(stateRoot, id string, expected externalOrcaIntentPayload, expectedRecordRaw, expectedIntentRaw []byte, invocation string, cause error, now func() time.Time) error {
	return withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, stored, err := readAndMatchOrcaIntent(stateRoot, id, expected)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("Orca intent payload changed before failure receipt")
		}
		stored.InvocationState = invocation
		data, err := preparationIntentCodec.Encode(stored)
		if err != nil {
			return err
		}
		message := boundedExecutionRemoteDiagnostic(cause)
		record.Execution.Failure = &issueops.ExecutionFailure{
			OperationID: stored.OperationID, Code: "external_operation_ambiguous", Message: message, At: executionNow(now),
		}
		_, err = persistOrcaIntentTransition(stateRoot, record, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: stored.OperationID, Data: data}})
		return err
	})
}

func recordOrcaIntentFailureFromRawState(stateRoot string, record issueops.IssueOpsRecord, expected externalOrcaIntentPayload, expectedRecordRaw, expectedIntentRaw []byte, invocation string, cause error, now func() time.Time) error {
	return withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		if err := validateOrcaIntentExpectedRecord(record, expected); err != nil {
			return err
		}
		next := record
		next.Execution.Failure = &issueops.ExecutionFailure{OperationID: expected.OperationID, Code: "external_operation_ambiguous", Message: boundedExecutionRemoteDiagnostic(cause), At: executionNow(now)}
		expected.InvocationState = invocation
		data, err := preparationIntentCodec.Encode(expected)
		if err != nil {
			return err
		}
		_, err = persistOrcaIntentTransition(stateRoot, next, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return err
	})
}

func advanceOrcaIntentReceipt(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, expected externalOrcaIntentPayload, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFunc, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	return advanceOrcaIntentReceiptWithExpectedRaw(ctx, stateRoot, record, expected, nil, nil, receipt, readIssue, now)
}

func advanceOrcaIntentReceiptWithExpectedRaw(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, expected externalOrcaIntentPayload, expectedRecordRaw, expectedIntentRaw []byte, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFunc, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	updated := expected
	updated.InvocationState = orcaIntentNotInvoked
	updated.InvocationAttempts = 0
	switch expected.Stage {
	case preparationcontract.IntentStageWorktree:
		if receipt.Workspace == nil || validateExecutionOrcaWorkspaceReceipt(intentPortWorkspaceRequest(expected.Workspace), *receipt.Workspace) != nil {
			return record, expected, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
		}
		prepared := record
		prepared.WorktreePath = receipt.Workspace.Workspace.Root
		prepared.Execution.Workspace = workspaceFromReceipt(receipt.Workspace.Workspace, expected.StartedAt)
		snapshot, err := readExecutionOwnerSnapshot(ctx, prepared, readIssue)
		if err != nil {
			return record, expected, err
		}
		if snapshot.issue.BodySHA256 != expected.IssueBodySHA256 {
			return record, expected, fmt.Errorf("remote issue body drifted before owner launch recovery")
		}
		tokenHash, err := createOrAdoptClaimToken(prepared)
		if err != nil {
			return record, expected, err
		}
		artifactManifest, err := materializeStagedArtifacts(stateRoot, prepared)
		if err != nil {
			return record, expected, err
		}
		artifacts, err := buildExecutionOwnerArtifacts(prepared, ExecutionPrepareRequest{
			ID: prepared.ID, Mode: string(issueops.ExecutionModeOrca), OwnerHost: expected.Probe.Host,
			OwnerModel: expected.Probe.Model, OwnerEffort: expected.Probe.Effort,
		}, snapshot, artifactManifest)
		if err != nil {
			return record, expected, err
		}
		updated.Prepared = intentContractOrcaWorkspaceReceiptPointer(receipt.Workspace)
		updated.Launch = &externalOrcaLaunchIdentity{
			PromptPath: artifacts.promptPath, PromptSHA256: artifacts.promptSHA256,
			ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		}
		updated.ClaimTokenSHA256 = tokenHash
		updated.Stage = preparationcontract.IntentStageTerminal
	case preparationcontract.IntentStageTerminal:
		if strings.TrimSpace(receipt.TerminalPTYID) == "" {
			return record, expected, fmt.Errorf("Orca terminal candidate is incomplete")
		}
		updated.TerminalPTYID = strings.TrimSpace(receipt.TerminalPTYID)
		updated.Stage = preparationcontract.IntentStageRun
	case preparationcontract.IntentStageRun:
		if strings.TrimSpace(receipt.RunID) == "" {
			return record, expected, fmt.Errorf("Orca Run candidate is incomplete")
		}
		updated.RunID = strings.TrimSpace(receipt.RunID)
		updated.Stage = preparationcontract.IntentStageRunBind
	case preparationcontract.IntentStageRunBind:
		if strings.TrimSpace(receipt.RunID) != expected.RunID || !receipt.RunBound {
			return record, expected, fmt.Errorf("Orca Run binding candidate is incomplete")
		}
		updated.RunBound = true
		updated.Stage = preparationcontract.IntentStageTask
	case preparationcontract.IntentStageTask:
		if strings.TrimSpace(receipt.TaskID) == "" {
			return record, expected, fmt.Errorf("Orca task candidate is incomplete")
		}
		updated.TaskID = strings.TrimSpace(receipt.TaskID)
		updated.Stage = preparationcontract.IntentStageDispatch
	case preparationcontract.IntentStageDispatch:
		if strings.TrimSpace(receipt.TaskID) != expected.TaskID || strings.TrimSpace(receipt.DispatchID) == "" {
			return record, expected, fmt.Errorf("Orca dispatch candidate is incomplete")
		}
	default:
		return record, expected, fmt.Errorf("unsupported Orca intent stage %q", expected.Stage)
	}

	var persisted issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, stored := record, expected
		if expectedRecordRaw == nil && expectedIntentRaw == nil {
			var err error
			current, stored, err = readAndMatchOrcaIntent(stateRoot, record.ID, expected)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(stored, expected) {
				return fmt.Errorf("Orca intent payload changed before receipt CAS")
			}
		} else if err := validateOrcaIntentExpectedRecord(current, expected); err != nil {
			return err
		}
		if expected.Stage == preparationcontract.IntentStageWorktree {
			current.WorktreePath = updated.Prepared.Workspace.Root
			current.Execution.Workspace = workspaceFromReceipt(intentPortWorkspaceReceipt(updated.Prepared.Workspace), expected.StartedAt)
		}
		if expected.Stage == preparationcontract.IntentStageDispatch {
			if normalizedOrcaIntentPurpose(expected) == orcaIntentPurposeResume {
				if expected.ResumeLease == nil || !reflect.DeepEqual(intentContractLease(current.Execution.Lease), *expected.ResumeLease) {
					return fmt.Errorf("resume lease changed before dispatch receipt CAS")
				}
			} else {
				current.Execution.Lease = issueops.WriteLease{
					Generation: expected.Generation, Status: issueops.LeaseStatusClaimable, ClaimTokenSHA256: expected.ClaimTokenSHA256,
				}
			}
			current.Execution.Orca = &issueops.OrcaBinding{
				RuntimeID: expected.Prepared.RuntimeID, RepoID: expected.Prepared.RepoID, WorktreeID: expected.Prepared.WorktreeID,
				WorktreeInstanceID: expected.Prepared.WorktreeInstanceID, LeaseGeneration: expected.Generation, OwnerHost: expected.Probe.Host,
				OwnerModel: expected.Probe.Model, OwnerEffort: expected.Probe.Effort, RunID: expected.RunID, TaskID: expected.TaskID,
				DispatchID: receipt.DispatchID, TerminalPTYID: expected.TerminalPTYID,
			}
			current.Execution.Pending = nil
			current.Execution.Failure = nil
			var persistErr error
			persisted, persistErr = persistOrcaIntentTransition(stateRoot, current, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Delete: true}})
			return persistErr
		}
		current.Execution.Pending.Kind = pendingKindForOrcaStage(updated.Stage)
		current.Execution.Failure = nil
		data, err := preparationIntentCodec.Encode(updated)
		if err != nil {
			return err
		}
		var persistErr error
		persisted, persistErr = persistOrcaIntentTransition(stateRoot, current, expected.OperationID, expectedRecordRaw, expectedIntentRaw, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return persistErr
	})
	if err != nil {
		return record, expected, err
	}
	return persisted, updated, nil
}

func persistOrcaIntentTransition(stateRoot string, record issueops.IssueOpsRecord, operationID string, expectedRecordRaw, expectedIntentRaw []byte, extra []sqlstore.Mutation) (issueops.IssueOpsRecord, error) {
	if expectedRecordRaw == nil && expectedIntentRaw == nil {
		return persistExecutionTransitionWithMutations(stateRoot, record, nil, extra)
	}
	expected := []sqlstore.ExpectedRecord{}
	if expectedRecordRaw != nil {
		expected = append(expected, sqlstore.ExpectedRecord{Bucket: issueOpsBucket, ID: record.ID, Data: expectedRecordRaw})
	}
	if expectedIntentRaw != nil {
		expected = append(expected, sqlstore.ExpectedRecord{Bucket: externalIntentBucket, ID: operationID, Data: expectedIntentRaw})
	}
	return persistExecutionTransitionWithRawCAS(stateRoot, record, expected, extra)
}

func readAndMatchOrcaIntent(stateRoot, id string, expected externalOrcaIntentPayload) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return issueops.IssueOpsRecord{}, externalOrcaIntentPayload{}, err
	}
	if err := validateOrcaIntentExpectedRecord(record, expected); err != nil {
		return record, externalOrcaIntentPayload{}, err
	}
	stored, err := readExternalOrcaIntentPayload(stateRoot, expected.OperationID)
	return record, stored, err
}

func validateOrcaIntentExpectedRecord(record issueops.IssueOpsRecord, expected externalOrcaIntentPayload) error {
	if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != expected.OperationID ||
		record.Execution.Pending.Marker != expected.Marker || record.Execution.Pending.Kind != pendingKindForOrcaStage(expected.Stage) ||
		record.Execution.Lease.Generation != expected.Generation {
		return fmt.Errorf("Orca intent authority changed before CAS")
	}
	switch normalizedOrcaIntentPurpose(expected) {
	case orcaIntentPurposePrepare:
		if record.Execution.Lease.Status != issueops.LeaseStatusReleased || record.Execution.Orca != nil {
			return fmt.Errorf("Orca prepare intent authority changed before CAS")
		}
	case orcaIntentPurposeResume:
		if expected.ResumeLease == nil || expected.PriorBinding == nil ||
			!reflect.DeepEqual(intentContractLease(record.Execution.Lease), *expected.ResumeLease) ||
			!reflect.DeepEqual(intentContractBindingPointer(record.Execution.Orca), expected.PriorBinding) {
			return fmt.Errorf("Orca resume intent authority changed before CAS")
		}
	default:
		return fmt.Errorf("unsupported Orca intent purpose")
	}
	return validateOrcaIntentRecordIdentity(record, expected)
}

func readExternalOrcaIntentPayload(stateRoot, operationID string) (externalOrcaIntentPayload, error) {
	payload, err := readExternalOrcaIntentPayloadShape(stateRoot, operationID)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	if err := validateExternalOrcaIntentPayload(payload, operationID); err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return payload, nil
}

func readExternalOrcaIntentPayloadShape(stateRoot, operationID string) (externalOrcaIntentPayload, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	if !ok {
		return externalOrcaIntentPayload{}, fmt.Errorf("Orca external intent payload is missing")
	}
	return preparationIntentCodec.DecodeShape(operationID, data)
}

func reconcileCanonicalOrcaIntent(
	stateRoot string,
	expected issueops.IssueOpsRecord,
) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	if expected.Execution == nil || expected.Execution.Pending == nil {
		return expected, externalOrcaIntentPayload{}, newOrcaIntentContractError("orca_intent_invalid", "Orca pending intent is missing")
	}
	var (
		persisted issueops.IssueOpsRecord
		intent    externalOrcaIntentPayload
	)
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		persisted = current
		if current.Execution == nil || current.Execution.Pending == nil ||
			!reflect.DeepEqual(current.Execution, expected.Execution) ||
			current.IssueURL != expected.IssueURL ||
			!reflect.DeepEqual(current.BranchPrepare, expected.BranchPrepare) {
			return newOrcaIntentContractError("orca_intent_authority_changed", "Orca intent authority changed before canonicalization")
		}
		pending := current.Execution.Pending
		db, err := sqlstore.Open(stateRoot)
		if err != nil {
			return err
		}
		raw, ok, err := db.Get(externalIntentBucket, pending.OperationID)
		if err != nil {
			return err
		}
		if !ok {
			return newOrcaIntentContractError("orca_intent_invalid", "Orca external intent payload is missing")
		}
		contractRaw, err := json.Marshal(current)
		if err != nil {
			return err
		}
		contractRecord, err := leasecontract.Decode(current.ID, contractRaw)
		if err != nil {
			return err
		}
		intent, _, err = preparationIntentCodec.Canonicalize(contractRecord, raw)
		if err != nil {
			return err
		}
		if err := validateOrcaIntentRecordIdentity(current, intent); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return persisted, intent, err
	}
	return persisted, intent, nil
}

func validateExternalOrcaIntentPayload(payload externalOrcaIntentPayload, operationID string) error {
	return preparationIntentCodec.Validate(payload, operationID)
}

func validateExternalOrcaIntentPayloadShape(payload externalOrcaIntentPayload, operationID string) error {
	return preparationIntentCodec.ValidateShape(payload, operationID)
}

func normalizedOrcaIntentPurpose(payload externalOrcaIntentPayload) string {
	if strings.TrimSpace(payload.Purpose) == "" {
		return orcaIntentPurposePrepare
	}
	return strings.TrimSpace(payload.Purpose)
}

func executionOrcaIntentRequest(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (port.ExecutionOrcaIntentRequest, error) {
	request, err := executionOrcaIntentInspectionRequest(record, payload)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if payload.Launch == nil {
		return request, nil
	}
	token, err := readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil || tokenSHA256(token) != payload.ClaimTokenSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed claim token identity changed")
	}
	prompt, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, payload.Launch.PromptPath)
	if err != nil || digestExecutionOwnerBytes(prompt) != payload.Launch.PromptSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	packet, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, payload.Launch.ContextPacketPath)
	if err != nil || digestExecutionOwnerBytes(packet) != payload.Launch.ContextPacketSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed context packet identity changed")
	}
	request.Launch.Prompt = string(prompt)
	return request, nil
}

// executionOrcaIntentInspectionRequest는 persisted payload의 봉인 메타데이터만
// 사용해 read-only Orca 인벤토리 요청을 만든다. 삭제된 worktree의 token과
// artifact 파일은 mutation 재시도에만 필요하며, 이미 사라진 외부 자원을
// 확인하는 조회의 전제 조건이 아니다.
func executionOrcaIntentInspectionRequest(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (port.ExecutionOrcaIntentRequest, error) {
	if err := validateExternalOrcaIntentPayload(payload, payload.OperationID); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentRecordIdentity(record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	request := port.ExecutionOrcaIntentRequest{
		Stage: intentPortStage(payload.Stage), Marker: payload.Marker,
		Workspace: intentPortWorkspaceRequest(payload.Workspace), Probe: intentPortProbeRequest(payload.Probe),
		Prepared: intentPortOrcaWorkspaceReceiptPointer(payload.Prepared), TerminalPTYID: payload.TerminalPTYID,
		RunID: payload.RunID, RunBound: payload.RunBound, TaskID: payload.TaskID,
	}
	if payload.Launch != nil {
		expectedPacketPath, expectedPromptPath := executionOwnerArtifactPaths(record)
		if !samePath(payload.Launch.PromptPath, expectedPromptPath) || !samePath(payload.Launch.ContextPacketPath, expectedPacketPath) {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner artifact path changed")
		}
		request.Launch = &port.ExecutionOrcaLaunchRequest{
			PromptPath: payload.Launch.PromptPath, PromptSHA256: payload.Launch.PromptSHA256,
			ContextPacketPath: payload.Launch.ContextPacketPath, ContextPacketSHA256: payload.Launch.ContextPacketSHA256,
		}
	}
	return request, nil
}

func validateOrcaIntentRecordIdentity(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) error {
	if err := validateOrcaIntentIssueIdentity(record, payload); err != nil {
		return err
	}
	if record.ID != payload.LifecycleID || record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca ||
		!samePath(record.Execution.Workspace.SourceRoot, payload.Workspace.SourceRoot) || !samePath(record.Execution.Workspace.Root, payload.Workspace.Root) ||
		record.Execution.Workspace.Branch != payload.Workspace.Branch || record.Execution.Workspace.BaseHead != payload.Workspace.BaseHead ||
		!sameOptionalExecutionPath(record.Execution.Workspace.ParentWorktree, payload.Workspace.ParentWorktree) ||
		record.Execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca intent record identity changed")
	}
	if payload.Prepared != nil && (!samePath(record.WorktreePath, payload.Prepared.Workspace.Root) ||
		!samePath(record.Execution.Workspace.Root, payload.Prepared.Workspace.Root) || record.Execution.Workspace.Branch != payload.Prepared.Workspace.Branch ||
		record.Execution.Workspace.BaseHead != payload.Prepared.Workspace.BaseHead ||
		!sameOptionalExecutionPath(record.Execution.Workspace.ParentWorktree, payload.Prepared.Workspace.ParentWorktree)) {
		return fmt.Errorf("Orca prepared workspace identity changed")
	}
	return nil
}

func sameOptionalExecutionPath(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	return samePath(left, right)
}

func pendingKindForOrcaStage(stage preparationcontract.IntentStage) string {
	switch stage {
	case preparationcontract.IntentStageWorktree:
		return "worktree_create"
	case preparationcontract.IntentStageTerminal, preparationcontract.IntentStageRun, preparationcontract.IntentStageRunBind, preparationcontract.IntentStageTask:
		return "owner_launch"
	case preparationcontract.IntentStageDispatch:
		return "dispatch"
	default:
		return ""
	}
}

func createOrAdoptClaimToken(record issueops.IssueOpsRecord) (string, error) {
	token, _, err := createClaimToken(record)
	if err == nil {
		return tokenSHA256(token), nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	token, err = readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil {
		return "", fmt.Errorf("recover deterministic claim token: %w", err)
	}
	return tokenSHA256(token), nil
}

func intentContractLease(lease issueops.WriteLease) leasecontract.Lease {
	result := leasecontract.Lease{
		Generation: lease.Generation, Status: string(lease.Status), ClaimTokenSHA256: lease.ClaimTokenSHA256,
		ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt,
		ReplacementReason: lease.ReplacementReason,
	}
	if lease.Holder != nil {
		result.Holder = &leasecontract.Actor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			result.Holder.SessionProcess = &leasecontract.ProcessReceipt{
				PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt,
				Executable: lease.Holder.SessionProcess.Executable,
			}
		}
	}
	return result
}

func intentContractBinding(binding issueops.OrcaBinding) preparationcontract.ResumeBinding {
	return preparationcontract.ResumeBinding{
		RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
		WorktreeInstanceID: binding.WorktreeInstanceID, LeaseGeneration: binding.LeaseGeneration,
		OwnerHost: binding.OwnerHost, OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
		RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
	}
}

func intentContractBindingPointer(binding *issueops.OrcaBinding) *preparationcontract.ResumeBinding {
	if binding == nil {
		return nil
	}
	result := intentContractBinding(*binding)
	return &result
}

func intentContractWorkspaceRequest(workspace port.ExecutionWorkspaceRequest) preparationcontract.WorkspaceRequest {
	return preparationcontract.WorkspaceRequest{
		LifecycleID: workspace.LifecycleID, SourceRoot: workspace.SourceRoot, Root: workspace.Root,
		Branch: workspace.Branch, BaseBranch: workspace.BaseBranch, BaseHead: workspace.BaseHead,
		ParentWorktree: workspace.ParentWorktree, Confirm: workspace.Confirm,
	}
}

func intentPortWorkspaceRequest(workspace preparationcontract.WorkspaceRequest) port.ExecutionWorkspaceRequest {
	return port.ExecutionWorkspaceRequest{
		LifecycleID: workspace.LifecycleID, SourceRoot: workspace.SourceRoot, Root: workspace.Root,
		Branch: workspace.Branch, BaseBranch: workspace.BaseBranch, BaseHead: workspace.BaseHead,
		ParentWorktree: workspace.ParentWorktree, Confirm: workspace.Confirm,
	}
}

func intentContractProbeRequest(probe port.ExecutionOrcaProbeRequest) preparationcontract.ProbeRequest {
	return preparationcontract.ProbeRequest{
		Repo: probe.Repo, Host: probe.Host, Model: probe.Model, Effort: probe.Effort,
		Provider: probe.Provider, Issue: probe.Issue, Marker: probe.Marker,
	}
}

func intentPortProbeRequest(probe preparationcontract.ProbeRequest) port.ExecutionOrcaProbeRequest {
	return port.ExecutionOrcaProbeRequest{
		Repo: probe.Repo, Host: probe.Host, Model: probe.Model, Effort: probe.Effort,
		Provider: probe.Provider, Issue: probe.Issue, Marker: probe.Marker,
	}
}

func intentContractOrcaWorkspaceReceiptPointer(receipt *port.ExecutionOrcaWorkspaceReceipt) *preparationcontract.OrcaWorkspaceReceipt {
	if receipt == nil {
		return nil
	}
	result := preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{
			SourceRoot: receipt.Workspace.SourceRoot, Root: receipt.Workspace.Root, Branch: receipt.Workspace.Branch,
			BaseHead: receipt.Workspace.BaseHead, ParentWorktree: receipt.Workspace.ParentWorktree,
			Driver: receipt.Workspace.Driver, Exists: receipt.Workspace.Exists,
		},
		RuntimeID: receipt.RuntimeID, RepoID: receipt.RepoID, WorktreeID: receipt.WorktreeID,
		WorktreeInstanceID: receipt.WorktreeInstanceID,
	}
	return &result
}

func intentPortWorkspaceReceipt(receipt preparationcontract.WorkspaceReceipt) port.ExecutionWorkspaceReceipt {
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead,
		ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver, Exists: receipt.Exists,
	}
}

func intentPortOrcaWorkspaceReceiptPointer(receipt *preparationcontract.OrcaWorkspaceReceipt) *port.ExecutionOrcaWorkspaceReceipt {
	if receipt == nil {
		return nil
	}
	return &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: intentPortWorkspaceReceipt(receipt.Workspace), RuntimeID: receipt.RuntimeID,
		RepoID: receipt.RepoID, WorktreeID: receipt.WorktreeID, WorktreeInstanceID: receipt.WorktreeInstanceID,
	}
}

func intentPortStage(stage preparationcontract.IntentStage) port.ExecutionOrcaIntentStage {
	return port.ExecutionOrcaIntentStage(stage)
}

func intentContractStage(stage port.ExecutionOrcaIntentStage) preparationcontract.IntentStage {
	return preparationcontract.IntentStage(stage)
}
