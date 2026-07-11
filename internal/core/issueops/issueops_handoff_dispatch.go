package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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
	OK                           bool                  `json:"ok"`
	ID                           string                `json:"id"`
	Preview                      bool                  `json:"preview,omitempty"`
	State                        string                `json:"state"`
	Disposition                  string                `json:"disposition,omitempty"`
	Attempt                      int                   `json:"attempt"`
	ContextSHA256                string                `json:"context_sha256,omitempty"`
	PlanSHA256                   string                `json:"plan_sha256,omitempty"`
	RecoveryCode                 string                `json:"recovery_code,omitempty"`
	CodexHookTrustBypassRequired bool                  `json:"codex_hook_trust_bypass_required"`
	CodexHookTrustBypassAttested bool                  `json:"codex_hook_trust_bypass_attested"`
	Orca                         *IssueOpsOrcaIdentity `json:"orca,omitempty"`
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
	if record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
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
	contextOptions, err := resolveHandoffContextOptions(record, req.Context)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	packet, err := handoff.BuildContext(record, contextOptions)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	if !req.Confirm {
		result := projectHandoffStart(record, true, packet.PlanSHA256)
		result.ContextSHA256 = packet.SHA256
		result.CodexHookTrustBypassAttested = contextOptions.AllowCodexHookTrustBypass
		return result, nil
	}
	if codexHookTrustBypassRequired(record) && !contextOptions.AllowCodexHookTrustBypass {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("confirmed supervised Codex start requires --allow-codex-hook-trust-bypass after the documented hooks/list attestation")
	}
	if client == nil {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca dispatch dependency is unavailable")
	}
	now := issueOpsHandoffStartNow(clock)
	record, err = persistHandoffContext(stateRoot, record.ID, packet, handoff.CanonicalContextOptions(contextOptions), now)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	fence := handoffFence(record)

	record, liveHandle, err := ensureHandoffTerminal(ctx, stateRoot, record, fence, client, now)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	record, err = ensureHandoffTask(ctx, stateRoot, record, fence, client, packet.Markdown, now)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	record, err = dispatchHandoff(ctx, stateRoot, record, fence, client, liveHandle, now)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	return projectHandoffStart(record, false, packet.PlanSHA256), nil
}

func ensureHandoffTerminal(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, now string) (IssueOpsRecord, string, error) {
	workerPTYID := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerPTYID)
	workerHandle := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerMailboxHandle)
	if (workerPTYID == "") != (workerHandle == "") {
		return record, "", fmt.Errorf("persisted Orca terminal checkpoint is incomplete")
	}
	if workerPTYID == "" {
		return createHandoffTerminal(ctx, stateRoot, record, fence, client, now)
	}
	terminal, err := client.RefreshTerminal(ctx, record.ExecutionHandoff.Orca.WorktreeID, workerPTYID)
	if err != nil {
		return record, "", fmt.Errorf("refresh persisted Orca terminal: %w", err)
	}
	if terminal.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || terminal.PTYID != workerPTYID || strings.TrimSpace(terminal.Handle) == "" || !terminal.Connected || !terminal.Writable || !terminalWorktreePathMatches(terminal, record.ExecutionHandoff.WorkerRoot) {
		return record, "", fmt.Errorf("refreshed Orca terminal identity does not match the persisted checkpoint")
	}
	return record, strings.TrimSpace(terminal.Handle), nil
}

