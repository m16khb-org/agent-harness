package issueops

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const (
	IssueOpsHandoffForceAbandonMinimumAge  = 5 * time.Minute
	IssueOpsHandoffForceAbandonReasonBytes = 4096
	IssueOpsHandoffCancellationMinimumAge  = 5 * time.Minute
	forceAbandonedOperationCode            = "force_abandoned_absent_operation"
)

type IssueOpsHandoffRecoverRequest struct {
	ID                 string `json:"id"`
	Action             string `json:"action"`
	Confirm            bool   `json:"confirm,omitempty"`
	Force              bool   `json:"force,omitempty"`
	Reason             string `json:"reason,omitempty"`
	CleanupDisposition string `json:"cleanup_disposition,omitempty"`
	CleanupStep        string `json:"cleanup_step,omitempty"`
}

type IssueOpsHandoffRecoverResult struct {
	OK           bool                             `json:"ok"`
	ID           string                           `json:"id"`
	Action       string                           `json:"action"`
	State        string                           `json:"state"`
	Disposition  string                           `json:"disposition,omitempty"`
	Attempt      int                              `json:"attempt"`
	RecoveryCode string                           `json:"recovery_code,omitempty"`
	NextCommand  string                           `json:"next_command,omitempty"`
	Orca         *IssueOpsOrcaIdentity            `json:"orca,omitempty"`
	Cleanup      *IssueOpsExecutionHandoffCleanup `json:"cleanup,omitempty"`
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
		return cancelIssueOpsHandoff(stateRoot, req.ID, req.Force, req.Reason, issueOpsHandoffNow(clock))
	case "finalize-cancel":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("finalize-cancel requires --confirm")
		}
		return finalizeCancelledIssueOpsHandoff(ctx, stateRoot, req.ID, client, issueOpsHandoffNow(clock))
	case "abandon":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("abandon requires --confirm")
		}
		if !req.Force {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("abandon requires --force")
		}
		return forceAbandonIssueOpsHandoff(ctx, stateRoot, req.ID, req.Reason, client, issueOpsHandoffNow(clock))
	case "retry":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires --confirm")
		}
		return retryIssueOpsHandoff(ctx, stateRoot, req.ID, client, clock)
	case "approve-cleanup":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("approve-cleanup requires --confirm")
		}
		return approveIssueOpsHandoffCleanup(stateRoot, req.ID, req.CleanupDisposition, req.Reason, issueOpsHandoffNow(clock))
	case "record-cleanup":
		if !req.Confirm {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("record-cleanup requires --confirm")
		}
		return recordIssueOpsHandoffCleanup(ctx, stateRoot, req.ID, req.CleanupStep, client, issueOpsHandoffNow(clock))
	default:
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("recovery action must be reconcile, abandon, cancel, finalize-cancel, retry, approve-cleanup, or record-cleanup")
	}
}

func forceAbandonIssueOpsHandoff(ctx context.Context, stateRoot, id, reason string, client any, now string) (IssueOpsHandoffRecoverResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("abandon requires a nonempty --reason")
	}
	if len(reason) > IssueOpsHandoffForceAbandonReasonBytes {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("abandon reason exceeds %d bytes", IssueOpsHandoffForceAbandonReasonBytes)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateRecoveryRequired || record.ExecutionHandoff.PendingOperation == nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("abandon requires recovery_required with a pending operation")
	}
	if record.ExecutionHandoff.PendingOperation.Kind == handoff.OperationRuntimeRefresh || record.ExecutionHandoff.PendingOperation.Kind == handoff.OperationLeaseAttestation {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("runtime_refresh is a read-only identity reconciliation and cannot be force-abandoned")
	}
	startedAt, err := time.Parse(time.RFC3339Nano, record.ExecutionHandoff.PendingOperation.StartedAt)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("parse pending operation age: %w", err)
	}
	nowAt, err := time.Parse(time.RFC3339Nano, now)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("parse abandon time: %w", err)
	}
	if nowAt.Sub(startedAt) < IssueOpsHandoffForceAbandonMinimumAge {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("pending operation must be at least %s old", IssueOpsHandoffForceAbandonMinimumAge)
	}
	validated := cloneHandoffReconcileSnapshot(record)
	if err := requireAbsentPendingOperation(ctx, record, client); err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}

	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("handoff changed during force-abandon inventory")
		}
		current.ExecutionHandoff.PendingOperation = nil
		current.ExecutionHandoff.State = handoff.StateClosed
		current.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
		current.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{
			Code: forceAbandonedOperationCode, Message: strings.TrimSpace(policy.RedactFreeform(reason)), At: now,
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return projectHandoffRecovery(persisted, "abandon", ""), err
}

