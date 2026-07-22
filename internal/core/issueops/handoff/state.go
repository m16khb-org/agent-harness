package handoff

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/model"
)

const (
	StateClosed           = "closed"
	StateRecoveryRequired = "recovery_required"

	StateOwnershipDispatching        = "ownership_dispatching"
	StateOwnershipDispatched         = "ownership_dispatched"
	StateOwnerOrienting              = "owner_orienting"
	StateOwnerActive                 = "owner_active"
	StateCleanupPendingHumanDecision = "cleanup_pending_human_decision"
	StateCleanupExecuting            = "cleanup_executing"

	DispositionWorkerFailed                 = "worker_failed"
	DispositionCancelled                    = "cancelled"
	DispositionOwnerClosedWorkspaceRetained = "owner_closed_workspace_retained"
	DispositionLocalResourcesRemoved        = "local_resources_removed"

	OperationWorktreeCreate   = "worktree_create"
	OperationTerminalCreate   = "terminal_create"
	OperationTaskCreate       = "task_create"
	OperationDispatch         = "dispatch"
	OperationRuntimeRefresh   = "runtime_refresh"
	OperationLeaseAttestation = "lease_attestation"
)

type Fence struct {
	Attempt        int
	OwnershipEpoch string
	ContextSHA256  string
}

type ClaimRequest struct {
	Fence      Fence
	Worker     model.IssueOpsHostSessionIdentity
	WorkerRoot string
	Now        string
}

func BeginOperation(record model.IssueOpsRecord, fence Fence, pending model.IssueOpsExecutionHandoffPendingOperation) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if handoff.State == StateClosed {
		return record, fmt.Errorf("closed handoff cannot begin an operation")
	}
	if strings.TrimSpace(pending.Kind) == "" {
		return record, fmt.Errorf("pending operation kind is required")
	}
	cleanPending := clonePending(pending)
	if handoff.PendingOperation != nil {
		return record, fmt.Errorf("pending operation %s is already in progress or requires recovery", handoff.PendingOperation.Kind)
	}
	handoff.PendingOperation = &cleanPending
	handoff.UpdatedAt = pending.StartedAt
	return updated, nil
}

func CompleteOperation(record model.IssueOpsRecord, fence Fence, kind, now string) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if handoff.PendingOperation == nil || handoff.PendingOperation.Kind != strings.TrimSpace(kind) {
		return record, fmt.Errorf("pending operation %q does not match", kind)
	}
	handoff.PendingOperation = nil
	handoff.UpdatedAt = now
	return updated, nil
}

func Dispatch(record model.IssueOpsRecord, fence Fence, identity model.IssueOpsOrcaIdentity, now string) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, true)
	if err != nil {
		return record, err
	}
	if handoff.State != StateOwnershipDispatching {
		return record, fmt.Errorf("dispatch requires %s state", StateOwnershipDispatching)
	}
	cleanIdentity := cloneOrca(identity)
	handoff.Orca = &cleanIdentity
	handoff.DeliveryMode = "inject"
	handoff.State = StateOwnershipDispatched
	handoff.DispatchedAt = now
	handoff.UpdatedAt = now
	return updated, nil
}

func Claim(record model.IssueOpsRecord, req ClaimRequest) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, req.Fence, true)
	if err != nil {
		return record, err
	}
	workerRoot := strings.TrimSpace(req.WorkerRoot)
	worker := cleanSession(req.Worker)
	if worker.Host == "" || worker.SessionID == "" || workerRoot == "" {
		return record, fmt.Errorf("worker host, session, and root are required")
	}
	if workerRoot != strings.TrimSpace(handoff.WorkerRoot) {
		return record, fmt.Errorf("worker root does not match handoff")
	}
	if handoff.State == StateOwnerOrienting {
		if handoff.OwnerSession != nil && *handoff.OwnerSession == worker {
			return updated, nil
		}
		return record, fmt.Errorf("ownership handoff is already orienting another owner")
	}
	if handoff.State != StateOwnershipDispatched {
		return record, fmt.Errorf("ownership claim requires %s state", StateOwnershipDispatched)
	}
	handoff.OwnerSession = &worker
	handoff.State = StateOwnerOrienting
	handoff.ClaimedAt = req.Now
	handoff.LastHeartbeatAt = req.Now
	handoff.UpdatedAt = req.Now
	return updated, nil
}

