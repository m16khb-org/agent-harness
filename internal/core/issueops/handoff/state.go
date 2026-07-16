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

type PrepareRequest struct {
	Attempt         int
	OwnershipEpoch  string
	AttemptBaseHead string
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
	if strings.TrimSpace(req.AttemptBaseHead) == "" {
		return record, fmt.Errorf("attempt base head is required")
	}
	agent, err := NormalizeAgent(req.Agent)
	if err != nil {
		return record, err
	}
	prepared := record
	prepared.ExecutionHandoff = &model.IssueOpsExecutionHandoff{
		ProtocolVersion: ProtocolVersion,
		State:           StateCoordinatorPreparing,
		Attempt:         req.Attempt,
		OwnershipEpoch:  strings.TrimSpace(req.OwnershipEpoch),
		AttemptBaseHead: strings.TrimSpace(req.AttemptBaseHead),
		Driver:          "orca",
		Agent:           agent,
		CoordinatorRoot: strings.TrimSpace(req.CoordinatorRoot),
		WorkerRoot:      strings.TrimSpace(req.WorkerRoot),
		PreparedAt:      req.Now,
		UpdatedAt:       req.Now,
	}
	return prepared, nil
}

func SetContext(record model.IssueOpsRecord, fence Fence, version int, sha, sourceSHA string, options model.IssueOpsExecutionHandoffContextOptions, now string) (model.IssueOpsRecord, error) {
	updated, handoff, err := fencedCopy(record, fence, false)
	if err != nil {
		return record, err
	}
	if handoff.State != StateCoordinatorPreparing {
		return record, fmt.Errorf("set context requires %s state", StateCoordinatorPreparing)
	}
	sha = strings.TrimSpace(sha)
	sourceSHA = strings.TrimSpace(sourceSHA)
	if version < 1 || len(sha) != 64 || len(sourceSHA) != 64 {
		return record, fmt.Errorf("valid context version, sha256, and source sha256 are required")
	}
	handoff.ContextVersion = version
	handoff.ContextSHA256 = sha
	handoff.ContextSourceSHA256 = sourceSHA
	canonicalOptions := CanonicalContextOptions(ContextOptionsFromModel(options))
	handoff.ContextOptions = cloneContextOptions(&canonicalOptions)
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
	if handoff.State != StateCoordinatorPreparing {
		return record, fmt.Errorf("dispatch requires %s state", StateCoordinatorPreparing)
	}
	cleanIdentity := cloneOrca(identity)
	handoff.Orca = &cleanIdentity
	handoff.DeliveryMode = "inject"
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
	if handoff.WorkerSession == nil || *handoff.WorkerSession != cleanSession(req.Worker) {
		return record, fmt.Errorf("finish requires the claimed worker")
	}
	if handoff.Result != nil && reflect.DeepEqual(handoff.Result, &cleanResult) {
		return updated, nil
	}
	if handoff.State != StateClaimed {
		return record, fmt.Errorf("finish requires the claimed worker")
	}
	switch cleanResult.Outcome {
	case OutcomeCompleted:
		candidate := *handoff
		candidate.Result = &cleanResult
		if !validCompletedResult(&candidate) {
			return record, fmt.Errorf("completed finish requires head, Turing report, verification, and cleanup receipts")
		}
		handoff.State = StateSubmitted
	case OutcomeFailed:
		candidate := *handoff
		candidate.Result = &cleanResult
		if !validFailedResult(&candidate) {
			return record, fmt.Errorf("failed finish requires exact Orca task and dispatch identity")
		}
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
	if handoff.State != StateCoordinatorPreparing || handoff.PendingOperation == nil {
		return record, fmt.Errorf("recovery requires coordinator_preparing with a pending operation")
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
	if (current.State != StateCoordinatorPreparing && current.State != StateRecoveryRequired) || current.PendingOperation == nil || current.PendingOperation.Kind != OperationWorktreeCreate {
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

func cloneResult(value model.IssueOpsExecutionHandoffResult) model.IssueOpsExecutionHandoffResult {
	value.Outcome = strings.TrimSpace(value.Outcome)
	value.FinalHead = strings.TrimSpace(value.FinalHead)
	value.TuringReportPath = strings.TrimSpace(value.TuringReportPath)
	value.EvidenceDigest = redact(value.EvidenceDigest)
	value.TaskID = strings.TrimSpace(value.TaskID)
	value.DispatchID = strings.TrimSpace(value.DispatchID)
	value.ChangedFiles = cleanChangedFileList(value.ChangedFiles)
	value.Verification = cleanResultList(value.Verification)
	value.CleanupReceipts = cleanResultList(value.CleanupReceipts)
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
	return clean
}

func cleanSession(value model.IssueOpsHostSessionIdentity) model.IssueOpsHostSessionIdentity {
	value.Host = strings.TrimSpace(value.Host)
	value.SessionID = strings.TrimSpace(value.SessionID)
	value.AgentID = strings.TrimSpace(value.AgentID)
	return value
}