func requireAbsentPendingOperation(ctx context.Context, record IssueOpsRecord, client any) error {
	pending := record.ExecutionHandoff.PendingOperation
	switch pending.Kind {
	case handoff.OperationWorktreeCreate:
		reader, ok := client.(interface {
			ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
		})
		if !ok {
			return fmt.Errorf("Orca worktree inventory dependency is unavailable")
		}
		rows, err := reader.ListWorktrees(ctx, record.Repo)
		if err != nil {
			return err
		}
		return requireNoExactWorktreeCandidates(record, pending.BaselineWorktreeIDs, rows)
	case handoff.OperationTerminalCreate:
		reader, ok := client.(interface {
			ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
		})
		if !ok || record.ExecutionHandoff.Orca == nil || strings.TrimSpace(record.ExecutionHandoff.Orca.WorktreeID) == "" {
			return fmt.Errorf("Orca terminal inventory dependency and worktree id are required")
		}
		rows, err := reader.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
		if err != nil {
			return err
		}
		return requireNoExactTerminalCandidates(record, pending.BaselinePTYIDs, rows)
	case handoff.OperationTaskCreate:
		reader, ok := client.(interface {
			ListTasks(context.Context) ([]port.OrcaTask, error)
		})
		if !ok {
			return fmt.Errorf("Orca task inventory dependency is unavailable")
		}
		rows, err := reader.ListTasks(ctx)
		if err != nil {
			return err
		}
		return requireNoExactTaskCandidates(record, pending.BaselineTaskIDs, rows)
	case handoff.OperationDispatch:
		reader, ok := client.(interface {
			ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
		})
		if !ok || record.ExecutionHandoff.Orca == nil || strings.TrimSpace(record.ExecutionHandoff.Orca.TaskID) == "" {
			return fmt.Errorf("Orca dispatch inventory dependency and task id are required")
		}
		_, err := reader.ShowDispatch(ctx, record.ExecutionHandoff.Orca.TaskID)
		if err == nil {
			return fmt.Errorf("dispatch still exists")
		}
		var orcaErr *port.OrcaError
		if !errors.As(err, &orcaErr) || strings.TrimSpace(orcaErr.Code) != "not_found" {
			return fmt.Errorf("inspect dispatch absence: %w", err)
		}
		return nil
	case handoff.OperationRuntimeRefresh, handoff.OperationLeaseAttestation:
		return nil
	default:
		return fmt.Errorf("unsupported pending operation %q", pending.Kind)
	}
}

// countExactPostBaselineCandidates is the generic exact-candidate reconcile
// matcher (Task B): bound the inventory, require stable identities (so an
// unidentifiable row is never absence evidence), skip rows already in the
// baseline, require each remaining row's classification fields to be complete,
// and count the rows that exactly match the fence marker. The per-artifact
// stable identity, classification-completeness predicate, and exact marker match
// are supplied by the caller. This replaces the per-artifact worktree/terminal/
// task re-implementations of the same rule (recurring-bug Cluster A/B). Error
// messages are byte-identical to the prior per-artifact code.
func countExactPostBaselineCandidates(kind string, ids, baseline []string, rowCount int, classified, matches func(i int) bool) (int, error) {
	if rowCount > handoff.MaxBaselineIDs {
		return 0, fmt.Errorf("%s inventory exceeds %d entries", kind, handoff.MaxBaselineIDs)
	}
	if err := requireStableInventoryIdentities(kind, ids); err != nil {
		return 0, err
	}
	before := baselineIdentitySet(baseline)
	candidates := 0
	for i := range rowCount {
		if _, existed := before[strings.TrimSpace(ids[i])]; existed {
			continue
		}
		if !classified(i) {
			return 0, fmt.Errorf("%s inventory row is missing classification fields", kind)
		}
		if matches(i) {
			candidates++
		}
	}
	return candidates, nil
}

func requireNoExactWorktreeCandidates(record IssueOpsRecord, baseline []string, rows []port.OrcaWorktree) error {
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		return fmt.Errorf("worktree recovery identity is unavailable")
	}
	marker := issueOpsHandoffMarker(record.ID, h.OwnershipEpoch, h.Attempt)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	candidates, err := countExactPostBaselineCandidates("worktree", ids, baseline, len(rows),
		func(i int) bool {
			r := rows[i]
			return strings.TrimSpace(r.RepoID) != "" && strings.TrimSpace(r.BaseRef) != "" && strings.TrimSpace(r.Path) != "" && strings.TrimSpace(r.Comment) != ""
		},
		func(i int) bool {
			r := rows[i]
			return r.RepoID == h.Orca.RepoID && r.BaseRef == h.Orca.BaseRef && filepath.Clean(strings.TrimSpace(r.Path)) == filepath.Clean(h.WorkerRoot) && r.Comment == marker
		})
	if err != nil {
		return err
	}
	return requireZeroExactCandidates("worktree", candidates)
}

func requireNoExactTerminalCandidates(record IssueOpsRecord, baseline []string, rows []port.OrcaTerminal) error {
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		return fmt.Errorf("terminal recovery identity is unavailable")
	}
	marker := issueOpsHandoffMarker(record.ID, h.OwnershipEpoch, h.Attempt)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].PTYID
	}
	candidates, err := countExactPostBaselineCandidates("terminal", ids, baseline, len(rows),
		func(i int) bool {
			r := rows[i]
			return strings.TrimSpace(r.WorktreeID) != "" && strings.TrimSpace(r.Title) != ""
		},
		func(i int) bool {
			r := rows[i]
			return r.WorktreeID == h.Orca.WorktreeID && r.Title == marker
		})
	if err != nil {
		return err
	}
	return requireZeroExactCandidates("terminal", candidates)
}

func requireNoExactTaskCandidates(record IssueOpsRecord, baseline []string, rows []port.OrcaTask) error {
	if record.ExecutionHandoff == nil {
		return fmt.Errorf("task recovery identity is unavailable")
	}
	title, display, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		return err
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	candidates, err := countExactPostBaselineCandidates("task", ids, baseline, len(rows),
		func(i int) bool {
			r := rows[i]
			return strings.TrimSpace(r.Title) != "" && strings.TrimSpace(r.DisplayName) != "" && strings.TrimSpace(r.Status) != ""
		},
		func(i int) bool {
			r := rows[i]
			return r.Title == title && r.DisplayName == display
		})
	if err != nil {
		return err
	}
	return requireZeroExactCandidates("task", candidates)
}

