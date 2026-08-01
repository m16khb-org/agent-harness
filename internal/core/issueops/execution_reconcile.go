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

type ExecutionReconcileRequest struct {
	ID       string            `json:"id"`
	Preview  bool              `json:"preview,omitempty"`
	Confirm  bool              `json:"confirm,omitempty"`
	Actor    model.NativeActor `json:"actor"`
	CWD      string            `json:"cwd"`
	Snapshot *IssueOpsRecord   `json:"-"`
}

type ExecutionReconcileDependencies struct {
	Orca      port.ExecutionOrcaProvisioner
	ReadIssue ExecutionIssueSnapshotReadFunc
	RemotePR  RemotePullRequestDependencies
	Now       func() time.Time
	Handler   ExecutionReconcileHandler
}

type ExecutionReconcileResult struct {
	OK                  bool                  `json:"ok"`
	ID                  string                `json:"id"`
	Preview             bool                  `json:"preview,omitempty"`
	Reconciled          bool                  `json:"reconciled"`
	Code                string                `json:"code"`
	Execution           model.Execution       `json:"execution"`
	Pending             *model.ExternalIntent `json:"pending,omitempty"`
	IssueSnapshotSource string                `json:"issue_snapshot_source,omitempty"`
	// ExternalStateInspected는 이 결과가 외부 자원을 실제로 조회하고 나온
	// 것인지 밝힌다. preview는 pending kind만 보고 상수 코드를 돌려주므로
	// false다 — 그 구분이 없으면 preview 출력이 "Orca 자원이 이런 상태다"라는
	// 관측 증거로 오독된다(#99의 오진단이 그렇게 생겼다, 이슈 #154).
	//
	// omitempty를 쓰지 않는다. "조회하지 않았다"가 이 필드의 핵심 정보이므로
	// false가 출력에서 사라지면 목적 자체가 무너진다.
	ExternalStateInspected bool   `json:"external_state_inspected"`
	IntentMigrationCode    string `json:"intent_migration_code,omitempty"`
}

func ReconcileExecution(stateRoot string, req ExecutionReconcileRequest) (ExecutionReconcileResult, error) {
	return ReconcileExecutionWithDependencies(context.Background(), stateRoot, req, ExecutionReconcileDependencies{})
}

func ReconcileExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
	if req.Preview == req.Confirm {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm")
	}
	if req.Confirm {
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionReconcileResult{OK: false, ID: req.ID}, err
		}
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	req.Actor = actor
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if !samePath(req.CWD, record.Execution.Workspace.SourceRoot) && !samePath(req.CWD, record.Execution.Workspace.Root) {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile cwd must be source_root or the canonical worktree")
	}
	isOrcaPending := record.Execution.Pending != nil && record.Execution.Mode == model.ExecutionModeOrca && pendingKindForOrcaStageFromKind(record.Execution.Pending.Kind)
	if req.Confirm && !isOrcaPending {
		mutationActor := IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return ExecutionReconcileResult{OK: false, ID: req.ID}, err
		}
	}
	result := executionReconcileResult(record, req.Preview, "")
	if record.Execution.Pending == nil {
		result.Reconciled = true
		result.Code = "no_pending_external_intent"
		return result, nil
	}
	if req.Preview {
		result.Code = reconcilePreviewCode(record.Execution.Pending.Kind)
		return result, nil
	}
	switch record.Execution.Pending.Kind {
	case externalIntentRemotePR:
		return reconcileRemotePullRequest(ctx, stateRoot, record, deps.RemotePR)
	case "worktree_create", "owner_launch", "dispatch":
		if deps.Handler == nil {
			return failedExecutionReconcileResult(record, "orca_reconcile_ambiguous"), ErrReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.Handler(ctx, stateRoot, req, deps)
	default:
		result.OK = false
		result.Code = "unsupported_external_intent"
		return result, fmt.Errorf("unsupported pending external intent kind %q", record.Execution.Pending.Kind)
	}
}

func pendingKindForOrcaStageFromKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "worktree_create", "owner_launch", "dispatch":
		return true
	default:
		return false
	}
}

