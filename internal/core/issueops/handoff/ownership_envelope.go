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
	if h == nil || h.ProtocolVersion != OwnershipTransferProtocolVersion || record.ExecutionWorkspace == nil {
		return fmt.Errorf("ownership handoff requires execution workspace")
	}
	if !canonicalNonSpace(h.WorkspaceEpoch) || !sha256Pattern.MatchString(h.WorkspaceSHA256) || h.WorkspaceEpoch != record.ExecutionWorkspace.WorkspaceEpoch || h.WorkerSession != nil || h.Result != nil || h.AcceptedAt != "" {
		return fmt.Errorf("ownership handoff contains missing workspace seal or protocol-v1 authority")
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
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil {
			return fmt.Errorf("ownership terminal recovery requires owner completion")
		}
	case StateCleanupExecuting, StateClosed:
		if !validWorkerSession(h.OwnerSession) || !validOwnershipOrientation(record, h.Orientation) || h.Completion == nil {
			return fmt.Errorf("ownership terminal state requires owner completion")
		}
	default:
		return fmt.Errorf("unknown ownership handoff state")
	}
	return nil
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
