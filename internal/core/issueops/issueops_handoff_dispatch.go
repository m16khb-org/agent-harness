package issueops

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type IssueOpsHandoffStartRequest struct {
	ID      string                 `json:"id"`
	Confirm bool                   `json:"confirm,omitempty"`
	Context handoff.ContextOptions `json:"context,omitempty"`
}

type IssueOpsHandoffStartResult struct {
	OK            bool                  `json:"ok"`
	ID            string                `json:"id"`
	Preview       bool                  `json:"preview,omitempty"`
	State         string                `json:"state"`
	Disposition   string                `json:"disposition,omitempty"`
	Attempt       int                   `json:"attempt"`
	ContextSHA256 string                `json:"context_sha256,omitempty"`
	PlanSHA256    string                `json:"plan_sha256,omitempty"`
	RecoveryCode  string                `json:"recovery_code,omitempty"`
	Orca          *IssueOpsOrcaIdentity `json:"orca,omitempty"`
}

type IssueOpsHandoffStartClock struct {
	Now func() time.Time
}

type IssueOpsOrcaDispatchClient interface {
	ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
	CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error)
	RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error)
	ListTasks(context.Context) ([]port.OrcaTask, error)
	CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
	Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
	SendTerminal(context.Context, string, string) error
}

func IssueOpsPreDispatchReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := make([]string, 0, len(ready.Missing))
	for _, item := range ready.Missing {
		if item != "handoff_worker_claim" {
			missing = append(missing, item)
		}
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
		missing = append(missing, "handoff_coordinator_preparing")
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil || strings.TrimSpace(record.ExecutionHandoff.Orca.WorktreeID) == "" {
		missing = append(missing, "handoff_orca_worktree")
	}
	ready.Missing = uniqueStrings(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func StartIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffStartRequest, client IssueOpsOrcaDispatchClient, clock IssueOpsHandoffStartClock) (IssueOpsHandoffStartResult, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	if record.ExecutionHandoff == nil {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("execution handoff is required")
	}
	if record.ExecutionHandoff.State == handoff.StateDispatched || record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
		return projectHandoffStart(record, false, ""), nil
	}
	if record.ExecutionHandoff.PendingOperation != nil {
		fence := handoffFence(record)
		now := issueOpsHandoffStartNow(clock)
		if err := markHandoffPrepareRecovery(stateRoot, record.ID, fence, "pending_operation_requires_recovery", "start observed an unresolved external mutation", now); err != nil {
			return IssueOpsHandoffStartResult{}, err
		}
		record, err = ReadIssueOps(stateRoot, record.ID)
		return projectHandoffStart(record, false, ""), err
	}
	readiness := IssueOpsPreDispatchReadiness(record)
	if !readiness.Ready {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("handoff pre-dispatch readiness missing: %s", strings.Join(readiness.Missing, ", "))
	}
	packet, err := handoff.BuildContext(record, req.Context)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	if !req.Confirm {
		result := projectHandoffStart(record, true, packet.PlanSHA256)
		result.ContextSHA256 = packet.SHA256
		return result, nil
	}
	if client == nil {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca dispatch dependency is unavailable")
	}
	now := issueOpsHandoffStartNow(clock)
	record, err = persistHandoffContext(stateRoot, record.ID, packet, now)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	fence := handoffFence(record)

	workerPTYID := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerPTYID)
	workerHandle := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerMailboxHandle)
	liveHandle := workerHandle
	if (workerPTYID == "") != (workerHandle == "") {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("persisted Orca terminal checkpoint is incomplete")
	}
	if workerPTYID == "" {
		terminals, listErr := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
		if listErr != nil {
			return IssueOpsHandoffStartResult{}, fmt.Errorf("list terminals before create: %w", listErr)
		}
		record, err = beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{
			Kind: handoff.OperationTerminalCreate, StartedAt: now, BaselinePTYIDs: terminalPTYIDs(terminals),
		})
		if err != nil {
			return IssueOpsHandoffStartResult{}, err
		}
		terminal, createErr := client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
			WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, Agent: record.ExecutionHandoff.Agent,
			Title: issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
		})
		if createErr != nil {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_create_ambiguous", createErr.Error(), now)
			return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca terminal create requires recovery: %w", createErr)
		}
		if terminal.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || terminal.PTYID == "" || terminal.Handle == "" || !terminal.Connected || !terminal.Writable {
			err = fmt.Errorf("Orca terminal identity does not match the prepared worktree")
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now)
			return IssueOpsHandoffStartResult{}, err
		}
		record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTerminalCreate, now, func(h *IssueOpsExecutionHandoff) error {
			h.Orca.TerminalBaselinePTYIDs = terminalPTYIDs(terminals)
			h.Orca.WorkerPTYID = terminal.PTYID
			h.Orca.WorkerMailboxHandle = terminal.Handle
			return nil
		})
		if err != nil {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_persist_failed", err.Error(), now)
			return IssueOpsHandoffStartResult{}, err
		}
		liveHandle = terminal.Handle
	} else {
		terminal, refreshErr := client.RefreshTerminal(ctx, record.ExecutionHandoff.Orca.WorktreeID, workerPTYID)
		if refreshErr != nil {
			return IssueOpsHandoffStartResult{}, fmt.Errorf("refresh persisted Orca terminal: %w", refreshErr)
		}
		if terminal.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || terminal.PTYID != workerPTYID || strings.TrimSpace(terminal.Handle) == "" || !terminal.Connected || !terminal.Writable {
			return IssueOpsHandoffStartResult{}, fmt.Errorf("refreshed Orca terminal identity does not match the persisted checkpoint")
		}
		liveHandle = strings.TrimSpace(terminal.Handle)
	}

	if strings.TrimSpace(record.ExecutionHandoff.Orca.TaskID) == "" {
		tasks, listErr := client.ListTasks(ctx)
		if listErr != nil {
			return IssueOpsHandoffStartResult{}, fmt.Errorf("list tasks before create: %w", listErr)
		}
		record, err = beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{
			Kind: handoff.OperationTaskCreate, StartedAt: now, BaselineTaskIDs: taskIDs(tasks),
		})
		if err != nil {
			return IssueOpsHandoffStartResult{}, err
		}
		marker := issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
		task, createErr := client.CreateTask(ctx, port.OrcaCreateTaskRequest{Spec: packet.Markdown, Title: marker, DisplayName: record.Branch})
		if createErr != nil {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_create_ambiguous", createErr.Error(), now)
			return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca task create requires recovery: %w", createErr)
		}
		if strings.TrimSpace(task.ID) == "" {
			err = fmt.Errorf("Orca task identity is empty")
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_identity_mismatch", err.Error(), now)
			return IssueOpsHandoffStartResult{}, err
		}
		record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTaskCreate, now, func(h *IssueOpsExecutionHandoff) error {
			h.Orca.TaskID = task.ID
			return nil
		})
		if err != nil {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_persist_failed", err.Error(), now)
			return IssueOpsHandoffStartResult{}, err
		}
	}

	record, err = beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, StartedAt: now})
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	dispatched, err := client.Dispatch(ctx, port.OrcaDispatchRequest{
		TaskID: record.ExecutionHandoff.Orca.TaskID, ToHandle: liveHandle, Inject: true, ReturnPreamble: true,
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_ambiguous", err.Error(), now)
		return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca dispatch requires recovery: %w", err)
	}
	if dispatched.ID == "" || dispatched.TaskID != record.ExecutionHandoff.Orca.TaskID || dispatched.AssigneeHandle != liveHandle || !dispatched.Injected {
		err = fmt.Errorf("Orca dispatch identity does not match persisted task and worker mailbox")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_identity_mismatch", err.Error(), now)
		return IssueOpsHandoffStartResult{}, err
	}
	record, err = finalizeHandoffDispatch(stateRoot, record.ID, fence, dispatched.ID, dispatched.AssigneeHandle, now)
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_persist_failed", err.Error(), now)
		return IssueOpsHandoffStartResult{}, err
	}
	return projectHandoffStart(record, false, packet.PlanSHA256), nil
}

