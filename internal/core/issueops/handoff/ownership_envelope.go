package handoff

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"agent-harness/internal/core/issueops/model"
)

func ValidateOwnershipLedger(record model.IssueOpsRecord) error {
	if record.ExecutionWorkspace != nil || record.ExecutionHandoff != nil {
		return fmt.Errorf("schema-v9 ownership authority must not use top-level execution fields")
	}
	switch record.CycleState {
	case model.IssueOpsCycleActive:
		if record.Phase == model.IssueOpsPhaseDone {
			return fmt.Errorf("active IssueOps cycle cannot be in done phase")
		}
	case model.IssueOpsCyclePaused:
		if record.Phase == model.IssueOpsPhaseDone {
			return fmt.Errorf("paused IssueOps cycle cannot be in done phase")
		}
	case model.IssueOpsCycleClosed:
		if record.Phase != model.IssueOpsPhaseDone {
			return fmt.Errorf("closed IssueOps cycle requires done phase")
		}
	default:
		return fmt.Errorf("unknown IssueOps cycle state")
	}

	ledger := record.Ownership
	if ledger == nil {
		if record.CycleState != model.IssueOpsCycleActive && record.CycleState != model.IssueOpsCycleClosed {
			return fmt.Errorf("paused IssueOps cycle requires ownership history")
		}
		return nil
	}
	if len(ledger.Attempts) == 0 {
		return fmt.Errorf("ownership ledger requires at least one attempt")
	}
	if record.CycleState == model.IssueOpsCycleActive && ledger.ActiveAttempt == 0 {
		return fmt.Errorf("active IssueOps ownership ledger requires an active attempt")
	}
	if record.CycleState != model.IssueOpsCycleActive && ledger.ActiveAttempt != 0 {
		return fmt.Errorf("paused or closed IssueOps cycle cannot retain an active attempt")
	}

	seenNumbers := make(map[int]struct{}, len(ledger.Attempts))
	seenWorkspaceEpochs := make(map[string]struct{}, len(ledger.Attempts))
	seenOwnershipEpochs := make(map[string]struct{}, len(ledger.Attempts))
	lastNumber := 0
	foundActive := false
	for index := range ledger.Attempts {
		attempt := &ledger.Attempts[index]
		if attempt.Number <= lastNumber {
			return fmt.Errorf("ownership attempt numbers must increase monotonically")
		}
		if _, exists := seenNumbers[attempt.Number]; exists {
			return fmt.Errorf("duplicate ownership attempt number")
		}
		seenNumbers[attempt.Number] = struct{}{}
		lastNumber = attempt.Number
		if attempt.Workspace == nil || attempt.Handoff == nil {
			return fmt.Errorf("ownership attempt requires workspace and handoff")
		}
		if attempt.Handoff.Attempt != attempt.Number || !canonicalNonSpace(attempt.Handoff.OwnershipEpoch) || !canonicalNonSpace(attempt.Workspace.WorkspaceEpoch) || attempt.Handoff.WorkspaceEpoch != attempt.Workspace.WorkspaceEpoch {
			return fmt.Errorf("ownership attempt number or epoch is inconsistent")
		}
		if _, exists := seenWorkspaceEpochs[attempt.Workspace.WorkspaceEpoch]; exists {
			return fmt.Errorf("ownership ledger reuses a workspace epoch")
		}
		if _, exists := seenOwnershipEpochs[attempt.Handoff.OwnershipEpoch]; exists {
			return fmt.Errorf("ownership ledger reuses an ownership epoch")
		}
		seenWorkspaceEpochs[attempt.Workspace.WorkspaceEpoch] = struct{}{}
		seenOwnershipEpochs[attempt.Handoff.OwnershipEpoch] = struct{}{}
		if attempt.StartedAt == "" || !canonicalTimestamp(attempt.StartedAt) || !canonicalTimestamp(attempt.ClosedAt) {
			return fmt.Errorf("ownership attempt timestamps are invalid")
		}

		isActive := ledger.ActiveAttempt == attempt.Number
		if isActive {
			foundActive = true
			if attempt.Handoff.State == StateClosed || attempt.ClosedAt != "" {
				return fmt.Errorf("active ownership attempt cannot be closed")
			}
		} else if attempt.Handoff.State != StateClosed || attempt.ClosedAt == "" {
			return fmt.Errorf("historical ownership attempt must be closed")
		}

		if index == 0 {
			if attempt.RestartedFrom != 0 || attempt.InheritedWIPSeal != nil {
				return fmt.Errorf("first ownership attempt cannot inherit restart evidence")
			}
		} else {
			if attempt.RestartedFrom <= 0 || attempt.RestartedFrom >= attempt.Number {
				return fmt.Errorf("successor ownership attempt requires an earlier restarted_from")
			}
			if _, exists := seenNumbers[attempt.RestartedFrom]; !exists {
				return fmt.Errorf("successor ownership attempt references missing predecessor")
			}
		}
		if attempt.InheritedWIPSeal != nil {
			if attempt.RestartedFrom == 0 || !validOwnershipWIPSeal(record.ID, attempt.Number, attempt.InheritedWIPSeal) {
				return fmt.Errorf("ownership attempt inherited WIP seal is invalid")
			}
		}

		projection := record
		projection.SchemaVersion = 8
		projection.CycleState = ""
		projection.Ownership = nil
		projection.ExecutionWorkspace = attempt.Workspace
		projection.ExecutionHandoff = attempt.Handoff
		if err := validateExecutionWorkspace(projection); err != nil {
			return fmt.Errorf("ownership attempt %d workspace: %w", attempt.Number, err)
		}
		if err := validateOwnershipEnvelope(projection); err != nil {
			return fmt.Errorf("ownership attempt %d handoff: %w", attempt.Number, err)
		}
	}
	if ledger.ActiveAttempt != 0 && !foundActive {
		return fmt.Errorf("active ownership attempt pointer does not resolve")
	}
	if ledger.PendingRestart != nil {
		if record.CycleState != model.IssueOpsCyclePaused || !validOwnerRestartIntent(record.ID, ledger.PendingRestart) {
			return fmt.Errorf("pending owner restart intent is invalid")
		}
	}
	return nil
}

