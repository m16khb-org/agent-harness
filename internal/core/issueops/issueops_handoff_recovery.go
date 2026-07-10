package issueops

import (
	"context"
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type IssueOpsHandoffRecoverRequest struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Confirm bool   `json:"confirm,omitempty"`
}

type IssueOpsHandoffRecoverResult struct {
	OK           bool                  `json:"ok"`
	ID           string                `json:"id"`
	Action       string                `json:"action"`
	State        string                `json:"state"`
	Disposition  string                `json:"disposition,omitempty"`
	Attempt      int                   `json:"attempt"`
	RecoveryCode string                `json:"recovery_code,omitempty"`
	NextCommand  string                `json:"next_command,omitempty"`
	Orca         *IssueOpsOrcaIdentity `json:"orca,omitempty"`
}

func RecoverIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffRecoverRequest, client any, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffRecoverResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "reconcile":
		return reconcileIssueOpsHandoff(ctx, stateRoot, req.ID, client, issueOpsHandoffNow(clock))
	case "cancel":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("cancel requires --confirm")
		}
		return cancelIssueOpsHandoff(stateRoot, req.ID, issueOpsHandoffNow(clock))
	case "retry":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires --confirm")
		}
		return retryIssueOpsHandoff(stateRoot, req.ID, clock)
	default:
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("recovery action must be reconcile, cancel, or retry")
	}
}

func cancelIssueOpsHandoff(stateRoot, id, now string) (IssueOpsHandoffRecoverResult, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil {
			return fmt.Errorf("execution handoff is required")
		}
		if record.ExecutionHandoff.State == handoff.StateClosed {
			if record.ExecutionHandoff.ClosedDisposition != handoff.DispositionCancelled {
				return fmt.Errorf("closed handoff cannot be cancelled")
			}
			persisted = record
			return nil
		}
		record.ExecutionHandoff.State = handoff.StateClosed
		record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
		record.ExecutionHandoff.PendingOperation = nil
		record.ExecutionHandoff.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return projectHandoffRecovery(persisted, "cancel", ""), err
}

func retryIssueOpsHandoff(stateRoot, id string, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffRecoverResult, error) {
	epoch, err := issueOpsHandoffEpoch(clock)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	now := issueOpsHandoffNow(clock)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, id, func() error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		old := record.ExecutionHandoff
		if old == nil || old.State != handoff.StateClosed {
			return fmt.Errorf("retry requires a safely closed prior attempt")
		}
		if old.PendingOperation != nil {
			return fmt.Errorf("retry requires every ambiguous operation to be reconciled")
		}
		var worktreeIdentity *model.IssueOpsOrcaIdentity
		if old.Orca != nil && old.Orca.WorktreeID != "" {
			worktreeIdentity = &model.IssueOpsOrcaIdentity{
				RuntimeID: old.Orca.RuntimeID, WorktreeID: old.Orca.WorktreeID, WorktreeInstanceID: old.Orca.WorktreeInstanceID, WorktreePath: old.Orca.WorktreePath,
			}
		}
		record.ExecutionHandoff = &model.IssueOpsExecutionHandoff{
			ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateCoordinatorPreparing,
			Attempt: old.Attempt + 1, OwnershipEpoch: epoch, Driver: "orca", Agent: old.Agent,
			CoordinatorRoot: old.CoordinatorRoot, WorkerRoot: old.WorkerRoot, Orca: worktreeIdentity,
			PreparedAt: now, ProvisionedAt: now, UpdatedAt: now,
		}
		record.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, record)
		return readErr
	})
	return projectHandoffRecovery(persisted, "retry", "agent-harness issueops handoff start --id "+id+" --confirm"), err
}

