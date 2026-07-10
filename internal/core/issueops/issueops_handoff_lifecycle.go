package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
)

type IssueOpsHandoffClaimRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt"`
	OwnershipEpoch string `json:"ownership_epoch"`
	ContextSHA256  string `json:"context_sha256"`
	Host           string `json:"host"`
	SessionID      string `json:"session_id"`
	AgentID        string `json:"agent_id,omitempty"`
	CWD            string `json:"cwd"`
	OrcaWorktreeID string `json:"orca_worktree_id"`
}

type IssueOpsHeartbeatRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt,omitempty"`
	OwnershipEpoch string `json:"ownership_epoch,omitempty"`
	ContextSHA256  string `json:"context_sha256,omitempty"`
	Host           string `json:"host,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
}

type IssueOpsHandoffFinishRequest struct {
	ID               string   `json:"id"`
	Attempt          int      `json:"attempt"`
	OwnershipEpoch   string   `json:"ownership_epoch"`
	ContextSHA256    string   `json:"context_sha256"`
	Host             string   `json:"host"`
	SessionID        string   `json:"session_id"`
	AgentID          string   `json:"agent_id,omitempty"`
	Outcome          string   `json:"outcome"`
	FinalHead        string   `json:"final_head,omitempty"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	TuringReportPath string   `json:"turing_report_path,omitempty"`
	Verification     []string `json:"verification,omitempty"`
	CleanupReceipts  []string `json:"cleanup_receipts,omitempty"`
	EvidenceDigest   string   `json:"evidence_digest,omitempty"`
	TaskID           string   `json:"task_id,omitempty"`
	DispatchID       string   `json:"dispatch_id,omitempty"`
}

type IssueOpsHandoffAcceptRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt"`
	OwnershipEpoch string `json:"ownership_epoch"`
	ContextSHA256  string `json:"context_sha256"`
	FinalHead      string `json:"final_head"`
}

func ClaimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if err := validateHandoffClaim(record, req); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Claim(record, handoff.ClaimRequest{
			Fence:      handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
			Worker:     model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
			WorkerRoot: pathutil.CleanAbsPath(req.CWD), Now: now,
		})
		if err != nil {
			return err
		}
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateHandoffClaim(record IssueOpsRecord, req IssueOpsHandoffClaimRequest) error {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
		return fmt.Errorf("dispatched Orca handoff is required")
	}
	if strings.TrimSpace(req.ContextSHA256) == "" || req.ContextSHA256 != record.ExecutionHandoff.ContextSHA256 {
		return fmt.Errorf("stale handoff context")
	}
	if pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(record.ExecutionHandoff.WorkerRoot) || pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(record.WorktreePath) {
		return fmt.Errorf("worker cwd/root does not match handoff worktree")
	}
	if strings.TrimSpace(req.OrcaWorktreeID) == "" || req.OrcaWorktreeID != record.ExecutionHandoff.Orca.WorktreeID {
		return fmt.Errorf("Orca worktree locator does not match handoff")
	}
	if branch := pathutil.GitBranchFromHead(req.CWD); branch == "" || branch != record.Branch {
		return fmt.Errorf("worker branch does not match handoff branch")
	}
	if head := issueOpsCurrentHead(record); head != "" && record.BranchPrepare != nil && record.BranchPrepare.BaseSHA != "" && head != record.BranchPrepare.BaseSHA {
		return fmt.Errorf("worker head does not match prepared base lineage")
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.SessionID) == "" {
		return fmt.Errorf("native host and session identity are required")
	}
	return nil
}

func RecordIssueOpsHeartbeatWithRequest(stateRoot string, req IssueOpsHeartbeatRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Phase == IssueOpsPhaseDone {
			persisted = record
			return nil
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if record.ExecutionHandoff != nil {
			record, err = handoff.Heartbeat(record, handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256}, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, now)
			if err != nil {
				return err
			}
		}
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func FinishIssueOpsHandoff(stateRoot string, req IssueOpsHandoffFinishRequest) (IssueOpsRecord, error) {
	if err := validateHandoffFinishRequest(req); err != nil {
		return IssueOpsRecord{}, err
	}
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
			return fmt.Errorf("execution handoff is required")
		}
		if req.TaskID != "" && req.TaskID != record.ExecutionHandoff.Orca.TaskID {
			return fmt.Errorf("worker result task id does not match handoff")
		}
		if req.DispatchID != "" && req.DispatchID != record.ExecutionHandoff.Orca.DispatchID {
			return fmt.Errorf("worker result dispatch id does not match handoff")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Finish(record, handoff.FinishRequest{
			Fence:  handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
			Worker: model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
			Result: model.IssueOpsExecutionHandoffResult{
				Outcome: req.Outcome, FinalHead: req.FinalHead, ChangedFiles: req.ChangedFiles, TuringReportPath: req.TuringReportPath,
				Verification: req.Verification, CleanupReceipts: req.CleanupReceipts, EvidenceDigest: req.EvidenceDigest, TaskID: req.TaskID, DispatchID: req.DispatchID,
			}, Now: now,
		})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateHandoffFinishRequest(req IssueOpsHandoffFinishRequest) error {
	if req.Outcome == handoff.OutcomeCompleted {
		if strings.TrimSpace(req.FinalHead) == "" || strings.TrimSpace(req.TuringReportPath) == "" || len(req.Verification) == 0 || len(req.CleanupReceipts) == 0 {
			return fmt.Errorf("completed finish requires final head, Turing report, verification, and cleanup receipts")
		}
	}
	if len(req.ChangedFiles) > 512 || len(req.Verification) > 128 || len(req.CleanupReceipts) > 128 {
		return fmt.Errorf("handoff result evidence exceeds bounded limits")
	}
	for _, value := range append(append(append([]string{}, req.ChangedFiles...), req.Verification...), req.CleanupReceipts...) {
		if len(value) > 4096 {
			return fmt.Errorf("handoff result evidence item exceeds bounded limit")
		}
	}
	return nil
}

func AcceptIssueOpsHandoff(stateRoot string, req IssueOpsHandoffAcceptRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if head := issueOpsCurrentHead(record); head != "" && head != strings.TrimSpace(req.FinalHead) {
			return fmt.Errorf("current worktree head does not match submitted head")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Accept(record, handoff.AcceptRequest{Fence: handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256}, FinalHead: req.FinalHead, Now: now})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}