func validOwnershipWIPSeal(id string, attempt int, seal *model.IssueOpsOwnershipWIPSeal) bool {
	if seal == nil {
		return false
	}
	wantRef := "refs/agent-harness/issueops/" + id + "/attempts/" + strconv.Itoa(attempt) + "/wip"
	return seal.Ref == wantRef && fullCommitPattern.MatchString(seal.Commit) && fullCommitPattern.MatchString(seal.Tree) && fullCommitPattern.MatchString(seal.BaseHead) && sha256Pattern.MatchString(seal.StatusSHA256) && sha256Pattern.MatchString(seal.PathManifestSHA256) && seal.PathCount > 0 && seal.CreatedAt != "" && canonicalTimestamp(seal.CreatedAt)
}

func validOwnerRestartIntent(id string, intent *model.IssueOpsOwnerRestartIntent) bool {
	if intent == nil || (intent.State != "intent" && intent.State != StateRecoveryRequired) || intent.FromAttempt <= 0 || intent.ToAttempt != intent.FromAttempt+1 || !sha256Pattern.MatchString(intent.InventoryFingerprint) || !fullCommitPattern.MatchString(intent.Head) || !sha256Pattern.MatchString(intent.StatusSHA256) || !validWorkerSession(intent.RequestedBy) || !canonicalNonSpace(intent.CoordinatorRecipient) || !canonicalNonSpace(intent.PriorWorktreeID) || !canonicalNonSpace(intent.PriorTaskID) || !canonicalNonSpace(intent.PriorDispatchID) || intent.StartedAt == "" || intent.UpdatedAt == "" || !canonicalTimestamp(intent.StartedAt) || !canonicalTimestamp(intent.UpdatedAt) {
		return false
	}
	if intent.Dirty && !intent.SealDirtyWIP {
		return false
	}
	if intent.WIPSeal != nil && !validOwnershipWIPSeal(id, intent.ToAttempt, intent.WIPSeal) {
		return false
	}
	if intent.State == StateRecoveryRequired {
		return validFailure(intent.Failure)
	}
	return intent.Failure == nil
}

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