func requireStableInventoryIdentities(kind string, values []string) error {
	canonical, err := handoff.CanonicalBaselineIDs(kind, values)
	if err != nil {
		return fmt.Errorf("%s inventory is not bounded: %w", kind, err)
	}
	if len(canonical) != len(values) {
		return fmt.Errorf("%s inventory contains missing or duplicate stable identity", kind)
	}
	return nil
}

func baselineIdentitySet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.TrimSpace(value)] = struct{}{}
	}
	return set
}

func requireZeroExactCandidates(kind string, candidates int) error {
	if candidates != 0 {
		return fmt.Errorf("%s inventory contains %d exact post-baseline candidate", kind, candidates)
	}
	return nil
}

func cancelIssueOpsHandoff(stateRoot, id string, force bool, reason, now string) (IssueOpsHandoffRecoverResult, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
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
		if record.ExecutionHandoff.Cancellation != nil {
			persisted = record
			return nil
		}
		if record.ExecutionHandoff.State == handoff.StateClaimed || record.ExecutionHandoff.State == handoff.StateSubmitted {
			if !force || strings.TrimSpace(reason) == "" {
				return fmt.Errorf("claimed or submitted handoff cancel requires --force with a nonempty --reason")
			}
		}
		if !handoffHasExternalMutation(record.ExecutionHandoff) {
			record.ExecutionHandoff.State = handoff.StateClosed
			record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
			record.ExecutionHandoff.UpdatedAt = now
			record.UpdatedAt = now
			persisted, err = writeIssueOps(stateRoot, record)
			return err
		}
		reason = strings.TrimSpace(policy.RedactFreeform(strings.TrimSpace(reason)))
		if reason == "" {
			reason = "coordinator requested cancellation"
		}
		record.ExecutionHandoff.State = handoff.StateRecoveryRequired
		record.ExecutionHandoff.ClosedDisposition = ""
		record.ExecutionHandoff.Cancellation = &model.IssueOpsExecutionHandoffCancellation{RequestedAt: now, Reason: reason}
		record.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{Code: "cancellation_requested", Message: reason, At: now}
		record.ExecutionHandoff.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return projectHandoffRecovery(persisted, "cancel", ""), err
}

func handoffHasExternalMutation(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil {
		return false
	}
	if h.PendingOperation != nil || h.CleanupOnly != nil || h.WorkerSession != nil || h.Result != nil {
		return true
	}
	if h.Orca == nil {
		return false
	}
	return strings.TrimSpace(h.Orca.WorktreeID) != "" || strings.TrimSpace(h.Orca.WorktreeInstanceID) != "" || strings.TrimSpace(h.Orca.WorktreePath) != "" || strings.TrimSpace(h.Orca.WorkerPTYID) != "" || strings.TrimSpace(h.Orca.WorkerTerminalHandle) != "" || strings.TrimSpace(h.Orca.WorkerMailboxHandle) != "" || strings.TrimSpace(h.Orca.TaskID) != "" || strings.TrimSpace(h.Orca.DispatchID) != ""
}

func finalizeCancelledIssueOpsHandoff(ctx context.Context, stateRoot, id string, client any, now string) (IssueOpsHandoffRecoverResult, error) {
	validated, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	h := validated.ExecutionHandoff
	if h == nil || h.State != handoff.StateRecoveryRequired || h.Cancellation == nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("finalize-cancel requires a cancellation_requested tombstone")
	}
	if h.PendingOperation != nil {
		if err := requireAbsentPendingOperation(ctx, validated, client); err != nil {
			return IssueOpsHandoffRecoverResult{}, err
		}
	}
	if err := requireCancellationQuiescence(ctx, validated, client, now); err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("cancellation tombstone changed during quiescence verification")
		}
		reason := current.ExecutionHandoff.Cancellation.Reason
		current.ExecutionHandoff.Cancellation = nil
		current.ExecutionHandoff.PendingOperation = nil
		current.ExecutionHandoff.State = handoff.StateClosed
		current.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
		current.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{Code: "cancellation_finalized", Message: reason, At: now}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return projectHandoffRecovery(persisted, "finalize-cancel", ""), err
}

