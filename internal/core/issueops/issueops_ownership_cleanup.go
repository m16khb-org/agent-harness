package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/preflight"
)

type IssueOpsOwnershipCleanupPreviewRequest struct {
	ID      string `json:"id"`
	Host    string `json:"host"`
	Session string `json:"session_id"`
	AgentID string `json:"agent_id,omitempty"`
	CWD     string `json:"cwd"`
}

type IssueOpsOwnershipCleanupPreview struct {
	ID                   string   `json:"id"`
	SourceCWD            string   `json:"source_cwd"`
	InventoryFingerprint string   `json:"inventory_fingerprint"`
	Choices              []string `json:"choices"`
}

type IssueOpsOwnershipCleanupApproveRequest struct {
	IssueOpsOwnershipCleanupPreviewRequest
	InventoryFingerprint string `json:"inventory_fingerprint"`
	Disposition          string `json:"disposition"`
	Reason               string `json:"reason"`
	Confirm              bool   `json:"confirm"`
}

type IssueOpsOwnershipCleanupRecordRequest struct {
	IssueOpsOwnershipCleanupPreviewRequest
	Step string `json:"step"`
}

func PreviewIssueOpsOwnershipCleanup(stateRoot string, req IssueOpsOwnershipCleanupPreviewRequest) (IssueOpsOwnershipCleanupPreview, error) {
	record, err := ReadIssueOps(stateRoot, strings.TrimSpace(req.ID))
	if err != nil {
		return IssueOpsOwnershipCleanupPreview{}, err
	}
	if err := validateOwnershipCleanupCandidate(record, req); err != nil {
		return IssueOpsOwnershipCleanupPreview{}, err
	}
	return IssueOpsOwnershipCleanupPreview{ID: record.ID, SourceCWD: record.Repo, InventoryFingerprint: ownershipCleanupInventoryFingerprint(record), Choices: []string{"retain resources (no command)", "close owner terminal and retain workspace", "remove local worker resources"}}, nil
}

