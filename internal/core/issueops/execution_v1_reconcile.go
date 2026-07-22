package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

type ExecutionReconcileRequestV1 struct {
	ID      string              `json:"id"`
	Preview bool                `json:"preview,omitempty"`
	Confirm bool                `json:"confirm,omitempty"`
	Actor   model.NativeActorV1 `json:"actor"`
	CWD     string              `json:"cwd"`
}

type ExecutionReconcileDependenciesV1 struct {
	Orca      port.ExecutionOrcaProvisioner
	ReadIssue ExecutionIssueSnapshotReadFuncV1
	RemotePR  RemotePullRequestDependenciesV1
	Now       func() time.Time
}

type ExecutionReconcileResultV1 struct {
	OK         bool                    `json:"ok"`
	ID         string                  `json:"id"`
	Preview    bool                    `json:"preview,omitempty"`
	Reconciled bool                    `json:"reconciled"`
	Code       string                  `json:"code"`
	Execution  model.ExecutionV1       `json:"execution"`
	Pending    *model.ExternalIntentV1 `json:"pending,omitempty"`
}

func ReconcileExecutionV1(stateRoot string, req ExecutionReconcileRequestV1) (ExecutionReconcileResultV1, error) {
	return ReconcileExecutionV1WithDependencies(context.Background(), stateRoot, req, ExecutionReconcileDependenciesV1{})
}

func ReconcileExecutionV1WithDependencies(ctx context.Context, stateRoot string, req ExecutionReconcileRequestV1, deps ExecutionReconcileDependenciesV1) (ExecutionReconcileResultV1, error) {
	if req.Preview == req.Confirm {
		return ExecutionReconcileResultV1{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm")
	}
	if req.Confirm {
		if err := RequireIssueOpsV1MutationAllowed(stateRoot); err != nil {
			return ExecutionReconcileResultV1{OK: false, ID: req.ID}, err
		}
	}
	actor, err := normalizeNativeActorV1(req.Actor)
	if err != nil {
		return ExecutionReconcileResultV1{OK: false, ID: req.ID}, err
	}
	req.Actor = actor
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionReconcileResultV1{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return ExecutionReconcileResultV1{OK: false, ID: req.ID}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if !samePath(req.CWD, record.Execution.Workspace.SourceRoot) && !samePath(req.CWD, record.Execution.Workspace.Root) {
		return ExecutionReconcileResultV1{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile cwd must be source_root or the canonical worktree")
	}
	isOrcaPending := record.Execution.Pending != nil && record.Execution.Mode == model.ExecutionModeOrca && pendingKindForOrcaStageV1FromKind(record.Execution.Pending.Kind)
	if req.Confirm && !isOrcaPending {
		mutationActor := IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return ExecutionReconcileResultV1{OK: false, ID: req.ID}, err
		}
	}
	result := executionReconcileResultV1(record, req.Preview, "")
	if record.Execution.Pending == nil {
		result.Reconciled = true
		result.Code = "no_pending_external_intent"
		return result, nil
	}
	if req.Preview {
		result.Code = reconcilePreviewCodeV1(record.Execution.Pending.Kind)
		return result, nil
	}
	switch record.Execution.Pending.Kind {
	case externalIntentRemotePRV1:
		return reconcileRemotePullRequestV1(ctx, stateRoot, record, deps.RemotePR)
	case "worktree_create", "owner_launch", "dispatch":
		return reconcileOrcaExecutionIntentV1(ctx, stateRoot, record, deps)
	default:
		result.OK = false
		result.Code = "unsupported_external_intent"
		return result, fmt.Errorf("unsupported pending external intent kind %q", record.Execution.Pending.Kind)
	}
}

func reconcileOrcaExecutionIntentV1(ctx context.Context, stateRoot string, record IssueOpsRecord, deps ExecutionReconcileDependenciesV1) (ExecutionReconcileResultV1, error) {
	pending := record.Execution.Pending
	payload, err := readExternalOrcaIntentPayloadV1(stateRoot, pending.OperationID)
	if err != nil {
		return failedExecutionReconcileResultV1(record, "orca_intent_payload_invalid"), err
	}
	updated, next, err := executeOrcaIntentStageV1(ctx, stateRoot, record, payload, deps.Orca, deps.ReadIssue, deps.Now)
	if err != nil {
		if latest, readErr := ReadIssueOps(stateRoot, record.ID); readErr == nil {
			updated = latest
		}
		return failedExecutionReconcileResultV1(updated, "orca_reconcile_ambiguous"), err
	}
	code := "orca_reconcile_completed"
	if updated.Execution != nil && updated.Execution.Pending != nil {
		code = "orca_reconcile_advanced_" + string(next.Stage)
	}
	result := executionReconcileResultV1(updated, false, code)
	result.Reconciled = true
	return result, nil
}

func pendingKindForOrcaStageV1FromKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "worktree_create", "owner_launch", "dispatch":
		return true
	default:
		return false
	}
}