func requireCancellationQuiescence(ctx context.Context, record IssueOpsRecord, client any, now string) error {
	h := record.ExecutionHandoff
	if h == nil {
		return nil
	}
	if h.CleanupOnly != nil {
		reader, ok := client.(interface {
			ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
		})
		if !ok {
			return fmt.Errorf("cleanup-only worktree absence inventory is unavailable")
		}
		rows, err := reader.ListWorktrees(ctx, record.Repo)
		if err != nil {
			return err
		}
		ids := make([]string, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		if err := requireStableInventoryIdentities("worktree", ids); err != nil {
			return err
		}
		for _, row := range rows {
			if row.ID == h.CleanupOnly.ID {
				return fmt.Errorf("cleanup-only worktree still exists")
			}
		}
		return nil
	}
	if h.Orca == nil {
		return nil
	}
	identity := h.Orca
	dispatchClient, ok := client.(IssueOpsOrcaDispatchClient)
	if !ok {
		return fmt.Errorf("complete cancellation terminal and dispatched-task inventory is unavailable")
	}
	if err := attestHandoffSoleWriter(ctx, record, dispatchClient, ""); err != nil {
		return fmt.Errorf("cancellation quiescence found a possible writer: %w", err)
	}
	if strings.TrimSpace(identity.WorkerPTYID) != "" || strings.TrimSpace(identity.WorkerTerminalHandle) != "" {
		reader, ok := client.(interface {
			ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
		})
		if !ok || strings.TrimSpace(identity.WorktreeID) == "" {
			return fmt.Errorf("terminal quiescence inventory is unavailable")
		}
		rows, err := reader.ListTerminals(ctx, identity.WorktreeID)
		if err != nil {
			return err
		}
		ids := make([]string, len(rows))
		handles := make([]string, len(rows))
		for i := range rows {
			ids[i] = rows[i].PTYID
			handles[i] = rows[i].Handle
		}
		if err := requireStableInventoryIdentities("terminal", ids); err != nil {
			return err
		}
		if err := requireStableInventoryIdentities("terminal", handles); err != nil {
			return err
		}
		for _, row := range rows {
			ptyMatch := strings.TrimSpace(identity.WorkerPTYID) != "" && row.PTYID == identity.WorkerPTYID
			handleMatch := strings.TrimSpace(identity.WorkerTerminalHandle) != "" && row.Handle == identity.WorkerTerminalHandle
			if ptyMatch != handleMatch {
				return fmt.Errorf("terminal quiescence identity is inconsistent")
			}
			if ptyMatch && handleMatch && row.Connected {
				return fmt.Errorf("exact worker terminal is still connected")
			}
		}
		stableObserved := strings.TrimSpace(identity.WorkerTabID) != "" || strings.TrimSpace(identity.WorkerLeafID) != ""
		marker := issueOpsHandoffMarker(record.ID, h.OwnershipEpoch, h.Attempt)
		reissued := make([]port.OrcaTerminal, 0, 1)
		for _, row := range rows {
			matches := row.StableTabTitle == marker
			if stableObserved {
				matches = row.TabID == identity.WorkerTabID && row.LeafID == identity.WorkerLeafID
			}
			if !matches || row.PTYID == identity.WorkerPTYID && row.Handle == identity.WorkerTerminalHandle {
				continue
			}
			reissued = append(reissued, row)
		}
		if len(reissued) > 1 {
			return fmt.Errorf("runtime-reissued worker terminal identity is ambiguous")
		}
		if len(reissued) == 1 {
			row := reissued[0]
			if strings.TrimSpace(row.RuntimeID) == "" || row.RuntimeID == identity.RuntimeID || row.WorktreeID != identity.WorktreeID || !terminalWorktreePathMatches(row, h.WorkerRoot) || (row.TabID == "") != (row.LeafID == "") {
				return fmt.Errorf("runtime-reissued worker terminal identity is inconsistent")
			}
			if row.Connected {
				return fmt.Errorf("runtime-reissued worker terminal is still connected")
			}
		}
	}
	taskID := strings.TrimSpace(identity.TaskID)
	dispatchID := strings.TrimSpace(identity.DispatchID)
	if taskID == "" && dispatchID != "" {
		return fmt.Errorf("task and dispatch quiescence identity is incomplete")
	}
	if taskID != "" && dispatchID == "" {
		if h.PendingOperation == nil || h.PendingOperation.Kind != handoff.OperationDispatch {
			return fmt.Errorf("task and dispatch quiescence identity is incomplete")
		}
		if err := requireTaskNotReady(ctx, client, taskID); err != nil {
			return err
		}
	}
	if taskID != "" && dispatchID != "" {
		reader, ok := client.(interface {
			ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
		})
		if !ok {
			return fmt.Errorf("task and dispatch quiescence identity is incomplete")
		}
		dispatch, err := reader.ShowDispatch(ctx, taskID)
		if err != nil {
			var orcaErr *port.OrcaError
			if !errors.As(err, &orcaErr) || orcaErr.Code != "not_found" {
				return err
			}
			if err := requireTaskNotReady(ctx, client, taskID); err != nil {
				return err
			}
		} else if dispatch.ID != dispatchID || dispatch.TaskID != taskID || strings.TrimSpace(dispatch.AssigneeHandle) != strings.TrimSpace(identity.WorkerMailboxHandle) || !terminalDispatchStatus(dispatch.Status) {
			return fmt.Errorf("exact task and dispatch are not terminal")
		}
	}
	if h.WorkerSession != nil || strings.TrimSpace(h.LastHeartbeatAt) != "" {
		last := strings.TrimSpace(h.LastHeartbeatAt)
		if last == "" {
			last = strings.TrimSpace(h.ClaimedAt)
		}
		lastAt, err := time.Parse(time.RFC3339Nano, last)
		if err != nil {
			return fmt.Errorf("worker liveness timestamp is unavailable")
		}
		nowAt, err := time.Parse(time.RFC3339Nano, now)
		if err != nil || nowAt.Sub(lastAt) < IssueOpsHandoffCancellationMinimumAge {
			return fmt.Errorf("worker heartbeat is not stale enough to finalize cancellation")
		}
	}
	return nil
}

func requireTaskNotReady(ctx context.Context, client any, taskID string) error {
	reader, ok := client.(interface {
		ListTasks(context.Context) ([]port.OrcaTask, error)
	})
	if !ok {
		return fmt.Errorf("task readiness inventory is unavailable")
	}
	rows, err := reader.ListTasks(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	if err := requireStableInventoryIdentities("task", ids); err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.ID) == taskID {
			return fmt.Errorf("exact worker task is still ready")
		}
	}
	return nil
}

func terminalDispatchStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "failed", "completed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func approveIssueOpsHandoffCleanup(stateRoot, id, disposition, reason, now string) (IssueOpsHandoffRecoverResult, error) {
	disposition = strings.ToLower(strings.TrimSpace(disposition))
	reason = strings.TrimSpace(policy.RedactFreeform(strings.TrimSpace(reason)))
	if disposition != "retry" && disposition != "remove" {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("cleanup disposition must be retry or remove")
	}
	if reason == "" || len(reason) > IssueOpsHandoffForceAbandonReasonBytes {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("cleanup approval requires a nonempty bounded reason")
	}
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		h := record.ExecutionHandoff
		if h == nil || h.State != handoff.StateClosed || h.ClosedDisposition != handoff.DispositionWorkerFailed && h.ClosedDisposition != handoff.DispositionCancelled || h.Orca == nil {
			return fmt.Errorf("cleanup approval requires a closed worker_failed or cancelled handoff")
		}
		if h.Cleanup != nil {
			if h.Cleanup.Disposition != disposition || h.Cleanup.Reason != reason {
				return fmt.Errorf("cleanup approval already exists with a different disposition or reason")
			}
			persisted = record
			return nil
		}
		h.Cleanup = &model.IssueOpsExecutionHandoffCleanup{Disposition: disposition, Reason: reason, ApprovedAt: now}
		h.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return projectHandoffRecovery(persisted, "approve-cleanup", ""), err
}

func recordIssueOpsHandoffCleanup(ctx context.Context, stateRoot, id, step string, client any, now string) (IssueOpsHandoffRecoverResult, error) {
	step = strings.ToLower(strings.TrimSpace(step))
	validated, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	h := validated.ExecutionHandoff
	if h == nil || h.Cleanup == nil || h.Orca == nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("cleanup receipt requires prior cleanup approval and exact Orca identity")
	}
	for _, receipt := range h.Cleanup.Receipts {
		if receipt.Step == step {
			return projectHandoffRecovery(validated, "record-cleanup", ""), nil
		}
	}
	expected := []string{"task_terminal", "terminal_quiescent"}
	if h.Cleanup.Disposition == "remove" {
		expected = append(expected, "worktree_removed")
	}
	if len(h.Cleanup.Receipts) >= len(expected) || step != expected[len(h.Cleanup.Receipts)] {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("cleanup receipt %q is out of order for disposition %s", step, h.Cleanup.Disposition)
	}
	receipt, err := verifyIssueOpsCleanupStep(ctx, validated, step, client, now)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("handoff changed during cleanup verification")
		}
		current.ExecutionHandoff.Cleanup.Receipts = append(current.ExecutionHandoff.Cleanup.Receipts, receipt)
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return projectHandoffRecovery(persisted, "record-cleanup", ""), err
}

func verifyIssueOpsCleanupStep(ctx context.Context, record IssueOpsRecord, step string, client any, now string) (model.IssueOpsExecutionHandoffCleanupReceipt, error) {
	h := record.ExecutionHandoff
	identity := h.Orca
	receipt := model.IssueOpsExecutionHandoffCleanupReceipt{Step: step, RecordedAt: now}
	switch step {
	case "task_terminal":
		taskID := strings.TrimSpace(identity.TaskID)
		dispatchID := strings.TrimSpace(identity.DispatchID)
		if taskID == "" && dispatchID == "" {
			break
		}
		reader, ok := client.(interface {
			ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
		})
		if !ok || taskID == "" || dispatchID == "" {
			return receipt, fmt.Errorf("task cleanup verification dependency or identity is unavailable")
		}
		dispatch, err := reader.ShowDispatch(ctx, taskID)
		if err != nil {
			return receipt, err
		}
		if dispatch.ID != dispatchID || dispatch.TaskID != taskID || dispatch.AssigneeHandle != identity.WorkerMailboxHandle || !terminalDispatchStatus(dispatch.Status) {
			return receipt, fmt.Errorf("exact task and dispatch are not terminal")
		}
		receipt.TaskID, receipt.DispatchID = taskID, dispatchID
	case "terminal_quiescent":
		reader, ok := client.(interface {
			ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
		})
		if !ok || identity.WorktreeID == "" || identity.WorkerPTYID == "" || identity.WorkerTerminalHandle == "" {
			return receipt, fmt.Errorf("terminal cleanup verification dependency or identity is unavailable")
		}
		rows, err := reader.ListTerminals(ctx, identity.WorktreeID)
		if err != nil {
			return receipt, err
		}
		ptys, handles := make([]string, len(rows)), make([]string, len(rows))
		exactHandles := map[string]struct{}{identity.WorkerTerminalHandle: {}}
		if identity.WorkerMailboxHandle != "" {
			exactHandles[identity.WorkerMailboxHandle] = struct{}{}
		}
		for i, row := range rows {
			ptys[i], handles[i] = row.PTYID, row.Handle
			if row.WorktreeID != identity.WorktreeID || !terminalWorktreePathMatches(row, h.WorkerRoot) {
				return receipt, fmt.Errorf("terminal cleanup inventory contains an inexact worktree row")
			}
			exactHandles[row.Handle] = struct{}{}
			if row.Connected || row.Writable {
				return receipt, fmt.Errorf("exact-worktree terminal %s is still connected or writable", row.Handle)
			}
			ptyMatch, handleMatch := row.PTYID == identity.WorkerPTYID, row.Handle == identity.WorkerTerminalHandle
			if ptyMatch != handleMatch {
				return receipt, fmt.Errorf("terminal cleanup identity is ambiguous")
			}
		}
		if err := requireStableInventoryIdentities("terminal", ptys); err != nil {
			return receipt, err
		}
		if err := requireStableInventoryIdentities("terminal", handles); err != nil {
			return receipt, err
		}
		dispatchReader, ok := client.(interface {
			ListDispatchedTasks(context.Context) ([]port.OrcaTask, error)
			ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
		})
		if !ok {
			return receipt, fmt.Errorf("dispatched-task cleanup verification dependency is unavailable")
		}
		tasks, err := dispatchReader.ListDispatchedTasks(ctx)
		if err != nil {
			return receipt, err
		}
		taskIDs := make([]string, len(tasks))
		for i, task := range tasks {
			taskIDs[i] = task.ID
			dispatch, showErr := dispatchReader.ShowDispatch(ctx, task.ID)
			if showErr != nil || dispatch.ID == "" || dispatch.TaskID != task.ID || dispatch.Status != "dispatched" || dispatch.AssigneeHandle == "" {
				return receipt, fmt.Errorf("dispatched-task cleanup inventory is ambiguous")
			}
			if _, assignedHere := exactHandles[dispatch.AssigneeHandle]; assignedHere {
				return receipt, fmt.Errorf("dispatched task %s remains assigned to the exact worktree", task.ID)
			}
		}
		if err := requireStableInventoryIdentities("task", taskIDs); err != nil {
			return receipt, err
		}
		receipt.TerminalHandle, receipt.PTYID, receipt.WorktreeID = identity.WorkerTerminalHandle, identity.WorkerPTYID, identity.WorktreeID
	case "worktree_removed":
		reader, ok := client.(interface {
			ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
		})
		if !ok || identity.WorktreeID == "" || identity.WorktreeInstanceID == "" {
			return receipt, fmt.Errorf("worktree cleanup verification dependency or identity is unavailable")
		}
		rows, err := reader.ListWorktrees(ctx, record.Repo)
		if err != nil {
			return receipt, err
		}
		ids := make([]string, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
			branch := strings.TrimPrefix(strings.TrimSpace(row.Branch), "refs/heads/")
			path := strings.TrimSpace(row.Path)
			if strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.InstanceID) == "" || strings.TrimSpace(row.RepoID) == "" || row.RepoID != identity.RepoID || path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || branch == "" {
				return receipt, fmt.Errorf("worktree cleanup inventory row lacks complete canonical classification identity")
			}
			idMatch, instanceMatch := row.ID == identity.WorktreeID, row.InstanceID == identity.WorktreeInstanceID
			if idMatch != instanceMatch {
				return receipt, fmt.Errorf("worktree cleanup identity is ambiguous")
			}
			if idMatch && instanceMatch {
				return receipt, fmt.Errorf("exact worker worktree still exists")
			}
			pathMatch := strings.TrimSpace(row.Path) != "" && filepath.Clean(row.Path) == filepath.Clean(h.WorkerRoot)
			branchMatch := branch == record.Branch
			if pathMatch || branchMatch {
				return receipt, fmt.Errorf("canonical worker worktree was reissued with a different external identity")
			}
		}
		if err := requireStableInventoryIdentities("worktree", ids); err != nil {
			return receipt, err
		}
		receipt.WorktreeID, receipt.WorktreeInstanceID = identity.WorktreeID, identity.WorktreeInstanceID
	default:
		return receipt, fmt.Errorf("unknown cleanup step %q", step)
	}
	return receipt, nil
}

