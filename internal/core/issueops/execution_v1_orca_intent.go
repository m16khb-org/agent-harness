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

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const (
	orcaIntentNotInvokedV1  = "not_invoked_proven"
	orcaIntentUnknownV1     = "unknown"
	maxOrcaIntentAttemptsV1 = 2
)

type externalOrcaLaunchIdentityV1 struct {
	PromptPath          string `json:"prompt_path"`
	PromptSHA256        string `json:"prompt_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
}

type externalOrcaIntentPayloadV1 struct {
	SchemaVersion      int                                 `json:"schema_version"`
	OperationID        string                              `json:"operation_id"`
	LifecycleID        string                              `json:"lifecycle_id"`
	Generation         uint64                              `json:"generation"`
	Stage              port.ExecutionOrcaIntentStage       `json:"stage"`
	Marker             string                              `json:"marker"`
	StartedAt          string                              `json:"started_at"`
	InvocationState    string                              `json:"invocation_state"`
	InvocationAttempts int                                 `json:"invocation_attempts"`
	Workspace          port.ExecutionWorkspaceRequest      `json:"workspace"`
	Probe              port.ExecutionOrcaProbeRequest      `json:"probe"`
	Prepared           *port.ExecutionOrcaWorkspaceReceipt `json:"prepared,omitempty"`
	Launch             *externalOrcaLaunchIdentityV1       `json:"launch,omitempty"`
	IssueBodySHA256    string                              `json:"issue_body_sha256"`
	ClaimTokenSHA256   string                              `json:"claim_token_sha256,omitempty"`
	TerminalPTYID      string                              `json:"terminal_pty_id,omitempty"`
	TaskID             string                              `json:"task_id,omitempty"`
}

func beginOrcaExecutionIntentV1(stateRoot string, record IssueOpsRecord, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, req ExecutionPrepareRequestV1, snapshot executionOwnerSnapshotV1, now func() time.Time) (IssueOpsRecord, externalOrcaIntentPayloadV1, error) {
	operationID, err := newExecutionOperationIDV1()
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayloadV1{}, err
	}
	startedAt := executionNow(now)
	marker := "agent-harness issueops-v1 lifecycle=" + record.ID + " operation=" + operationID
	probe.Marker = marker
	payload := externalOrcaIntentPayloadV1{
		SchemaVersion: model.IssueOpsSchemaVersion, OperationID: operationID, LifecycleID: record.ID,
		Generation: 1, Stage: port.ExecutionOrcaIntentWorktree, Marker: marker, StartedAt: startedAt,
		InvocationState: orcaIntentNotInvokedV1, Workspace: workspace, Probe: probe,
		IssueBodySHA256: snapshot.issue.BodySHA256,
	}
	if strings.ToLower(strings.TrimSpace(req.OwnerHost)) != probe.Host || strings.TrimSpace(req.OwnerModel) != probe.Model || strings.TrimSpace(req.OwnerEffort) != probe.Effort {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayloadV1{}, fmt.Errorf("owner profile changed before Orca intent persistence")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayloadV1{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return err
		}
		if current.Execution != nil {
			return fmt.Errorf("IssueOps execution already exists; reconcile or inspect its current state")
		}
		current.Execution = &model.ExecutionV1{
			Mode: model.ExecutionModeOrca,
			Workspace: model.WorkspaceV1{
				SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
				BaseHead: workspace.BaseHead, Driver: "orca", LinkedAt: startedAt,
			},
			Lease: model.WriteLeaseV1{Generation: payload.Generation, Status: model.LeaseStatusReleased},
			Pending: &model.ExternalIntentV1{
				OperationID: operationID, Kind: string(port.ExecutionOrcaIntentWorktree), Marker: marker, StartedAt: startedAt,
			},
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{
			Bucket: externalIntentV1Bucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func executeOrcaIntentStageV1(ctx context.Context, stateRoot string, record IssueOpsRecord, payload externalOrcaIntentPayloadV1, orca port.ExecutionOrcaProvisioner, readIssue ExecutionIssueSnapshotReadFuncV1, now func() time.Time) (IssueOpsRecord, externalOrcaIntentPayloadV1, error) {
	if orca == nil {
		return record, payload, fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := executionOrcaIntentRequestV1(record, payload)
	if err != nil {
		return record, payload, err
	}
	inventory, err := orca.InspectIntent(ctx, request)
	if err != nil {
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
	}
	if len(inventory.Candidates) > 1 {
		err := fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if len(inventory.Candidates) == 1 {
		updated, next, advanceErr := advanceOrcaIntentReceiptV1(ctx, stateRoot, record, payload, inventory.Candidates[0], readIssue, now)
		if advanceErr != nil {
			_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, orcaIntentUnknownV1, advanceErr, now)
		}
		return updated, next, advanceErr
	}
	if !inventory.AuthoritativeZero {
		err := fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if payload.InvocationState != orcaIntentNotInvokedV1 {
		err := fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	if payload.InvocationAttempts >= maxOrcaIntentAttemptsV1 {
		err := fmt.Errorf("Orca intent retry is exhausted; intent retained")
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, payload.InvocationState, err, now)
		return record, payload, err
	}
	payload, err = markOrcaIntentInvokingV1(stateRoot, record.ID, payload)
	if err != nil {
		return record, payload, err
	}
	receipt, invokeErr := orca.InvokeIntent(ctx, request)
	if invokeErr != nil {
		invocation := orcaIntentUnknownV1
		var typed *port.OrcaError
		if errors.As(invokeErr, &typed) && !typed.Invoked {
			invocation = orcaIntentNotInvokedV1
		}
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, invocation, invokeErr, now)
		return record, payload, fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", invokeErr)
	}
	updated, next, advanceErr := advanceOrcaIntentReceiptV1(ctx, stateRoot, record, payload, receipt, readIssue, now)
	if advanceErr != nil {
		_ = recordOrcaIntentFailureV1(stateRoot, record.ID, payload, orcaIntentUnknownV1, advanceErr, now)
	}
	return updated, next, advanceErr
}

func markOrcaIntentInvokingV1(stateRoot, id string, expected externalOrcaIntentPayloadV1) (externalOrcaIntentPayloadV1, error) {
	updated := expected
	updated.InvocationState = orcaIntentUnknownV1
	updated.InvocationAttempts++
	data, err := json.Marshal(updated)
	if err != nil {
		return externalOrcaIntentPayloadV1{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, stored, err := readAndMatchOrcaIntentV1(stateRoot, id, expected)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("Orca intent payload changed before invocation CAS")
		}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func recordOrcaIntentFailureV1(stateRoot, id string, expected externalOrcaIntentPayloadV1, invocation string, cause error, now func() time.Time) error {
	return withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, stored, err := readAndMatchOrcaIntentV1(stateRoot, id, expected)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("Orca intent payload changed before failure receipt")
		}
		stored.InvocationState = invocation
		data, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		message := boundedExecutionRemoteDiagnosticV1(cause)
		record.Execution.Failure = &model.ExecutionFailureV1{
			OperationID: stored.OperationID, Code: "external_operation_ambiguous", Message: message, At: executionNow(now),
		}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: stored.OperationID, Data: data}})
		return err
	})
}

func advanceOrcaIntentReceiptV1(ctx context.Context, stateRoot string, record IssueOpsRecord, expected externalOrcaIntentPayloadV1, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFuncV1, now func() time.Time) (IssueOpsRecord, externalOrcaIntentPayloadV1, error) {
	updated := expected
	updated.InvocationState = orcaIntentNotInvokedV1
	updated.InvocationAttempts = 0
	switch expected.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if receipt.Workspace == nil || validateExecutionOrcaWorkspaceReceiptV1(expected.Workspace, *receipt.Workspace) != nil {
			return record, expected, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
		}
		prepared := record
		prepared.WorktreePath = receipt.Workspace.Workspace.Root
		prepared.Execution.Workspace = workspaceFromReceipt(receipt.Workspace.Workspace, expected.StartedAt)
		snapshot, err := readExecutionOwnerSnapshotV1(ctx, prepared, readIssue)
		if err != nil {
			return record, expected, err
		}
		if snapshot.issue.BodySHA256 != expected.IssueBodySHA256 {
			return record, expected, fmt.Errorf("remote issue body drifted before owner launch recovery")
		}
		tokenHash, err := createOrAdoptClaimTokenV1(prepared)
		if err != nil {
			return record, expected, err
		}
		artifacts, err := buildExecutionOwnerArtifactsV1(prepared, ExecutionPrepareRequestV1{
			ID: prepared.ID, Mode: string(model.ExecutionModeOrca), OwnerHost: expected.Probe.Host,
			OwnerModel: expected.Probe.Model, OwnerEffort: expected.Probe.Effort,
		}, snapshot)
		if err != nil {
			return record, expected, err
		}
		updated.Prepared = receipt.Workspace
		updated.Launch = &externalOrcaLaunchIdentityV1{
			PromptPath: artifacts.promptPath, PromptSHA256: artifacts.promptSHA256,
			ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		}
		updated.ClaimTokenSHA256 = tokenHash
		updated.Stage = port.ExecutionOrcaIntentTerminal
	case port.ExecutionOrcaIntentTerminal:
		if strings.TrimSpace(receipt.TerminalPTYID) == "" {
			return record, expected, fmt.Errorf("Orca terminal candidate is incomplete")
		}
		updated.TerminalPTYID = strings.TrimSpace(receipt.TerminalPTYID)
		updated.Stage = port.ExecutionOrcaIntentTask
	case port.ExecutionOrcaIntentTask:
		if strings.TrimSpace(receipt.TaskID) == "" {
			return record, expected, fmt.Errorf("Orca task candidate is incomplete")
		}
		updated.TaskID = strings.TrimSpace(receipt.TaskID)
		updated.Stage = port.ExecutionOrcaIntentDispatch
	case port.ExecutionOrcaIntentDispatch:
		if strings.TrimSpace(receipt.TaskID) != expected.TaskID || strings.TrimSpace(receipt.DispatchID) == "" {
			return record, expected, fmt.Errorf("Orca dispatch candidate is incomplete")
		}
	default:
		return record, expected, fmt.Errorf("unsupported Orca intent stage %q", expected.Stage)
	}

	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, stored, err := readAndMatchOrcaIntentV1(stateRoot, record.ID, expected)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("Orca intent payload changed before receipt CAS")
		}
		if expected.Stage == port.ExecutionOrcaIntentWorktree {
			current.WorktreePath = updated.Prepared.Workspace.Root
			current.Execution.Workspace = workspaceFromReceipt(updated.Prepared.Workspace, expected.StartedAt)
		}
		if expected.Stage == port.ExecutionOrcaIntentDispatch {
			current.Execution.Lease = model.WriteLeaseV1{
				Generation: expected.Generation, Status: model.LeaseStatusClaimable, ClaimTokenSHA256: expected.ClaimTokenSHA256,
			}
			current.Execution.Orca = &model.OrcaBindingV1{
				RuntimeID: expected.Prepared.RuntimeID, RepoID: expected.Prepared.RepoID, WorktreeID: expected.Prepared.WorktreeID,
				WorktreeInstanceID: expected.Prepared.WorktreeInstanceID, OwnerHost: expected.Probe.Host,
				OwnerModel: expected.Probe.Model, OwnerEffort: expected.Probe.Effort, TaskID: expected.TaskID,
				DispatchID: receipt.DispatchID, TerminalPTYID: expected.TerminalPTYID,
			}
			current.Execution.Pending = nil
			current.Execution.Failure = nil
			persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: expected.OperationID, Delete: true}})
			return err
		}
		current.Execution.Pending.Kind = pendingKindForOrcaStageV1(updated.Stage)
		current.Execution.Failure = nil
		data, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: expected.OperationID, Data: data}})
		return err
	})
	if err != nil {
		return record, expected, err
	}
	return persisted, updated, nil
}

func readAndMatchOrcaIntentV1(stateRoot, id string, expected externalOrcaIntentPayloadV1) (IssueOpsRecord, externalOrcaIntentPayloadV1, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsRecord{}, externalOrcaIntentPayloadV1{}, err
	}
	if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != expected.OperationID ||
		record.Execution.Pending.Marker != expected.Marker || record.Execution.Pending.Kind != pendingKindForOrcaStageV1(expected.Stage) ||
		record.Execution.Lease.Generation != expected.Generation || record.Execution.Lease.Status != model.LeaseStatusReleased || record.Execution.Orca != nil {
		return record, externalOrcaIntentPayloadV1{}, fmt.Errorf("Orca intent authority changed before CAS")
	}
	if err := validateOrcaIntentRecordIdentityV1(record, expected); err != nil {
		return record, externalOrcaIntentPayloadV1{}, err
	}
	stored, err := readExternalOrcaIntentPayloadV1(stateRoot, expected.OperationID)
	return record, stored, err
}

func readExternalOrcaIntentPayloadV1(stateRoot, operationID string) (externalOrcaIntentPayloadV1, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalOrcaIntentPayloadV1{}, err
	}
	data, ok, err := db.Get(externalIntentV1Bucket, operationID)
	if err != nil {
		return externalOrcaIntentPayloadV1{}, err
	}
	if !ok {
		return externalOrcaIntentPayloadV1{}, fmt.Errorf("Orca external intent payload is missing")
	}
	var payload externalOrcaIntentPayloadV1
	if err := json.Unmarshal(data, &payload); err != nil {
		return externalOrcaIntentPayloadV1{}, fmt.Errorf("decode Orca external intent payload: %w", err)
	}
	if err := validateExternalOrcaIntentPayloadV1(payload, operationID); err != nil {
		return externalOrcaIntentPayloadV1{}, err
	}
	return payload, nil
}

func validateExternalOrcaIntentPayloadV1(payload externalOrcaIntentPayloadV1, operationID string) error {
	if payload.SchemaVersion != model.IssueOpsSchemaVersion || payload.OperationID != operationID || payload.LifecycleID == "" || payload.Generation != 1 ||
		payload.Marker == "" || payload.StartedAt == "" || payload.Workspace.LifecycleID != payload.LifecycleID || payload.Probe.Marker != payload.Marker ||
		!samePath(payload.Probe.Repo, payload.Workspace.SourceRoot) || strings.TrimSpace(payload.Probe.Model) == "" ||
		(payload.Probe.Host != "codex" && payload.Probe.Host != "claude") ||
		(payload.InvocationState != orcaIntentNotInvokedV1 && payload.InvocationState != orcaIntentUnknownV1) ||
		payload.InvocationAttempts < 0 || payload.InvocationAttempts > maxOrcaIntentAttemptsV1 || !executionV1SHA256.MatchString(payload.IssueBodySHA256) {
		return fmt.Errorf("Orca external intent payload is invalid")
	}
	switch payload.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if payload.Prepared != nil || payload.Launch != nil || payload.ClaimTokenSHA256 != "" || payload.TerminalPTYID != "" || payload.TaskID != "" {
			return fmt.Errorf("Orca worktree intent payload contains later-stage receipts")
		}
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch:
		if payload.Prepared == nil || payload.Launch == nil || !executionV1SHA256.MatchString(payload.ClaimTokenSHA256) ||
			!executionV1SHA256.MatchString(payload.Launch.PromptSHA256) || !executionV1SHA256.MatchString(payload.Launch.ContextPacketSHA256) ||
			strings.TrimSpace(payload.Launch.PromptPath) == "" || strings.TrimSpace(payload.Launch.ContextPacketPath) == "" ||
			validateExecutionOrcaWorkspaceReceiptV1(payload.Workspace, *payload.Prepared) != nil {
			return fmt.Errorf("Orca owner intent payload is missing sealed launch receipts")
		}
		if payload.Stage == port.ExecutionOrcaIntentTerminal && (payload.TerminalPTYID != "" || payload.TaskID != "") {
			return fmt.Errorf("Orca terminal intent payload contains later-stage receipts")
		}
		if payload.Stage == port.ExecutionOrcaIntentTask && (payload.TerminalPTYID == "" || payload.TaskID != "") {
			return fmt.Errorf("Orca task intent payload is incomplete")
		}
		if payload.Stage == port.ExecutionOrcaIntentDispatch && (payload.TerminalPTYID == "" || payload.TaskID == "") {
			return fmt.Errorf("Orca dispatch intent payload is incomplete")
		}
	default:
		return fmt.Errorf("unsupported Orca external intent stage %q", payload.Stage)
	}
	return nil
}

func executionOrcaIntentRequestV1(record IssueOpsRecord, payload externalOrcaIntentPayloadV1) (port.ExecutionOrcaIntentRequest, error) {
	if err := validateExternalOrcaIntentPayloadV1(payload, payload.OperationID); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentRecordIdentityV1(record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	request := port.ExecutionOrcaIntentRequest{
		Stage: payload.Stage, Marker: payload.Marker, Workspace: payload.Workspace, Probe: payload.Probe,
		Prepared: payload.Prepared, TerminalPTYID: payload.TerminalPTYID, TaskID: payload.TaskID,
	}
	if payload.Launch != nil {
		expectedPacketPath, expectedPromptPath := executionOwnerArtifactPathsV1(record)
		if !samePath(payload.Launch.PromptPath, expectedPromptPath) || !samePath(payload.Launch.ContextPacketPath, expectedPacketPath) {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner artifact path changed")
		}
		token, err := readClaimToken(record, claimTokenPath(record))
		if err != nil || tokenSHA256(token) != payload.ClaimTokenSHA256 {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed claim token identity changed")
		}
		prompt, err := readExecutionOwnerArtifactV1(record.Execution.Workspace.Root, payload.Launch.PromptPath)
		if err != nil || digestExecutionOwnerBytesV1(prompt) != payload.Launch.PromptSHA256 {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner prompt identity changed")
		}
		packet, err := readExecutionOwnerArtifactV1(record.Execution.Workspace.Root, payload.Launch.ContextPacketPath)
		if err != nil || digestExecutionOwnerBytesV1(packet) != payload.Launch.ContextPacketSHA256 {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed context packet identity changed")
		}
		request.Launch = &port.ExecutionOrcaLaunchRequest{
			Prompt: string(prompt), PromptPath: payload.Launch.PromptPath, PromptSHA256: payload.Launch.PromptSHA256,
			ContextPacketPath: payload.Launch.ContextPacketPath, ContextPacketSHA256: payload.Launch.ContextPacketSHA256,
		}
	}
	return request, nil
}

func validateOrcaIntentRecordIdentityV1(record IssueOpsRecord, payload externalOrcaIntentPayloadV1) error {
	if record.ID != payload.LifecycleID || record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca ||
		!samePath(record.Execution.Workspace.SourceRoot, payload.Workspace.SourceRoot) || !samePath(record.Execution.Workspace.Root, payload.Workspace.Root) ||
		record.Execution.Workspace.Branch != payload.Workspace.Branch || record.Execution.Workspace.BaseHead != payload.Workspace.BaseHead ||
		record.Execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca intent record identity changed")
	}
	if payload.Prepared != nil && (!samePath(record.WorktreePath, payload.Prepared.Workspace.Root) ||
		!samePath(record.Execution.Workspace.Root, payload.Prepared.Workspace.Root) || record.Execution.Workspace.Branch != payload.Prepared.Workspace.Branch ||
		record.Execution.Workspace.BaseHead != payload.Prepared.Workspace.BaseHead) {
		return fmt.Errorf("Orca prepared workspace identity changed")
	}
	return nil
}

func pendingKindForOrcaStageV1(stage port.ExecutionOrcaIntentStage) string {
	switch stage {
	case port.ExecutionOrcaIntentWorktree:
		return "worktree_create"
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask:
		return "owner_launch"
	case port.ExecutionOrcaIntentDispatch:
		return "dispatch"
	default:
		return ""
	}
}

func createOrAdoptClaimTokenV1(record IssueOpsRecord) (string, error) {
	token, _, err := createClaimToken(record)
	if err == nil {
		return tokenSHA256(token), nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	token, err = readClaimToken(record, claimTokenPath(record))
	if err != nil {
		return "", fmt.Errorf("recover deterministic claim token: %w", err)
	}
	return tokenSHA256(token), nil
}