func Heartbeat(record model.IssueOpsRecord, fence Fence, worker model.IssueOpsHostSessionIdentity, now string) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, true)
	if err != nil {
		return record, err
	}
	if (handoff.State != StateOwnerOrienting && handoff.State != StateOwnerActive) || handoff.OwnerSession == nil || *handoff.OwnerSession != cleanSession(worker) {
		return record, fmt.Errorf("heartbeat requires the active handoff owner")
	}
	handoff.LastHeartbeatAt = now
	handoff.UpdatedAt = now
	return updated, nil
}

func MarkRecoveryRequired(record model.IssueOpsRecord, fence Fence, failure model.IssueOpsExecutionHandoffFailure) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if handoff.State != StateOwnershipDispatching || handoff.PendingOperation == nil {
		return record, fmt.Errorf("recovery requires a dispatching handoff with a pending operation")
	}
	cleanFailure := failure
	cleanFailure.Code = strings.TrimSpace(cleanFailure.Code)
	cleanFailure.Message = redact(cleanFailure.Message)
	if cleanFailure.Code == "" {
		return record, fmt.Errorf("recovery failure code is required")
	}
	handoff.State = StateRecoveryRequired
	handoff.Failure = &cleanFailure
	handoff.UpdatedAt = failure.At
	return updated, nil
}

func MarkCleanupOnlyWorktree(record model.IssueOpsRecord, fence Fence, artifact model.IssueOpsOrcaCleanupArtifact, failure model.IssueOpsExecutionHandoffFailure) (model.IssueOpsRecord, error) {
	updated, current, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if current.State != StateRecoveryRequired || current.PendingOperation == nil || current.PendingOperation.Kind != OperationWorktreeCreate {
		return record, fmt.Errorf("cleanup-only worktree recovery requires a pending worktree_create operation")
	}
	artifact.Kind = strings.TrimSpace(artifact.Kind)
	artifact.ID = strings.TrimSpace(artifact.ID)
	artifact.InstanceID = strings.TrimSpace(artifact.InstanceID)
	artifact.Path = strings.TrimSpace(artifact.Path)
	artifact.Reason = redact(artifact.Reason)
	if artifact.Kind != "worktree" || artifact.ID == "" || artifact.Reason == "" {
		return record, fmt.Errorf("cleanup-only worktree evidence requires exact kind, id, and reason")
	}
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Message = redact(failure.Message)
	if failure.Code != "worktree_cleanup_only" {
		return record, fmt.Errorf("cleanup-only worktree recovery requires worktree_cleanup_only failure code")
	}
	current.PendingOperation = nil
	current.State = StateRecoveryRequired
	current.CleanupOnly = &artifact
	current.Failure = &failure
	current.UpdatedAt = failure.At
	return updated, nil
}

func fencedCopy(record model.IssueOpsRecord, fence Fence, requireContext bool) (model.IssueOpsRecord, *model.IssueOpsExecutionHandoff, error) {
	if err := ValidateEnvelope(record); err != nil {
		return record, nil, err
	}
	current := model.CurrentOwnershipAttempt(record)
	if current == nil || current.Handoff == nil {
		return record, nil, fmt.Errorf("execution handoff is required")
	}
	updated := record
	copyLedger := *record.Ownership
	copyLedger.Attempts = append([]model.IssueOpsOwnershipAttempt(nil), record.Ownership.Attempts...)
	updated.Ownership = &copyLedger
	updatedAttempt := model.CurrentOwnershipAttempt(updated)
	copyHandoff := cloneHandoff(*current.Handoff)
	updatedAttempt.Handoff = &copyHandoff
	if copyHandoff.Attempt != fence.Attempt || copyHandoff.OwnershipEpoch != strings.TrimSpace(fence.OwnershipEpoch) {
		return record, nil, fmt.Errorf("stale handoff attempt or ownership epoch")
	}
	if requireContext && (copyHandoff.ContextSHA256 == "" || copyHandoff.ContextSHA256 != strings.TrimSpace(fence.ContextSHA256)) {
		return record, nil, fmt.Errorf("stale handoff context")
	}
	return updated, updatedAttempt.Handoff, nil
}

