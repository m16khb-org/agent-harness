package issueops

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
)

type IssueOpsHandoffAcknowledgeRequest struct {
	ID                string `json:"id"`
	Attempt           int    `json:"attempt"`
	OwnershipEpoch    string `json:"ownership_epoch"`
	ContextSHA256     string `json:"context_sha256"`
	Host              string `json:"host"`
	SessionID         string `json:"session_id"`
	AgentID           string `json:"agent_id,omitempty"`
	CWD               string `json:"cwd"`
	IssueURL          string `json:"issue_url"`
	PlanSHA256        string `json:"plan_sha256"`
	Understanding     string `json:"understanding"`
	ScopeConfirmation string `json:"scope_confirmation"`
}

func AcknowledgeIssueOpsHandoffContext(stateRoot string, req IssueOpsHandoffAcknowledgeRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		h := record.ExecutionHandoff
		if h == nil {
			return fmt.Errorf("ownership acknowledgement requires a handoff")
		}
		if h.State != handoff.StateOwnerOrienting && h.State != handoff.StateOwnerActive {
			return fmt.Errorf("ownership acknowledgement requires %s state", handoff.StateOwnerOrienting)
		}
		owner := model.IssueOpsHostSessionIdentity{Host: strings.TrimSpace(req.Host), SessionID: strings.TrimSpace(req.SessionID), AgentID: strings.TrimSpace(req.AgentID)}
		if h.Attempt != req.Attempt || h.OwnershipEpoch != strings.TrimSpace(req.OwnershipEpoch) || h.ContextSHA256 != strings.TrimSpace(req.ContextSHA256) || h.OwnerSession == nil || *h.OwnerSession != owner || pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(h.WorkerRoot) {
			return fmt.Errorf("ownership acknowledgement does not match the sealed owner fence and worker root")
		}
		packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
		if err != nil {
			return fmt.Errorf("render ownership acknowledgement context: %w", err)
		}
		orientation := model.IssueOpsOwnershipOrientation{
			IssueURL: strings.TrimSpace(req.IssueURL), PlanSHA256: strings.TrimSpace(req.PlanSHA256),
			Understanding: strings.TrimSpace(req.Understanding), ScopeConfirmation: strings.TrimSpace(req.ScopeConfirmation),
		}
		if orientation.IssueURL == "" || orientation.IssueURL != record.IssueURL || orientation.PlanSHA256 != packet.PlanSHA256 || orientation.Understanding == "" || orientation.ScopeConfirmation == "" || len(orientation.Understanding) > 4096 || len(orientation.ScopeConfirmation) > 4096 {
			return fmt.Errorf("ownership acknowledgement issue, plan, understanding, or scope confirmation is invalid")
		}
		if h.State == handoff.StateOwnerActive {
			if h.Orientation != nil {
				existing := *h.Orientation
				existing.RecordedAt = ""
				if reflect.DeepEqual(existing, orientation) {
					persisted = record
					return nil
				}
			}
			return fmt.Errorf("ownership acknowledgement conflicts with the recorded orientation")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		orientation.RecordedAt = now
		h.Orientation = &orientation
		h.State = handoff.StateOwnerActive
		h.LastHeartbeatAt = now
		h.UpdatedAt = now
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}