func retryIssueOpsHandoff(ctx context.Context, stateRoot, id string, client any, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffRecoverResult, error) {
	validated, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	old := validated.ExecutionHandoff
	if old == nil || old.State != handoff.StateClosed {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires a safely closed prior attempt")
	}
	if old.ClosedDisposition != handoff.DispositionWorkerFailed && old.ClosedDisposition != handoff.DispositionCancelled {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires a closed worker_failed or cancelled prior attempt")
	}
	if old.PendingOperation != nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires every ambiguous operation to be reconciled")
	}
	if old.CleanupOnly != nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry is forbidden for a cleanup-only invalid Orca artifact; remove exact worktree id:%s and start a fresh IssueOps cycle", old.CleanupOnly.ID)
	}
	if old.Failure != nil && old.Failure.Code == forceAbandonedOperationCode {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry is forbidden after a force-abandoned ambiguous operation; start a fresh IssueOps cycle")
	}
	if old.Cleanup == nil || old.Cleanup.Disposition != "retry" || len(old.Cleanup.Receipts) != 2 || old.Cleanup.Receipts[0].Step != "task_terminal" || old.Cleanup.Receipts[1].Step != "terminal_quiescent" {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires durable retry cleanup approval with task and terminal quiescence receipts")
	}
	attemptBaseHead, err := retryIssueOpsHandoffCheckpoint(validated)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	now := issueOpsHandoffNow(clock)
	if err := requireRetryHandoffQuiescence(ctx, validated, client, now); err != nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("retry requires exact external quiescence: %w", err)
	}
	epoch, err := issueOpsHandoffEpoch(clock)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(record, validated) {
			return fmt.Errorf("handoff changed after retry checkpoint validation")
		}
		old := record.ExecutionHandoff
		priorAttempt, snapshotErr := handoff.SnapshotPriorAttempt(old)
		if snapshotErr != nil {
			return snapshotErr
		}
		priorAttempts := append([]model.IssueOpsExecutionHandoffPriorAttempt(nil), old.PriorAttempts...)
		priorAttempts = append(priorAttempts, priorAttempt)
		var worktreeIdentity *model.IssueOpsOrcaIdentity
		if old.Orca != nil && old.Orca.WorktreeID != "" {
			worktreeIdentity = &model.IssueOpsOrcaIdentity{
				RuntimeID: old.Orca.RuntimeID, RepoID: old.Orca.RepoID, BaseRef: old.Orca.BaseRef, ProviderIssueLinkStatus: old.Orca.ProviderIssueLinkStatus,
				WorktreeID: old.Orca.WorktreeID, WorktreeInstanceID: old.Orca.WorktreeInstanceID, WorktreePath: old.Orca.WorktreePath,
			}
		}
		var contextOptions *model.IssueOpsExecutionHandoffContextOptions
		if old.ContextOptions != nil {
			cloned := *old.ContextOptions
			cloned.CriteriaIDs = append([]string(nil), old.ContextOptions.CriteriaIDs...)
			cloned.RequiredDocs = append([]string(nil), old.ContextOptions.RequiredDocs...)
			cloned.RequiredSkills = append([]string(nil), old.ContextOptions.RequiredSkills...)
			cloned.VerificationCommands = append([]string(nil), old.ContextOptions.VerificationCommands...)
			cloned.StopConditions = append([]string(nil), old.ContextOptions.StopConditions...)
			cloned.AllowCodexHookTrustBypass = false
			contextOptions = &cloned
		}
		record.ExecutionHandoff = &model.IssueOpsExecutionHandoff{
			ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateCoordinatorPreparing,
			Attempt: old.Attempt + 1, OwnershipEpoch: epoch, Driver: "orca", Agent: old.Agent,
			AttemptBaseHead: attemptBaseHead,
			CoordinatorRoot: old.CoordinatorRoot, WorkerRoot: old.WorkerRoot, Orca: worktreeIdentity, ContextOptions: contextOptions,
			PriorAttempts: priorAttempts,
			PreparedAt:    now, ProvisionedAt: now, UpdatedAt: now,
		}
		record.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, record)
		return readErr
	})
	return projectHandoffRecovery(persisted, "retry", "agent-harness issueops handoff start --id "+id+" --confirm"), err
}