func reconcileIssueOpsHandoff(ctx context.Context, stateRoot, id string, client any, now string) (IssueOpsHandoffRecoverResult, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateRecoveryRequired || record.ExecutionHandoff.PendingOperation == nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("reconcile requires recovery_required with a pending operation")
	}
	pending := *record.ExecutionHandoff.PendingOperation
	identity := record.ExecutionHandoff.Orca
	if identity == nil {
		identity = &model.IssueOpsOrcaIdentity{}
	}
	next := "agent-harness issueops handoff start --id " + id + " --confirm"
	newState := handoff.StateCoordinatorPreparing
	switch pending.Kind {
	case handoff.OperationWorktreeCreate:
		reader, ok := client.(interface {
			ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
		})
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca worktree recovery dependency is unavailable")
		}
		rows, listErr := reader.ListWorktrees(ctx, record.Repo)
		if listErr != nil {
			return IssueOpsHandoffRecoverResult{}, listErr
		}
		candidate, matchErr := ReconcileIssueOpsHandoffWorktree(pending, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		if validateErr := validateCreatedHandoffWorktree(record, record.ExecutionHandoff.WorkerRoot, candidate); validateErr != nil {
			return IssueOpsHandoffRecoverResult{}, validateErr
		}
		identity.WorktreeID, identity.WorktreeInstanceID, identity.WorktreePath = candidate.ID, candidate.InstanceID, candidate.Path
		record.WorktreePath = candidate.Path
	case handoff.OperationTerminalCreate:
		reader, ok := client.(interface {
			ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
		})
		if !ok || identity.WorktreeID == "" {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca terminal recovery dependency and worktree id are required")
		}
		rows, listErr := reader.ListTerminals(ctx, identity.WorktreeID)
		if listErr != nil {
			return IssueOpsHandoffRecoverResult{}, listErr
		}
		candidate, matchErr := ReconcileIssueOpsHandoffTerminal(pending.BaselinePTYIDs, identity.WorktreeID, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.TerminalBaselinePTYIDs = append([]string(nil), pending.BaselinePTYIDs...)
		identity.WorkerPTYID, identity.WorkerMailboxHandle = candidate.PTYID, candidate.Handle
	case handoff.OperationTaskCreate:
		reader, ok := client.(interface {
			ListTasks(context.Context) ([]port.OrcaTask, error)
		})
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca task recovery dependency is unavailable")
		}
		rows, listErr := reader.ListTasks(ctx)
		if listErr != nil {
			return IssueOpsHandoffRecoverResult{}, listErr
		}
		marker := issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
		candidate, matchErr := ReconcileIssueOpsHandoffTask(pending.BaselineTaskIDs, marker, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.TaskID = candidate.ID
	case handoff.OperationDispatch:
		reader, ok := client.(interface {
			ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
		})
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca dispatch recovery dependency is unavailable")
		}
		candidate, matchErr := ReconcileIssueOpsHandoffDispatch(ctx, identity.TaskID, reader)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.DispatchID, identity.WorkerMailboxHandle = candidate.ID, candidate.AssigneeHandle
		newState = handoff.StateDispatched
		next = "agent-harness issueops handoff claim --id " + id
	default:
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("unsupported pending operation %q", pending.Kind)
	}

	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, id, func() error {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if current.ExecutionHandoff == nil || current.ExecutionHandoff.State != handoff.StateRecoveryRequired || current.ExecutionHandoff.PendingOperation == nil || current.ExecutionHandoff.PendingOperation.Kind != pending.Kind || current.ExecutionHandoff.Attempt != record.ExecutionHandoff.Attempt || current.ExecutionHandoff.OwnershipEpoch != record.ExecutionHandoff.OwnershipEpoch {
			return fmt.Errorf("stale reconciliation result")
		}
		current.ExecutionHandoff.Orca = identity
		current.ExecutionHandoff.PendingOperation = nil
		current.ExecutionHandoff.State = newState
		current.ExecutionHandoff.Failure = nil
		current.ExecutionHandoff.UpdatedAt = now
		current.WorktreePath = record.WorktreePath
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return projectHandoffRecovery(persisted, "reconcile", next), err
}

func projectHandoffRecovery(record IssueOpsRecord, action, next string) IssueOpsHandoffRecoverResult {
	result := IssueOpsHandoffRecoverResult{OK: record.OK, ID: record.ID, Action: action, NextCommand: next}
	if record.ExecutionHandoff == nil {
		return result
	}
	result.State = record.ExecutionHandoff.State
	result.Disposition = record.ExecutionHandoff.ClosedDisposition
	result.Attempt = record.ExecutionHandoff.Attempt
	result.Orca = record.ExecutionHandoff.Orca
	if record.ExecutionHandoff.Failure != nil {
		result.RecoveryCode = record.ExecutionHandoff.Failure.Code
	}
	return result
}