func reconcileRemotePullRequestV1(ctx context.Context, stateRoot string, record IssueOpsRecord, deps RemotePullRequestDependenciesV1) (ExecutionReconcileResultV1, error) {
	pending := record.Execution.Pending
	payload, err := readExternalRemotePRPayloadV1(stateRoot, pending.OperationID)
	if err != nil {
		return failedExecutionReconcileResultV1(record, "external_intent_payload_invalid"), err
	}
	if deps.Reconcile == nil {
		return failedExecutionReconcileResultV1(record, "remote_reconcile_unavailable"), fmt.Errorf("remote reconcile provider is unavailable")
	}
	inventory, err := deps.Reconcile(payload.Provider, remotePullRequestReconcileRequestV1(payload))
	if err != nil {
		_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResultV1(record, "remote_reconcile_ambiguous"), fmt.Errorf("remote reconcile transport is ambiguous; intent retained: %w", err)
	}
	if len(inventory.Candidates) > 1 {
		err := fmt.Errorf("remote reconcile found multiple candidates; intent retained")
		_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResultV1(record, "remote_reconcile_multiple"), err
	}
	if len(inventory.Candidates) == 1 {
		candidate := inventory.Candidates[0]
		if err := validateRemotePullRequestCandidateV1(record, payload, candidate); err != nil {
			_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, payload.KnownURL, err, deps.Now)
			return failedExecutionReconcileResultV1(record, "remote_reconcile_candidate_mismatch"), err
		}
		if err := verifyRemotePullRequestResultV1(record, payload, candidate.URL, deps.Verify); err != nil {
			_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, candidate.URL, err, deps.Now)
			return failedExecutionReconcileResultV1(record, "remote_reconcile_verification_failed"), err
		}
		updated, err := finishRemotePullRequestIntentV1(stateRoot, record.ID, payload, candidate.URL, false, deps.Now)
		if err != nil {
			return failedExecutionReconcileResultV1(record, "remote_reconcile_receipt_failed"), err
		}
		result := executionReconcileResultV1(updated, false, "remote_reconcile_adopted")
		result.Reconciled = true
		return result, nil
	}
	if !inventory.AuthoritativeZero {
		err := fmt.Errorf("remote reconcile returned a non-authoritative zero candidate result; intent retained")
		_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResultV1(record, "remote_reconcile_zero_ambiguous"), err
	}
	if payload.InvocationState != remoteInvocationNotInvokedV1 {
		err := fmt.Errorf("authoritative zero cannot clear an invocation whose absence was not proven; intent retained")
		return failedExecutionReconcileResultV1(record, "remote_reconcile_zero_unproven"), err
	}
	if payload.RetryCount != 0 || deps.Create == nil {
		err := fmt.Errorf("remote create pre-invocation retry is unavailable or already consumed")
		return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_exhausted"), err
	}
	payload, err = markRemotePullRequestRetryV1(stateRoot, record.ID, payload)
	if err != nil {
		return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_cas_failed"), err
	}
	created, createErr := deps.Create(payload.Provider, payload.Request)
	if createErr != nil {
		var typed *port.IssueProviderCreateError
		if errors.As(createErr, &typed) && !typed.Invoked {
			updated, finishErr := finishRemotePullRequestPreInvocationFailureV1(stateRoot, record.ID, payload, createErr, deps.Now)
			if finishErr != nil {
				return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_receipt_failed"), finishErr
			}
			result := executionReconcileResultV1(updated, false, "remote_reconcile_retry_not_invoked")
			result.Reconciled = true
			return result, createErr
		}
		_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, created.URL, createErr, deps.Now)
		return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_ambiguous"), fmt.Errorf("remote retry outcome is ambiguous; creation was not retried again: %w", createErr)
	}
	if err := verifyRemotePullRequestResultV1(record, payload, created.URL, deps.Verify); err != nil {
		_ = recordRemotePullRequestFailureV1(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknownV1, payload.RetryCount, created.URL, err, deps.Now)
		return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_verification_failed"), err
	}
	updated, err := finishRemotePullRequestIntentV1(stateRoot, record.ID, payload, created.URL, false, deps.Now)
	if err != nil {
		return failedExecutionReconcileResultV1(record, "remote_reconcile_retry_receipt_failed"), err
	}
	result := executionReconcileResultV1(updated, false, "remote_reconcile_retry_succeeded")
	result.Reconciled = true
	return result, nil
}