func persistHandoffContext(stateRoot, id string, packet handoff.ContextPacket, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff.ContextSHA256 != "" {
			if record.ExecutionHandoff.ContextSHA256 != packet.SHA256 || record.ExecutionHandoff.ContextVersion != packet.Version {
				return fmt.Errorf("persisted handoff context differs from current context")
			}
			persisted = record
			return nil
		}
		record, err = handoff.SetContext(record, handoffFence(record), packet.Version, packet.SHA256, now)
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func beginHandoffOperation(stateRoot, id string, fence handoff.Fence, pending model.IssueOpsExecutionHandoffPendingOperation) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		record, err = handoff.BeginOperation(record, fence, pending)
		if err != nil {
			return err
		}
		record.UpdatedAt = pending.StartedAt
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func completeHandoffOperation(stateRoot, id string, fence handoff.Fence, kind, now string, update func(*IssueOpsExecutionHandoff) error) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
			return fmt.Errorf("handoff is no longer coordinator_preparing")
		}
		if update != nil {
			if err := update(record.ExecutionHandoff); err != nil {
				return err
			}
		}
		record, err = handoff.CompleteOperation(record, fence, kind, now)
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func finalizeHandoffDispatch(stateRoot, id string, fence handoff.Fence, dispatchID, assigneeHandle, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
			return fmt.Errorf("handoff is no longer coordinator_preparing")
		}
		identity := *record.ExecutionHandoff.Orca
		identity.DispatchID = dispatchID
		identity.WorkerMailboxHandle = strings.TrimSpace(assigneeHandle)
		record, err = handoff.CompleteOperation(record, fence, handoff.OperationDispatch, now)
		if err != nil {
			return err
		}
		record, err = handoff.Dispatch(record, fence, identity, now)
		if err != nil {
			return err
		}
		record.ExecutionHandoff.DeliveryMode = "inject"
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func projectHandoffStart(record IssueOpsRecord, preview bool, planSHA string) IssueOpsHandoffStartResult {
	result := IssueOpsHandoffStartResult{OK: true, ID: record.ID, Preview: preview, PlanSHA256: planSHA}
	if record.ExecutionHandoff == nil {
		return result
	}
	result.State = record.ExecutionHandoff.State
	result.Disposition = record.ExecutionHandoff.ClosedDisposition
	result.Attempt = record.ExecutionHandoff.Attempt
	result.ContextSHA256 = record.ExecutionHandoff.ContextSHA256
	result.Orca = record.ExecutionHandoff.Orca
	if record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
		result.RecoveryCode = "explicit_reconcile_required"
		if record.ExecutionHandoff.Failure != nil && record.ExecutionHandoff.Failure.Code != "" {
			result.RecoveryCode = record.ExecutionHandoff.Failure.Code
		}
	}
	return result
}