func createHandoffTerminal(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, now string) (IssueOpsRecord, string, error) {
	terminals, err := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
	if err != nil {
		return record, "", fmt.Errorf("list terminals before create: %w", err)
	}
	baseline, err := handoff.CanonicalBaselineIDs("terminal", terminalPTYIDs(terminals))
	if err != nil {
		return record, "", fmt.Errorf("Orca terminal baseline is unsafe: %w", err)
	}
	if err := handoff.RequireBaselineDeltaHeadroom("terminal", baseline); err != nil {
		return record, "", fmt.Errorf("Orca terminal baseline is unsafe: %w", err)
	}
	record, err = beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTerminalCreate, StartedAt: now, BaselinePTYIDs: baseline,
	})
	if err != nil {
		return record, "", err
	}
	created, err := client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
		Agent:      record.ExecutionHandoff.Agent,
		Title:      issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
		AllowCodexHookTrustBypass: record.ExecutionHandoff.ContextOptions != nil &&
			record.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass,
	})
	if err != nil {
		if externalMutationNotInvoked(err) {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTerminalCreate, now, nil)
			if clearErr != nil {
				return record, "", fmt.Errorf("clear non-invoked terminal create journal: %w", clearErr)
			}
			return cleared, "", fmt.Errorf("Orca terminal create was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_create_ambiguous", err.Error(), now)
		return record, "", fmt.Errorf("Orca terminal create requires recovery: %w", err)
	}
	if created.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || strings.TrimSpace(created.Handle) == "" {
		err = fmt.Errorf("Orca terminal identity does not match the prepared worktree")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now)
		return record, "", err
	}
	currentTerminals, err := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
	if err != nil {
		err = fmt.Errorf("list terminals after create: %w", err)
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now)
		return record, "", err
	}
	terminal, err := ReconcileIssueOpsHandoffTerminal(baseline, record.ExecutionHandoff.Orca.WorktreeID, record.ExecutionHandoff.WorkerRoot, currentTerminals)
	if err != nil {
		err = fmt.Errorf("reconcile created terminal: %w", err)
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now)
		return record, "", err
	}
	if createdPTYID := strings.TrimSpace(created.PTYID); createdPTYID != "" && terminal.PTYID != createdPTYID {
		err = fmt.Errorf("created terminal PTY does not match the exact terminal-list delta")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now)
		return record, "", err
	}
	record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTerminalCreate, now, func(h *IssueOpsExecutionHandoff) error {
		h.Orca.TerminalBaselinePTYIDs = baseline
		h.Orca.WorkerPTYID = terminal.PTYID
		h.Orca.WorkerMailboxHandle = terminal.Handle
		return nil
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_persist_failed", err.Error(), now)
		return record, "", err
	}
	return record, terminal.Handle, nil
}

func ensureHandoffTask(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, contextMarkdown, now string) (IssueOpsRecord, error) {
	if strings.TrimSpace(record.ExecutionHandoff.Orca.TaskID) != "" {
		return record, nil
	}
	tasks, err := client.ListTasks(ctx)
	if err != nil {
		return record, fmt.Errorf("list tasks before create: %w", err)
	}
	baseline, err := handoff.CanonicalBaselineIDs("task", taskIDs(tasks))
	if err != nil {
		return record, fmt.Errorf("Orca task baseline is unsafe: %w", err)
	}
	if err := handoff.RequireBaselineDeltaHeadroom("task", baseline); err != nil {
		return record, fmt.Errorf("Orca task baseline is unsafe: %w", err)
	}
	record, err = beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, StartedAt: now, BaselineTaskIDs: baseline,
	})
	if err != nil {
		return record, err
	}
	title, displayName, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		return record, err
	}
	task, err := client.CreateTask(ctx, port.OrcaCreateTaskRequest{Spec: contextMarkdown, Title: title, DisplayName: displayName})
	if err != nil {
		if externalMutationNotInvoked(err) {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTaskCreate, now, nil)
			if clearErr != nil {
				return record, fmt.Errorf("clear non-invoked task create journal: %w", clearErr)
			}
			return cleared, fmt.Errorf("Orca task create was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_create_ambiguous", err.Error(), now)
		return record, fmt.Errorf("Orca task create requires recovery: %w", err)
	}
	if strings.TrimSpace(task.ID) == "" || task.Title != title || task.DisplayName != displayName || !validInitialOrcaTaskStatus(task.Status) {
		err = fmt.Errorf("Orca task identity, marker, display name, or initial status does not match the prepared task")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_identity_mismatch", err.Error(), now)
		return record, err
	}
	record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTaskCreate, now, func(h *IssueOpsExecutionHandoff) error {
		h.Orca.TaskID = task.ID
		return nil
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_persist_failed", err.Error(), now)
	}
	return record, err
}