func markRemotePullRequestRetryV1(stateRoot, id string, expected externalRemotePRPayloadV1) (externalRemotePRPayloadV1, error) {
	updated := expected
	updated.InvocationState = remoteInvocationUnknownV1
	updated.RetryCount++
	data, err := jsonMarshalExecutionIntentV1(updated)
	if err != nil {
		return externalRemotePRPayloadV1{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != expected.OperationID {
			return fmt.Errorf("external intent changed before retry CAS")
		}
		current, err := readExternalRemotePRPayloadV1(stateRoot, expected.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("external intent payload changed before retry CAS")
		}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func finishRemotePullRequestPreInvocationFailureV1(stateRoot, id string, payload externalRemotePRPayloadV1, cause error, now func() time.Time) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != payload.OperationID {
			return fmt.Errorf("external intent changed before terminal pre-invocation receipt")
		}
		current, err := readExternalRemotePRPayloadV1(stateRoot, payload.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, payload) {
			return fmt.Errorf("external intent payload changed before terminal pre-invocation receipt")
		}
		record.Execution.Pending = nil
		record.Execution.Failure = &model.ExecutionFailureV1{
			OperationID: payload.OperationID, Code: "external_operation_not_invoked", Message: boundedExecutionRemoteDiagnosticV1(cause), At: executionNow(now),
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentV1Bucket, ID: payload.OperationID, Delete: true}})
		return err
	})
	return persisted, err
}

func executionReconcileResultV1(record IssueOpsRecord, preview bool, code string) ExecutionReconcileResultV1 {
	result := ExecutionReconcileResultV1{OK: true, ID: record.ID, Preview: preview, Code: code}
	if record.Execution != nil {
		result.Execution = *record.Execution
		result.Pending = record.Execution.Pending
	}
	return result
}

func failedExecutionReconcileResultV1(record IssueOpsRecord, code string) ExecutionReconcileResultV1 {
	result := executionReconcileResultV1(record, false, code)
	result.OK = false
	return result
}

func reconcilePreviewCodeV1(kind string) string {
	switch strings.TrimSpace(kind) {
	case externalIntentRemotePRV1:
		return "remote_reconcile_required"
	case "worktree_create", "owner_launch", "dispatch":
		return "orca_reconcile_required"
	default:
		return "unsupported_external_intent"
	}
}

func jsonMarshalExecutionIntentV1(payload externalRemotePRPayloadV1) ([]byte, error) {
	return json.Marshal(payload)
}