func ReconcileIssueOpsHandoffTerminal(baseline []string, worktreeID string, rows []port.OrcaTerminal) (port.OrcaTerminal, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTerminal, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.PTYID]; existed || row.PTYID == "" {
			continue
		}
		if row.WorktreeID == strings.TrimSpace(worktreeID) && strings.TrimSpace(row.Handle) != "" && row.Connected && row.Writable {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTerminal{}, fmt.Errorf("terminal recovery requires exactly one PTY delta; found %d", len(candidates))
	}
	return candidates[0], nil
}

func ReconcileIssueOpsHandoffTask(baseline []string, marker string, rows []port.OrcaTask) (port.OrcaTask, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTask, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.ID]; existed || row.ID == "" {
			continue
		}
		if strings.Contains(row.Title, marker) || strings.Contains(row.DisplayName, marker) {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTask{}, fmt.Errorf("task recovery requires exactly one marker candidate; found %d", len(candidates))
	}
	return candidates[0], nil
}

func ReconcileIssueOpsHandoffDispatch(ctx context.Context, taskID string, client interface {
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
}) (port.OrcaDispatch, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return port.OrcaDispatch{}, fmt.Errorf("persisted task id is required for dispatch recovery")
	}
	dispatch, err := client.ShowDispatch(ctx, taskID)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if dispatch.ID == "" || dispatch.TaskID != taskID {
		return port.OrcaDispatch{}, fmt.Errorf("dispatch recovery result does not match persisted task")
	}
	return dispatch, nil
}

func handoffFence(record IssueOpsRecord) handoff.Fence {
	if record.ExecutionHandoff == nil {
		return handoff.Fence{}
	}
	return handoff.Fence{Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256}
}

func issueOpsHandoffStartNow(clock IssueOpsHandoffStartClock) string {
	if clock.Now != nil {
		return clock.Now().UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func terminalPTYIDs(rows []port.OrcaTerminal) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PTYID != "" {
			values = append(values, row.PTYID)
		}
	}
	sort.Strings(values)
	return values
}

func taskIDs(rows []port.OrcaTask) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			values = append(values, row.ID)
		}
	}
	sort.Strings(values)
	return values
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