func requireRetryHandoffQuiescence(ctx context.Context, record IssueOpsRecord, client any, now string) error {
	clone := cloneHandoffReconcileSnapshot(record)
	clone.ExecutionHandoff.WorkerSession = nil
	clone.ExecutionHandoff.LastHeartbeatAt = ""
	clone.ExecutionHandoff.ClaimedAt = ""
	if err := requireCancellationQuiescence(ctx, clone, client, now); err != nil {
		return err
	}
	dispatchClient, ok := client.(IssueOpsOrcaDispatchClient)
	if !ok {
		return fmt.Errorf("complete Orca terminal and dispatched-task inventory is unavailable")
	}
	return attestHandoffSoleWriter(ctx, record, dispatchClient, "")
}

func reconcileIssueOpsHandoff(ctx context.Context, stateRoot, id string, client any, now string) (IssueOpsHandoffRecoverResult, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsHandoffRecoverResult{}, err
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateRecoveryRequired || record.ExecutionHandoff.PendingOperation == nil {
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("reconcile requires recovery_required with a pending operation")
	}
	validated := cloneHandoffReconcileSnapshot(record)
	pending := *record.ExecutionHandoff.PendingOperation
	identity := &model.IssueOpsOrcaIdentity{}
	if record.ExecutionHandoff.Orca != nil {
		*identity = *record.ExecutionHandoff.Orca
		identity.TerminalBaselinePTYIDs = append([]string(nil), record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs...)
	}
	desiredWorktreePath := record.WorktreePath
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
		candidate, matchErr := ReconcileIssueOpsHandoffWorktree(pending, record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		if validateErr := validateCreatedHandoffWorktree(record, record.ExecutionHandoff.WorkerRoot, identity.RepoID, identity.BaseRef, candidate); validateErr != nil {
			if !worktreeCleanupCandidateExact(record, identity.RepoID, handoffFence(record), pending.BaselineWorktreeIDs, candidate) {
				return IssueOpsHandoffRecoverResult{}, validateErr
			}
			persisted, persistErr := persistReconciledCleanupOnlyWorktree(stateRoot, validated, candidate, validateErr.Error(), now)
			if persistErr != nil {
				return IssueOpsHandoffRecoverResult{}, persistErr
			}
			return projectHandoffRecovery(persisted, "reconcile", "cancel this handoff, then remove exact Orca worktree id:"+candidate.ID+" and start a fresh IssueOps cycle"), validateErr
		}
		identity.WorktreeID, identity.WorktreeInstanceID, identity.WorktreePath = candidate.ID, candidate.InstanceID, candidate.Path
		identity.ProviderIssueLinkStatus = providerIssueLinkStatus(record, candidate)
		desiredWorktreePath = candidate.Path
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
		candidate, matchErr := ReconcileIssueOpsHandoffTerminal(pending.BaselinePTYIDs, identity.WorktreeID, record.ExecutionHandoff.WorkerRoot, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.TerminalBaselinePTYIDs = append([]string(nil), pending.BaselinePTYIDs...)
		if candidate.RuntimeID != "" {
			identity.RuntimeID = candidate.RuntimeID
		}
		identity.WorkerPTYID, identity.WorkerTerminalHandle = candidate.PTYID, candidate.Handle
		identity.WorkerTabID, identity.WorkerLeafID = candidate.TabID, candidate.LeafID
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
		title, displayName, identityErr := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
		if identityErr != nil {
			return IssueOpsHandoffRecoverResult{}, identityErr
		}
		candidate, matchErr := ReconcileIssueOpsHandoffTask(pending.BaselineTaskIDs, title, displayName, rows)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.TaskID = candidate.ID
	case handoff.OperationDispatch:
		reader, ok := client.(interface {
			ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error)
		})
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca dispatch recovery dependency is unavailable")
		}
		candidate, matchErr := ReconcileIssueOpsHandoffDispatch(ctx, identity.TaskID, pending.ExpectedAssigneeHandle, pending.DeliveryMode, reader, record.ExecutionHandoff.CoordinatorMailboxHandle)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.DispatchID, identity.WorkerMailboxHandle = candidate.ID, candidate.AssigneeHandle
		identity.WorkerTerminalHandle = candidate.AssigneeHandle
		newState = handoff.StateDispatched
		next = "agent-harness issueops handoff claim --id " + id
	case handoff.OperationRuntimeRefresh:
		reader, ok := client.(IssueOpsOrcaDispatchClient)
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca runtime restart recovery dependency is unavailable")
		}
		worktree, terminal, matchErr := reconcileRuntimeReissuedHandoffIdentity(ctx, record, reader)
		if matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
		identity.RuntimeID = worktree.RuntimeID
		identity.WorktreeInstanceID = worktree.InstanceID
		identity.ProviderIssueLinkStatus = providerIssueLinkStatus(record, worktree)
		identity.WorkerPTYID = terminal.PTYID
		identity.WorkerTerminalHandle = terminal.Handle
		identity.WorkerTabID = terminal.TabID
		identity.WorkerLeafID = terminal.LeafID
	case handoff.OperationLeaseAttestation:
		reader, ok := client.(IssueOpsOrcaDispatchClient)
		if !ok {
			return IssueOpsHandoffRecoverResult{}, fmt.Errorf("Orca sole writer recovery dependency is unavailable")
		}
		allowedHandle := ""
		if identity.WorkerTerminalHandle != "" {
			allowedHandle = identity.WorkerTerminalHandle
		}
		if matchErr := attestHandoffSoleWriter(ctx, record, reader, allowedHandle); matchErr != nil {
			return IssueOpsHandoffRecoverResult{}, matchErr
		}
	default:
		return IssueOpsHandoffRecoverResult{}, fmt.Errorf("unsupported pending operation %q", pending.Kind)
	}

	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("stale reconciliation result")
		}
		if pending.Kind == handoff.OperationRuntimeRefresh || pending.Kind == handoff.OperationLeaseAttestation {
			if err := validateHandoffContextSource(current); err != nil {
				return err
			}
			if err := validateHandoffCleanExactCheckpoint(current); err != nil {
				return err
			}
		}
		current.ExecutionHandoff.Orca = identity
		current.ExecutionHandoff.PendingOperation = nil
		current.ExecutionHandoff.State = newState
		current.ExecutionHandoff.Failure = nil
		if pending.Kind == handoff.OperationDispatch {
			current.ExecutionHandoff.DeliveryMode = pending.DeliveryMode
			current.ExecutionHandoff.DispatchedAt = now
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.WorktreePath = desiredWorktreePath
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return projectHandoffRecovery(persisted, "reconcile", next), err
}