func reconcileRemotePullRequest(ctx context.Context, stateRoot string, record IssueOpsRecord, deps RemotePullRequestDependencies) (result ExecutionReconcileResult, err error) {
	// 반환 경로가 여러 갈래이므로 조회 여부를 한 곳에서 표시한다. 경로마다
	// 따로 붙이면 하나를 빠뜨렸을 때 결과가 조용히 거짓말을 한다(이슈 #154).
	inspected := false
	defer func() { result.ExternalStateInspected = inspected }()

	pending := record.Execution.Pending
	payload, err := readExternalRemotePRPayload(stateRoot, pending.OperationID)
	if err != nil {
		return failedExecutionReconcileResult(record, "external_intent_payload_invalid"), err
	}
	if deps.Reconcile == nil {
		return failedExecutionReconcileResult(record, "remote_reconcile_unavailable"), fmt.Errorf("remote reconcile provider is unavailable")
	}
	inventory, err := deps.Reconcile(payload.Provider, remotePullRequestReconcileRequest(payload))
	inspected = true
	if err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResult(record, "remote_reconcile_ambiguous"), fmt.Errorf("remote reconcile transport is ambiguous; intent retained: %w", err)
	}
	if len(inventory.Candidates) > 1 {
		err := fmt.Errorf("remote reconcile found multiple candidates; intent retained")
		_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResult(record, "remote_reconcile_multiple"), err
	}
	if len(inventory.Candidates) == 1 {
		candidate := inventory.Candidates[0]
		if err := validateRemotePullRequestCandidate(record, payload, candidate); err != nil {
			_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, payload.KnownURL, err, deps.Now)
			return failedExecutionReconcileResult(record, "remote_reconcile_candidate_mismatch"), err
		}
		if err := verifyRemotePullRequestResult(record, payload, candidate.URL, deps.Verify); err != nil {
			_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, candidate.URL, err, deps.Now)
			return failedExecutionReconcileResult(record, "remote_reconcile_verification_failed"), err
		}
		updated, err := finishRemotePullRequestIntent(stateRoot, record.ID, payload, candidate.URL, false, deps.Now)
		if err != nil {
			return failedExecutionReconcileResult(record, "remote_reconcile_receipt_failed"), err
		}
		result := executionReconcileResult(updated, false, "remote_reconcile_adopted")
		result.Reconciled = true
		return result, nil
	}
	if !inventory.AuthoritativeZero {
		err := fmt.Errorf("remote reconcile returned a non-authoritative zero candidate result; intent retained")
		_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, payload.KnownURL, err, deps.Now)
		return failedExecutionReconcileResult(record, "remote_reconcile_zero_ambiguous"), err
	}
	if payload.InvocationState != remoteInvocationNotInvoked {
		err := fmt.Errorf("authoritative zero cannot clear an invocation whose absence was not proven; intent retained")
		return failedExecutionReconcileResult(record, "remote_reconcile_zero_unproven"), err
	}
	if payload.RetryCount != 0 || deps.Create == nil {
		err := fmt.Errorf("remote create pre-invocation retry is unavailable or already consumed")
		return failedExecutionReconcileResult(record, "remote_reconcile_retry_exhausted"), err
	}
	payload, err = markRemotePullRequestRetry(stateRoot, record.ID, payload)
	if err != nil {
		return failedExecutionReconcileResult(record, "remote_reconcile_retry_cas_failed"), err
	}
	created, createErr := deps.Create(payload.Provider, payload.Request)
	if createErr != nil {
		var typed *port.IssueProviderCreateError
		if errors.As(createErr, &typed) && !typed.Invoked {
			updated, finishErr := finishRemotePullRequestPreInvocationFailure(stateRoot, record.ID, payload, createErr, deps.Now)
			if finishErr != nil {
				return failedExecutionReconcileResult(record, "remote_reconcile_retry_receipt_failed"), finishErr
			}
			result := executionReconcileResult(updated, false, "remote_reconcile_retry_not_invoked")
			result.Reconciled = true
			return result, createErr
		}
		_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, created.URL, createErr, deps.Now)
		return failedExecutionReconcileResult(record, "remote_reconcile_retry_ambiguous"), fmt.Errorf("remote retry outcome is ambiguous; creation was not retried again: %w", createErr)
	}
	if err := verifyRemotePullRequestResult(record, payload, created.URL, deps.Verify); err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, record.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, created.URL, err, deps.Now)
		return failedExecutionReconcileResult(record, "remote_reconcile_retry_verification_failed"), err
	}
	updated, err := finishRemotePullRequestIntent(stateRoot, record.ID, payload, created.URL, false, deps.Now)
	if err != nil {
		return failedExecutionReconcileResult(record, "remote_reconcile_retry_receipt_failed"), err
	}
	result = executionReconcileResult(updated, false, "remote_reconcile_retry_succeeded")
	result.Reconciled = true
	return result, nil
}

func markRemotePullRequestRetry(stateRoot, id string, expected externalRemotePRPayload) (externalRemotePRPayload, error) {
	updated := expected
	updated.InvocationState = remoteInvocationUnknown
	updated.RetryCount++
	data, err := jsonMarshalExecutionIntent(updated)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != expected.OperationID {
			return fmt.Errorf("external intent changed before retry CAS")
		}
		current, err := readExternalRemotePRPayload(stateRoot, expected.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("external intent payload changed before retry CAS")
		}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func finishRemotePullRequestPreInvocationFailure(stateRoot, id string, payload externalRemotePRPayload, cause error, now func() time.Time) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != payload.OperationID {
			return fmt.Errorf("external intent changed before terminal pre-invocation receipt")
		}
		current, err := readExternalRemotePRPayload(stateRoot, payload.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, payload) {
			return fmt.Errorf("external intent payload changed before terminal pre-invocation receipt")
		}
		record.Execution.Pending = nil
		record.Execution.Failure = &model.ExecutionFailure{
			OperationID: payload.OperationID, Code: "external_operation_not_invoked", Message: boundedExecutionRemoteDiagnostic(cause), At: executionNow(now),
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: payload.OperationID, Delete: true}})
		return err
	})
	return persisted, err
}

func executionReconcileResult(record IssueOpsRecord, preview bool, code string) ExecutionReconcileResult {
	result := ExecutionReconcileResult{OK: true, ID: record.ID, Preview: preview, Code: code}
	if record.Execution != nil {
		result.Execution = *record.Execution
		result.Pending = record.Execution.Pending
	}
	return result
}

func failedExecutionReconcileResult(record IssueOpsRecord, code string) ExecutionReconcileResult {
	result := executionReconcileResult(record, false, code)
	result.OK = false
	return result
}

func reconcilePreviewCode(kind string) string {
	switch strings.TrimSpace(kind) {
	case externalIntentRemotePR:
		return "remote_reconcile_required"
	case "worktree_create", "owner_launch", "dispatch":
		return "orca_reconcile_required"
	default:
		return "unsupported_external_intent"
	}
}

func jsonMarshalExecutionIntent(payload externalRemotePRPayload) ([]byte, error) {
	return json.Marshal(payload)
}