func cloneHandoff(value model.IssueOpsExecutionHandoff) model.IssueOpsExecutionHandoff {
	cloned := value
	if value.OwnerSession != nil {
		v := *value.OwnerSession
		cloned.OwnerSession = &v
	}
	if value.Orientation != nil {
		v := *value.Orientation
		cloned.Orientation = &v
	}
	if value.Completion != nil {
		v := *value.Completion
		v.ChangedFiles = append([]string(nil), value.Completion.ChangedFiles...)
		v.Verification = append([]string(nil), value.Completion.Verification...)
		cloned.Completion = &v
	}
	if value.Orca != nil {
		v := cloneOrca(*value.Orca)
		cloned.Orca = &v
	}
	if value.PendingOperation != nil {
		v := clonePending(*value.PendingOperation)
		cloned.PendingOperation = &v
	}
	if value.Failure != nil {
		v := *value.Failure
		cloned.Failure = &v
	}
	if value.Cancellation != nil {
		v := *value.Cancellation
		cloned.Cancellation = &v
	}
	if value.PublishReceipt != nil {
		v := *value.PublishReceipt
		cloned.PublishReceipt = &v
	}
	if value.PublicationRecovery != nil {
		v := *value.PublicationRecovery
		cloned.PublicationRecovery = &v
	}
	if value.Cleanup != nil {
		v := *value.Cleanup
		v.Receipts = append([]model.IssueOpsExecutionHandoffCleanupReceipt(nil), value.Cleanup.Receipts...)
		cloned.Cleanup = &v
	}
	if value.CleanupOnly != nil {
		v := *value.CleanupOnly
		cloned.CleanupOnly = &v
	}
	if value.ContextOptions != nil {
		cloned.ContextOptions = cloneContextOptions(value.ContextOptions)
	}
	return cloned
}

func cloneContextOptions(value *model.IssueOpsExecutionHandoffContextOptions) *model.IssueOpsExecutionHandoffContextOptions {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.CriteriaIDs = append([]string(nil), value.CriteriaIDs...)
	cloned.RequiredDocs = append([]string(nil), value.RequiredDocs...)
	cloned.RequiredSkills = append([]string(nil), value.RequiredSkills...)
	cloned.VerificationCommands = append([]string(nil), value.VerificationCommands...)
	cloned.StopConditions = append([]string(nil), value.StopConditions...)
	return &cloned
}

func clonePending(value model.IssueOpsExecutionHandoffPendingOperation) model.IssueOpsExecutionHandoffPendingOperation {
	value.BaselineWorktreeIDs = append([]string(nil), value.BaselineWorktreeIDs...)
	value.BaselineTaskIDs = append([]string(nil), value.BaselineTaskIDs...)
	value.BaselinePTYIDs = append([]string(nil), value.BaselinePTYIDs...)
	return value
}

func cloneOrca(value model.IssueOpsOrcaIdentity) model.IssueOpsOrcaIdentity {
	value.TerminalBaselinePTYIDs = append([]string(nil), value.TerminalBaselinePTYIDs...)
	return value
}

func cleanChangedFileList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clean := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func cleanResultList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clean := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = redact(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func cleanSession(value model.IssueOpsHostSessionIdentity) model.IssueOpsHostSessionIdentity {
	value.Host = strings.TrimSpace(value.Host)
	value.SessionID = strings.TrimSpace(value.SessionID)
	value.AgentID = strings.TrimSpace(value.AgentID)
	return value
}