func dispatchHandoff(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, liveHandle, now string) (IssueOpsRecord, error) {
	record, err := beginHandoffOperation(stateRoot, record.ID, fence, model.IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, StartedAt: now})
	if err != nil {
		return record, err
	}
	dispatched, err := client.Dispatch(ctx, port.OrcaDispatchRequest{
		TaskID: record.ExecutionHandoff.Orca.TaskID, ToHandle: liveHandle, Inject: true, ReturnPreamble: true,
	})
	if err != nil {
		if externalMutationNotInvoked(err) {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationDispatch, now, nil)
			if clearErr != nil {
				return record, fmt.Errorf("clear non-invoked dispatch journal: %w", clearErr)
			}
			return cleared, fmt.Errorf("Orca dispatch was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_ambiguous", err.Error(), now)
		return record, fmt.Errorf("Orca dispatch requires recovery: %w", err)
	}
	if dispatched.ID == "" || dispatched.TaskID != record.ExecutionHandoff.Orca.TaskID || dispatched.AssigneeHandle != liveHandle || !dispatched.Injected {
		err = fmt.Errorf("Orca dispatch identity does not match persisted task and worker mailbox")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_identity_mismatch", err.Error(), now)
		return record, err
	}
	record, err = finalizeHandoffDispatch(stateRoot, record.ID, fence, dispatched.ID, dispatched.AssigneeHandle, now)
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_persist_failed", err.Error(), now)
	}
	return record, err
}

func persistHandoffContext(stateRoot, id string, packet handoff.ContextPacket, options model.IssueOpsExecutionHandoffContextOptions, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff.ContextSHA256 != "" {
			persistedOptions := model.IssueOpsExecutionHandoffContextOptions{}
			if record.ExecutionHandoff.ContextOptions != nil {
				persistedOptions = handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(*record.ExecutionHandoff.ContextOptions))
			}
			if record.ExecutionHandoff.ContextSHA256 != packet.SHA256 || record.ExecutionHandoff.ContextSourceSHA256 != packet.SourceSHA256 || record.ExecutionHandoff.ContextVersion != packet.Version || record.ExecutionHandoff.ContextOptions == nil || !reflect.DeepEqual(persistedOptions, handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(options))) {
				return fmt.Errorf("persisted handoff context differs from current context")
			}
			persisted = record
			return nil
		}
		currentSourceSHA, err := handoff.ContextSourceSHA256(record)
		if err != nil {
			return fmt.Errorf("re-render handoff context source before persist: %w", err)
		}
		if currentSourceSHA != packet.SourceSHA256 {
			return fmt.Errorf("stale handoff context source changed before persist")
		}
		record, err = handoff.SetContext(record, handoffFence(record), packet.Version, packet.SHA256, packet.SourceSHA256, options, now)
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func resolveHandoffContextOptions(record IssueOpsRecord, supplied handoff.ContextOptions) (handoff.ContextOptions, error) {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.ContextOptions == nil {
		return supplied, nil
	}
	persisted := *record.ExecutionHandoff.ContextOptions
	if handoff.ContextOptionsEmpty(supplied) {
		return handoff.ContextOptionsFromModel(persisted), nil
	}
	if record.ExecutionHandoff.ContextSHA256 == "" && !persisted.AllowCodexHookTrustBypass && supplied.AllowCodexHookTrustBypass {
		expected := handoff.ContextOptionsFromModel(persisted)
		expected.AllowCodexHookTrustBypass = true
		withoutAttestation := supplied
		withoutAttestation.AllowCodexHookTrustBypass = false
		if handoff.ContextOptionsEmpty(withoutAttestation) || reflect.DeepEqual(handoff.CanonicalContextOptions(supplied), handoff.CanonicalContextOptions(expected)) {
			return expected, nil
		}
	}
	if !reflect.DeepEqual(handoff.CanonicalContextOptions(supplied), handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(persisted))) {
		return handoff.ContextOptions{}, fmt.Errorf("supplied handoff context options do not match the sealed delivery contract")
	}
	return handoff.ContextOptionsFromModel(persisted), nil
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
	result.CodexHookTrustBypassRequired = codexHookTrustBypassRequired(record)
	result.CodexHookTrustBypassAttested = record.ExecutionHandoff.ContextOptions != nil && record.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass
	result.Orca = record.ExecutionHandoff.Orca
	if record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
		result.RecoveryCode = "explicit_reconcile_required"
		if record.ExecutionHandoff.Failure != nil && record.ExecutionHandoff.Failure.Code != "" {
			result.RecoveryCode = record.ExecutionHandoff.Failure.Code
		}
	}
	return result
}