func cloneHandoffReconcileSnapshot(record IssueOpsRecord) IssueOpsRecord {
	if record.ExecutionHandoff == nil {
		return record
	}
	h := *record.ExecutionHandoff
	record.ExecutionHandoff = &h
	if h.Orca != nil {
		orca := *h.Orca
		orca.TerminalBaselinePTYIDs = append([]string(nil), h.Orca.TerminalBaselinePTYIDs...)
		h.Orca = &orca
	}
	if h.PendingOperation != nil {
		pending := *h.PendingOperation
		pending.BaselineWorktreeIDs = append([]string(nil), h.PendingOperation.BaselineWorktreeIDs...)
		pending.BaselineTaskIDs = append([]string(nil), h.PendingOperation.BaselineTaskIDs...)
		pending.BaselinePTYIDs = append([]string(nil), h.PendingOperation.BaselinePTYIDs...)
		h.PendingOperation = &pending
	}
	if h.CleanupOnly != nil {
		cleanup := *h.CleanupOnly
		h.CleanupOnly = &cleanup
	}
	return record
}

func persistReconciledCleanupOnlyWorktree(stateRoot string, validated IssueOpsRecord, candidate port.OrcaWorktree, reason, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, validated.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, validated.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("stale worktree cleanup-only reconciliation result")
		}
		current, err = handoff.MarkCleanupOnlyWorktree(current, handoffFence(current), model.IssueOpsOrcaCleanupArtifact{
			Kind: "worktree", ID: candidate.ID, InstanceID: candidate.InstanceID, Path: candidate.Path, Reason: reason,
		}, model.IssueOpsExecutionHandoffFailure{Code: "worktree_cleanup_only", Message: reason, At: now})
		if err != nil {
			return err
		}
		current.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, current)
		return err
	})
	return persisted, err
}

func retryIssueOpsHandoffCheckpoint(record IssueOpsRecord) (string, error) {
	if record.ExecutionHandoff == nil {
		return "", fmt.Errorf("execution handoff is required")
	}
	workerRoot := pathutil.CleanAbsPath(record.ExecutionHandoff.WorkerRoot)
	if workerRoot == "" || workerRoot != pathutil.CleanAbsPath(record.WorktreePath) {
		return "", fmt.Errorf("retry requires the exact persisted worker worktree")
	}
	branch := strings.TrimSpace(preflight.GitOut(workerRoot, "branch", "--show-current"))
	if branch == "" || branch != strings.TrimSpace(record.Branch) {
		return "", fmt.Errorf("retry requires the clean exact branch and HEAD checkpoint")
	}
	code, head, _ := preflight.GitCmd(workerRoot, "rev-parse", "--verify", "HEAD^{commit}")
	head = strings.TrimSpace(head)
	if code != 0 || head == "" {
		return "", fmt.Errorf("retry requires a readable current HEAD checkpoint")
	}
	code, status, _ := preflight.GitCmd(workerRoot, "status", "--porcelain=v1")
	if code != 0 {
		return "", fmt.Errorf("retry requires a readable worker worktree status")
	}
	if strings.TrimSpace(status) != "" {
		return "", fmt.Errorf("retry requires a clean worker worktree checkpoint")
	}
	return head, nil
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
	result.Cleanup = record.ExecutionHandoff.Cleanup
	if record.ExecutionHandoff.Failure != nil {
		result.RecoveryCode = record.ExecutionHandoff.Failure.Code
	}
	return result
}
