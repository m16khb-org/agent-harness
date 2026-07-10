package handoff

import (
	"fmt"
	"reflect"
	"strings"

	"agent-harness/internal/core/issueops/model"
)

const (
	ProtocolVersion = 1

	StateCoordinatorPreparing = "coordinator_preparing"
	StateDispatched           = "dispatched"
	StateClaimed              = "claimed"
	StateSubmitted            = "submitted"
	StateClosed               = "closed"
	StateRecoveryRequired     = "recovery_required"

	DispositionAccepted     = "accepted"
	DispositionWorkerFailed = "worker_failed"
	DispositionCancelled    = "cancelled"

	OutcomeCompleted = "completed"
	OutcomeFailed    = "failed"

	OperationWorktreeCreate = "worktree_create"
	OperationTerminalCreate = "terminal_create"
	OperationTaskCreate     = "task_create"
	OperationDispatch       = "dispatch"
)

type Fence struct {
	Attempt        int
	OwnershipEpoch string
	ContextSHA256  string
}

type PrepareRequest struct {
	Attempt         int
	OwnershipEpoch  string
	CoordinatorRoot string
	WorkerRoot      string
	Agent           string
	Now             string
}

type ClaimRequest struct {
	Fence      Fence
	Worker     model.IssueOpsHostSessionIdentity
	WorkerRoot string
	Now        string
}

type FinishRequest struct {
	Fence  Fence
	Worker model.IssueOpsHostSessionIdentity
	Result model.IssueOpsExecutionHandoffResult
	Now    string
}

type AcceptRequest struct {
	Fence     Fence
	FinalHead string
	Now       string
}

func Prepare(record model.IssueOpsRecord, req PrepareRequest) (model.IssueOpsRecord, error) {
	if record.ExecutionHandoff != nil {
		return record, fmt.Errorf("execution handoff already exists")
	}
	if req.Attempt < 1 {
		return record, fmt.Errorf("handoff attempt must be positive")
	}
	if strings.TrimSpace(req.OwnershipEpoch) == "" {
		return record, fmt.Errorf("ownership epoch is required")
	}
	prepared := record
	prepared.ExecutionHandoff = &model.IssueOpsExecutionHandoff{
		ProtocolVersion: ProtocolVersion,
		State:           StateCoordinatorPreparing,
		Attempt:         req.Attempt,
		OwnershipEpoch:  strings.TrimSpace(req.OwnershipEpoch),
		Driver:          "orca",
		Agent:           strings.TrimSpace(req.Agent),
		CoordinatorRoot: strings.TrimSpace(req.CoordinatorRoot),
		WorkerRoot:      strings.TrimSpace(req.WorkerRoot),
		PreparedAt:      req.Now,
		UpdatedAt:       req.Now,
	}
	return prepared, nil
}

func SetContext(record model.IssueOpsRecord, fence Fence, version int, sha, now string) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if handoff.State != StateCoordinatorPreparing {
		return record, fmt.Errorf("set context requires %s state", StateCoordinatorPreparing)
	}
	sha = strings.TrimSpace(sha)
	if version < 1 || len(sha) != 64 {
		return record, fmt.Errorf("valid context version and sha256 are required")
	}
	handoff.ContextVersion = version
	handoff.ContextSHA256 = sha
	handoff.UpdatedAt = now
	return updated, nil
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
		if reflect.DeepEqual(handoff.PendingOperation, &cleanPending) {
			return updated, nil
		}
		return record, fmt.Errorf("pending operation %s requires recovery", handoff.PendingOperation.Kind)
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
	if handoff.State != StateCoordinatorPreparing {
		return record, fmt.Errorf("dispatch requires %s state", StateCoordinatorPreparing)
	}
	cleanIdentity := cloneOrca(identity)
	handoff.Orca = &cleanIdentity
	handoff.State = StateDispatched
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
	if handoff.State == StateClaimed {
		if handoff.WorkerSession != nil && *handoff.WorkerSession == worker {
			return updated, nil
		}
		return record, fmt.Errorf("handoff is already claimed by another worker")
	}
	if handoff.State != StateDispatched {
		return record, fmt.Errorf("claim requires %s state", StateDispatched)
	}
	handoff.WorkerSession = &worker
	handoff.State = StateClaimed
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
	if handoff.State != StateClaimed || handoff.WorkerSession == nil || *handoff.WorkerSession != cleanSession(worker) {
		return record, fmt.Errorf("heartbeat requires the claimed worker")
	}
	handoff.LastHeartbeatAt = now
	handoff.UpdatedAt = now
	return updated, nil
}