func codexHookTrustBypassRequired(record IssueOpsRecord) bool {
	return record.ExecutionHandoff != nil && record.ExecutionHandoff.Driver == "orca" && record.ExecutionHandoff.Agent == "codex"
}

func ReconcileIssueOpsHandoffTerminal(baseline []string, worktreeID, workerRoot string, rows []port.OrcaTerminal) (port.OrcaTerminal, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTerminal, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.PTYID]; existed || row.PTYID == "" {
			continue
		}
		if row.WorktreeID == strings.TrimSpace(worktreeID) && strings.TrimSpace(row.Handle) != "" && row.Connected && row.Writable && terminalWorktreePathMatches(row, workerRoot) {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTerminal{}, fmt.Errorf("terminal recovery requires exactly one PTY delta; found %d", len(candidates))
	}
	return candidates[0], nil
}

func terminalWorktreePathMatches(terminal port.OrcaTerminal, workerRoot string) bool {
	path := strings.TrimSpace(terminal.WorktreePath)
	return path != "" && filepath.Clean(path) == filepath.Clean(strings.TrimSpace(workerRoot))
}

func ReconcileIssueOpsHandoffTask(baseline []string, marker, displayName string, rows []port.OrcaTask) (port.OrcaTask, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTask, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.ID]; existed || row.ID == "" {
			continue
		}
		if row.Title == marker && row.DisplayName == displayName && validInitialOrcaTaskStatus(row.Status) {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTask{}, fmt.Errorf("task recovery requires exactly one marker candidate; found %d", len(candidates))
	}
	return candidates[0], nil
}

func validInitialOrcaTaskStatus(status string) bool {
	return status == "ready"
}

const (
	orcaTaskTitleMaxLength   = 80
	orcaDisplayNameMaxLength = 160
)

func issueOpsHandoffTaskIdentity(id, epoch string, attempt int) (string, string, error) {
	title := "ah id=" + strings.TrimSpace(id) + " a=" + strconv.Itoa(attempt) + " e=" + strings.TrimSpace(epoch)
	display := "ah " + strings.TrimSpace(id) + " attempt=" + strconv.Itoa(attempt)
	if len(title) == 0 || len(title) > orcaTaskTitleMaxLength || len(display) == 0 || len(display) > orcaDisplayNameMaxLength {
		return "", "", fmt.Errorf("Orca task identity exceeds title/display limits %d/%d", orcaTaskTitleMaxLength, orcaDisplayNameMaxLength)
	}
	return title, display, nil
}

func ReconcileIssueOpsHandoffDispatch(ctx context.Context, taskID, assigneeHandle string, client interface {
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
}) (port.OrcaDispatch, error) {
	taskID = strings.TrimSpace(taskID)
	assigneeHandle = strings.TrimSpace(assigneeHandle)
	if taskID == "" || assigneeHandle == "" {
		return port.OrcaDispatch{}, fmt.Errorf("persisted task id and worker mailbox are required for dispatch recovery")
	}
	dispatch, err := client.ShowDispatch(ctx, taskID)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if dispatch.ID == "" || dispatch.TaskID != taskID || strings.TrimSpace(dispatch.AssigneeHandle) != assigneeHandle || !dispatch.Injected {
		return port.OrcaDispatch{}, fmt.Errorf("dispatch recovery result does not match persisted task, assignee, and injected delivery")
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
