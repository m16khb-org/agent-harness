package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

// CompleteIssueOpsOwnershipTransfer records the transferred owner's finished
// work without starting resource cleanup. Cleanup is deliberately left in
// cleanup_pending_human_decision for a later human decision in the source root.
func CompleteIssueOpsOwnershipTransfer(stateRoot string, req IssueOpsHandoffFinishRequest) (IssueOpsRecord, error) {
	req = normalizeHandoffFinishRequest(req)
	if req.ID == "" || req.Attempt < 1 || req.OwnershipEpoch == "" || req.ContextSHA256 == "" || req.FinalHead == "" || req.TuringReportPath == "" || len(req.Verification) == 0 {
		return IssueOpsRecord{}, fmt.Errorf("ownership completion requires id, exact fence, final head, and verification")
	}
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		h := record.ExecutionHandoff
		actor := IssueOpsActor{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID, CWD: req.CWD}
		if h == nil || h.ProtocolVersion != handoff.OwnershipTransferProtocolVersion || h.State != handoff.StateOwnerActive {
			return fmt.Errorf("ownership completion requires an active ownership-transfer handoff")
		}
		if req.Attempt != h.Attempt || req.OwnershipEpoch != h.OwnershipEpoch || req.ContextSHA256 != h.ContextSHA256 {
			return fmt.Errorf("ownership completion requires the exact active handoff fence")
		}
		if err := validatePostTransferMutation(record, &actor); err != nil {
			return err
		}
		if record.Phase != model.IssueOpsPhasePR || h.PublishReceipt == nil || h.PublishReceipt.FinalHead != req.FinalHead {
			return fmt.Errorf("ownership completion requires the exact published pr-phase head")
		}
		if record.RemoteArtifact == nil {
			return fmt.Errorf("ownership completion requires a durable remote artifact")
		}
		if record.RemoteCreateClaim != nil {
			return fmt.Errorf("ownership completion requires no active remote-create claim")
		}
		if err := validateOwnershipCompletionWorkerEvidence(record, req); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		h.Completion = &model.IssueOpsOwnershipCompletion{FinalHead: req.FinalHead, ChangedFiles: req.ChangedFiles, TuringReport: req.TuringReportPath, Verification: req.Verification, CompletedAt: now}
		projection, err := ownershipCompletionProjection(record, req, now)
		if err != nil {
			return err
		}
		h.WorkerDoneProjection = &projection
		h.State = handoff.StateCleanupPendingHumanDecision
		record.Phase = model.IssueOpsPhaseDone
		h.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

// CompleteIssueOpsOwnershipTransferWithProjection sends only the durable
// completion notification. It never invokes Orca cleanup operations.
func CompleteIssueOpsOwnershipTransferWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffFinishRequest, client IssueOpsWorkerDoneProjectionClient) (IssueOpsRecord, error) {
	record, err := CompleteIssueOpsOwnershipTransfer(stateRoot, req)
	if err != nil || record.ExecutionHandoff == nil || record.ExecutionHandoff.WorkerDoneProjection == nil || client == nil {
		return record, err
	}
	projection := record.ExecutionHandoff.WorkerDoneProjection
	result, sendErr := client.SendWorkerDone(ctx, workerDoneRequestFromProjection(projection))
	return persistWorkerDoneProjectionOutcome(stateRoot, record.ID, sendErr, result)
}

func validateOwnershipCompletionWorkerEvidence(record IssueOpsRecord, req IssueOpsHandoffFinishRequest) error {
	h := record.ExecutionHandoff
	if h == nil {
		return fmt.Errorf("ownership completion requires handoff evidence")
	}
	head := strings.TrimSpace(preflight.GitOut(h.WorkerRoot, "rev-parse", "HEAD^{commit}"))
	if head == "" || head != req.FinalHead {
		return fmt.Errorf("ownership completion final head does not match clean worker HEAD")
	}
	if strings.TrimSpace(preflight.GitOut(h.WorkerRoot, "status", "--porcelain")) != "" {
		return fmt.Errorf("ownership completion requires a clean worker checkout")
	}
	report := filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(req.TuringReportPath)))
	if !filepath.IsAbs(report) || !pathutil.PathWithin(report, h.WorkerRoot) {
		return fmt.Errorf("ownership completion report path is outside the worker root")
	}
	return nil
}

func ownershipCompletionProjection(record IssueOpsRecord, req IssueOpsHandoffFinishRequest, now string) (model.IssueOpsExecutionHandoffWorkerDoneProjection, error) {
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil || h.OwnerSession == nil || strings.TrimSpace(h.Orca.WorkerMailboxHandle) == "" || strings.TrimSpace(h.CoordinatorMailboxHandle) == "" {
		return model.IssueOpsExecutionHandoffWorkerDoneProjection{}, fmt.Errorf("ownership completion requires sealed notification identities")
	}
	report := filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(req.TuringReportPath)))
	subject := fmt.Sprintf("IssueOps %s ownership work complete", record.ID)
	body := fmt.Sprintf("IssueOps %s ownership work completed at %s. No coordinator acceptance or cleanup has run; cleanup awaits a human decision in the source checkout.", record.ID, req.FinalHead)
	request := port.OrcaWorkerDoneRequest{FromHandle: h.Orca.WorkerMailboxHandle, ToHandle: h.CoordinatorMailboxHandle, Subject: subject, Body: body, TaskID: h.Orca.TaskID, DispatchID: h.Orca.DispatchID, ChangedFiles: append([]string(nil), req.ChangedFiles...), ReportPath: report}
	payload, err := json.Marshal(request)
	if err != nil {
		return model.IssueOpsExecutionHandoffWorkerDoneProjection{}, err
	}
	sum := sha256.Sum256(payload)
	return model.IssueOpsExecutionHandoffWorkerDoneProjection{Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch, State: workerDoneProjectionIntent, DiagnosticCode: "intent_persisted", PayloadSHA256: hex.EncodeToString(sum[:]), FromHandle: request.FromHandle, ToHandle: request.ToHandle, Subject: subject, Body: body, TaskID: request.TaskID, DispatchID: request.DispatchID, FinalHead: req.FinalHead, ChangedFiles: request.ChangedFiles, ReportPath: report, HostIdentity: h.OwnerSession.Host + "/" + h.OwnerSession.SessionID, StartedAt: now}, nil
}