func ApproveIssueOpsOwnershipCleanup(stateRoot string, req IssueOpsOwnershipCleanupApproveRequest) (IssueOpsRecord, error) {
	if !req.Confirm {
		return IssueOpsRecord{}, fmt.Errorf("ownership cleanup approval requires --confirm")
	}
	disposition := strings.TrimSpace(strings.ToLower(req.Disposition))
	if disposition != "close-owner" && disposition != "remove-local" {
		return IssueOpsRecord{}, fmt.Errorf("ownership cleanup disposition must be close-owner or remove-local")
	}
	reason := strings.TrimSpace(policy.RedactFreeform(req.Reason))
	if reason == "" || len(reason) > IssueOpsHandoffForceAbandonReasonBytes {
		return IssueOpsRecord{}, fmt.Errorf("ownership cleanup approval requires a nonempty bounded reason")
	}
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, strings.TrimSpace(req.ID), func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, strings.TrimSpace(req.ID))
		if err != nil {
			return err
		}
		if err := validateOwnershipCleanupCandidate(record, req.IssueOpsOwnershipCleanupPreviewRequest); err != nil {
			return err
		}
		fingerprint := ownershipCleanupInventoryFingerprint(record)
		if req.InventoryFingerprint != fingerprint {
			return fmt.Errorf("ownership cleanup inventory changed; preview again before approval")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		h := retainedCleanupHandoff(record)
		h.Cleanup = &model.IssueOpsExecutionHandoffCleanup{Disposition: disposition, Reason: reason, ApprovedAt: now, ApprovedBySession: &model.IssueOpsHostSessionIdentity{Host: strings.TrimSpace(req.Host), SessionID: strings.TrimSpace(req.Session), AgentID: strings.TrimSpace(req.AgentID)}, InventoryFingerprint: fingerprint}
		h.State = handoff.StateCleanupExecuting
		h.UpdatedAt, record.UpdatedAt = now, now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

// RecordIssueOpsOwnershipCleanup records a single observed cleanup receipt.
// It deliberately does not invoke Orca or Git: the authenticated source
// session performs the human-approved external operation first, then supplies
// a fresh observation to this function for ordered durable recording.
func RecordIssueOpsOwnershipCleanup(ctx context.Context, stateRoot string, req IssueOpsOwnershipCleanupRecordRequest, client any) (IssueOpsRecord, error) {
	step := strings.TrimSpace(strings.ToLower(req.Step))
	validated, err := ReadIssueOps(stateRoot, strings.TrimSpace(req.ID))
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if err := validateOwnershipCleanupExecutor(validated, req.IssueOpsOwnershipCleanupPreviewRequest); err != nil {
		return IssueOpsRecord{}, err
	}
	h := retainedCleanupHandoff(validated)
	expected := ownershipCleanupSteps(h.Cleanup.Disposition)
	if len(h.Cleanup.Receipts) >= len(expected) || step != expected[len(h.Cleanup.Receipts)] {
		return IssueOpsRecord{}, fmt.Errorf("ownership cleanup receipt %q is out of order for disposition %s", step, h.Cleanup.Disposition)
	}
	receipt, err := verifyOwnershipCleanupStep(ctx, validated, step, client, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return IssueOpsRecord{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, validated.ID, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, validated.ID)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("ownership cleanup changed during receipt verification")
		}
		h := retainedCleanupHandoff(current)
		if h == nil {
			return fmt.Errorf("ownership cleanup lost retained completion authority")
		}
		h.Cleanup.Receipts = append(h.Cleanup.Receipts, receipt)
		h.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		current.UpdatedAt = h.UpdatedAt
		if len(h.Cleanup.Receipts) == len(expected) {
			h.State = handoff.StateClosed
			if h.Cleanup.Disposition == "close-owner" {
				h.ClosedDisposition = handoff.DispositionOwnerClosedWorkspaceRetained
			} else {
				h.ClosedDisposition = handoff.DispositionLocalResourcesRemoved
			}
		}
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return persisted, err
}

func ownershipCleanupSteps(disposition string) []string {
	if disposition == "close-owner" {
		return []string{"task_terminal", "terminal_quiescent"}
	}
	return []string{"remote_head_safe", "task_terminal", "terminal_quiescent", "worktree_removed", "local_branch_removed"}
}

func validateOwnershipCleanupExecutor(record IssueOpsRecord, req IssueOpsOwnershipCleanupPreviewRequest) error {
	h := retainedCleanupHandoff(record)
	if h == nil || h.State != handoff.StateCleanupExecuting || h.Cleanup == nil || h.Cleanup.ApprovedBySession == nil || pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(record.Repo) {
		return fmt.Errorf("ownership cleanup receipt requires approved cleanup execution from the exact source root")
	}
	want := h.Cleanup.ApprovedBySession
	if !strings.EqualFold(strings.TrimSpace(req.Host), strings.TrimSpace(want.Host)) || strings.TrimSpace(req.Session) != strings.TrimSpace(want.SessionID) || strings.TrimSpace(req.AgentID) != strings.TrimSpace(want.AgentID) {
		return fmt.Errorf("ownership cleanup receipt requires the human-approved source session")
	}
	return nil
}

func verifyOwnershipCleanupStep(ctx context.Context, record IssueOpsRecord, step string, client any, now string) (model.IssueOpsExecutionHandoffCleanupReceipt, error) {
	if step == "remote_head_safe" {
		h := retainedCleanupHandoff(record)
		if h.PublishReceipt == nil || h.Completion == nil || h.PublishReceipt.FinalHead != h.Completion.FinalHead {
			return model.IssueOpsExecutionHandoffCleanupReceipt{}, fmt.Errorf("ownership cleanup requires the verified remote head")
		}
		return model.IssueOpsExecutionHandoffCleanupReceipt{Step: step, RecordedAt: now}, nil
	}
	if step == "local_branch_removed" {
		code, _, stderr := preflight.GitCmd(record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/"+record.Branch)
		if code != 1 {
			return model.IssueOpsExecutionHandoffCleanupReceipt{}, fmt.Errorf("ownership cleanup local branch remains or cannot be verified: %s", strings.TrimSpace(stderr))
		}
		return model.IssueOpsExecutionHandoffCleanupReceipt{Step: step, RecordedAt: now}, nil
	}
	return verifyIssueOpsCleanupStep(ctx, record, step, client, now)
}

func validateOwnershipCleanupCandidate(record IssueOpsRecord, req IssueOpsOwnershipCleanupPreviewRequest) error {
	h := retainedCleanupHandoff(record)
	if h == nil || h.State != handoff.StateCleanupPendingHumanDecision || h.Completion == nil || h.Cleanup != nil {
		return fmt.Errorf("ownership cleanup requires cleanup_pending_human_decision")
	}
	if pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(record.Repo) || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Session) == "" {
		return fmt.Errorf("ownership cleanup preview requires an authenticated exact source root")
	}
	if h.OwnerSession != nil && strings.EqualFold(strings.TrimSpace(req.Host), strings.TrimSpace(h.OwnerSession.Host)) && strings.TrimSpace(req.Session) == strings.TrimSpace(h.OwnerSession.SessionID) && strings.TrimSpace(req.AgentID) == strings.TrimSpace(h.OwnerSession.AgentID) {
		return fmt.Errorf("completed owner cannot become a source cleanup candidate")
	}
	return nil
}

func ownershipCleanupInventoryFingerprint(record IssueOpsRecord) string {
	h := retainedCleanupHandoff(record)
	values := []string{record.ID, record.Repo, record.Branch, h.WorkspaceEpoch, h.WorkerRoot, h.Completion.FinalHead}
	if h.Orca != nil {
		values = append(values, h.Orca.WorktreeID, h.Orca.WorkerTerminalHandle, h.Orca.TaskID, h.Orca.DispatchID)
	}
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}