func Finish(record model.IssueOpsRecord, req FinishRequest) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, req.Fence, true)
	if err != nil {
		return record, err
	}
	cleanResult := cloneResult(req.Result)
	if handoff.Result != nil && reflect.DeepEqual(handoff.Result, &cleanResult) {
		return updated, nil
	}
	if handoff.State != StateClaimed || handoff.WorkerSession == nil || *handoff.WorkerSession != cleanSession(req.Worker) {
		return record, fmt.Errorf("finish requires the claimed worker")
	}
	switch cleanResult.Outcome {
	case OutcomeCompleted:
		if strings.TrimSpace(cleanResult.FinalHead) == "" || len(cleanResult.Verification) == 0 || len(cleanResult.CleanupReceipts) == 0 {
			return record, fmt.Errorf("completed finish requires head, verification, and cleanup receipts")
		}
		handoff.State = StateSubmitted
	case OutcomeFailed:
		handoff.State = StateClosed
		handoff.ClosedDisposition = DispositionWorkerFailed
	default:
		return record, fmt.Errorf("unsupported handoff outcome %q", cleanResult.Outcome)
	}
	handoff.Result = &cleanResult
	handoff.CompletedAt = req.Now
	handoff.UpdatedAt = req.Now
	return updated, nil
}

func Accept(record model.IssueOpsRecord, req AcceptRequest) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, req.Fence, true)
	if err != nil {
		return record, err
	}
	if handoff.State == StateClosed && handoff.ClosedDisposition == DispositionAccepted && handoff.Result != nil && handoff.Result.FinalHead == strings.TrimSpace(req.FinalHead) {
		return updated, nil
	}
	if handoff.State != StateSubmitted || handoff.Result == nil {
		return record, fmt.Errorf("accept requires %s state", StateSubmitted)
	}
	if strings.TrimSpace(req.FinalHead) == "" || strings.TrimSpace(req.FinalHead) != strings.TrimSpace(handoff.Result.FinalHead) {
		return record, fmt.Errorf("accepted head does not match submitted result")
	}
	handoff.State = StateClosed
	handoff.ClosedDisposition = DispositionAccepted
	handoff.AcceptedAt = req.Now
	handoff.UpdatedAt = req.Now
	return updated, nil
}

func MarkRecoveryRequired(record model.IssueOpsRecord, fence Fence, failure model.IssueOpsExecutionHandoffFailure) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	cleanFailure := failure
	cleanFailure.Code = strings.TrimSpace(cleanFailure.Code)
	cleanFailure.Message = strings.TrimSpace(cleanFailure.Message)
	if cleanFailure.Code == "" {
		return record, fmt.Errorf("recovery failure code is required")
	}
	handoff.State = StateRecoveryRequired
	handoff.Failure = &cleanFailure
	handoff.UpdatedAt = failure.At
	return updated, nil
}

func fencedCopy(record model.IssueOpsRecord, fence Fence, requireContext bool) (model.IssueOpsRecord, *model.IssueOpsExecutionHandoff, error) {
	if record.ExecutionHandoff == nil {
		return record, nil, fmt.Errorf("execution handoff is required")
	}
	updated := record
	copyHandoff := cloneHandoff(*record.ExecutionHandoff)
	updated.ExecutionHandoff = &copyHandoff
	if copyHandoff.Attempt != fence.Attempt || copyHandoff.OwnershipEpoch != strings.TrimSpace(fence.OwnershipEpoch) {
		return record, nil, fmt.Errorf("stale handoff attempt or ownership epoch")
	}
	if requireContext && (copyHandoff.ContextSHA256 == "" || copyHandoff.ContextSHA256 != strings.TrimSpace(fence.ContextSHA256)) {
		return record, nil, fmt.Errorf("stale handoff context")
	}
	return updated, updated.ExecutionHandoff, nil
}

func cloneHandoff(value model.IssueOpsExecutionHandoff) model.IssueOpsExecutionHandoff {
	cloned := value
	if value.WorkerSession != nil {
		v := *value.WorkerSession
		cloned.WorkerSession = &v
	}
	if value.Orca != nil {
		v := cloneOrca(*value.Orca)
		cloned.Orca = &v
	}
	if value.PendingOperation != nil {
		v := clonePending(*value.PendingOperation)
		cloned.PendingOperation = &v
	}
	if value.Result != nil {
		v := cloneResult(*value.Result)
		cloned.Result = &v
	}
	if value.Failure != nil {
		v := *value.Failure
		cloned.Failure = &v
	}
	return cloned
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

func cloneResult(value model.IssueOpsExecutionHandoffResult) model.IssueOpsExecutionHandoffResult {
	value.ChangedFiles = append([]string(nil), value.ChangedFiles...)
	value.Verification = append([]string(nil), value.Verification...)
	value.CleanupReceipts = append([]string(nil), value.CleanupReceipts...)
	return value
}

func cleanSession(value model.IssueOpsHostSessionIdentity) model.IssueOpsHostSessionIdentity {
	value.Host = strings.TrimSpace(value.Host)
	value.SessionID = strings.TrimSpace(value.SessionID)
	value.AgentID = strings.TrimSpace(value.AgentID)
	return value
}
