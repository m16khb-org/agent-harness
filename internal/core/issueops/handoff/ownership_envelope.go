package handoff

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"agent-harness/internal/core/issueops/model"
)

func validateExecutionWorkspace(record model.IssueOpsRecord) error {
	w := record.ExecutionWorkspace
	if w == nil {
		return nil
	}
	if w.State != "provisioning" && w.State != "ready" && w.State != StateRecoveryRequired {
		return fmt.Errorf("unknown execution workspace state")
	}
	if w.Driver != "orca" || !canonicalNonSpace(w.WorkspaceEpoch) || !filepath.IsAbs(w.CoordinatorRoot) || !filepath.IsAbs(w.WorkerRoot) || filepath.Clean(w.CoordinatorRoot) != filepath.Clean(record.Repo) || !validWorkerSession(w.PreparationSession) || !fullCommitPattern.MatchString(w.BaseHead) {
		return fmt.Errorf("execution workspace identity is incomplete")
	}
	if w.Orca != nil && (!canonicalOrcaIdentity(w.Orca) || w.Orca.WorkerTerminalHandle != "" || w.Orca.WorkerMailboxHandle != "" || w.Orca.TaskID != "" || w.Orca.DispatchID != "") {
		return fmt.Errorf("execution workspace Orca identity contains owner authority")
	}
	return nil
}

func validateOwnershipEnvelope(record model.IssueOpsRecord) error {
	h := record.ExecutionHandoff
	if h == nil || record.ExecutionWorkspace == nil {
		return fmt.Errorf("ownership handoff requires execution workspace")
	}
	if !canonicalNonSpace(h.WorkspaceEpoch) || !sha256Pattern.MatchString(h.WorkspaceSHA256) || h.WorkspaceEpoch != record.ExecutionWorkspace.WorkspaceEpoch {
		return fmt.Errorf("ownership handoff contains a missing or mismatched workspace seal")
	}
	if err := validateHandoffExternalStringBounds(h); err != nil || !validOptionalTimestamps(h) {
		return fmt.Errorf("ownership handoff external fields or timestamps are invalid")
	}
	switch h.State {
	case StateOwnershipDispatching, StateOwnershipDispatched:
		if h.OwnerSession != nil || h.Orientation != nil || h.Completion != nil {
			return fmt.Errorf("ownership dispatch state contains owner completion authority")
		}
	case StateOwnerOrienting:
		if !validWorkerSession(h.OwnerSession) || h.Orientation != nil || h.Completion != nil {
			return fmt.Errorf("owner_orienting requires only owner identity")
		}
	case StateOwnerActive:
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion != nil {
			return fmt.Errorf("owner_active requires orientation and no completion")
		}
	case StateCleanupPendingHumanDecision:
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil || h.Cleanup != nil {
			return fmt.Errorf("cleanup_pending_human_decision requires immutable owner completion and no cleanup approval")
		}
	case StateRecoveryRequired:
		if h.OwnerSession == nil && h.Orientation == nil && h.Completion == nil {
			if h.PendingOperation == nil || !validFailure(h.Failure) {
				return fmt.Errorf("ownership dispatch recovery requires pending operation and failure")
			}
			return nil
		}
		if h.Cancellation != nil {
			if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion != nil || !validCancellationRecovery(h) {
				return fmt.Errorf("ownership cancellation recovery requires the active owner context and cancellation tombstone")
			}
			return nil
		}
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil {
			return fmt.Errorf("ownership terminal recovery requires owner completion")
		}
	case StateCleanupExecuting:
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil || h.Cleanup == nil || !validWorkerSession(h.Cleanup.ApprovedBySession) || !sha256Pattern.MatchString(h.Cleanup.InventoryFingerprint) {
			return fmt.Errorf("ownership cleanup execution requires explicit human approval")
		}
	case StateClosed:
		if h.ClosedDisposition == DispositionCancelled && h.Completion == nil && validFinalizedCancellation(h) {
			return nil
		}
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil {
			return fmt.Errorf("ownership terminal state requires owner completion")
		}
	default:
		return fmt.Errorf("unknown ownership handoff state")
	}
	return nil
}

func validFinalizedCancellation(h *model.IssueOpsExecutionHandoff) bool {
	return h != nil && h.Cancellation == nil && validFailure(h.Failure) && h.Failure.Code == "cancellation_finalized"
}

func validCancellationRecovery(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil || h.Cancellation == nil || h.Cancellation.RequestedAt == "" || !canonicalTimestamp(h.Cancellation.RequestedAt) || strings.TrimSpace(h.Cancellation.Reason) == "" || h.Cancellation.Reason != redact(h.Cancellation.Reason) || !validFailure(h.Failure) {
		return false
	}
	return h.Failure.Code == "cancellation_requested" && h.Failure.Message == h.Cancellation.Reason && h.Failure.At == h.Cancellation.RequestedAt
}

func validOwnershipOrientation(record model.IssueOpsRecord, orientation *model.IssueOpsOwnershipOrientation) bool {
	if orientation == nil || orientation.IssueURL != record.IssueURL || !sha256Pattern.MatchString(orientation.PlanSHA256) || !canonicalTimestamp(orientation.RecordedAt) {
		return false
	}
	for _, value := range []string{orientation.Understanding, orientation.ScopeConfirmation} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || len(value) > 4096 {
			return false
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return false
			}
		}
	}
	return true
}

func ownershipState(state string) bool {
	return strings.TrimSpace(state) != ""
}
