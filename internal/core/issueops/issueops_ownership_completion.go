package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const (
	workerDoneProjectionIntent = "intent"
	workerDoneProjectionSent   = "sent"
	workerDoneProjectionFailed = "failed"
)

var workerDoneDiagnosticCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`)

type IssueOpsWorkerDoneProjectionClient interface {
	SendWorkerDone(context.Context, port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error)
}

// CompleteIssueOpsHandoff records the transferred owner's finished
// work without starting resource cleanup. Cleanup is deliberately left in
// cleanup_pending_human_decision for a later human decision in the source root.
func CompleteIssueOpsHandoff(stateRoot string, req IssueOpsHandoffCompleteRequest) (IssueOpsRecord, error) {
	req = normalizeHandoffCompleteRequest(req)
	if req.ID == "" || req.Attempt < 1 || req.OwnershipEpoch == "" || req.ContextSHA256 == "" || req.FinalHead == "" || req.TuringReportPath == "" || len(req.Verification) == 0 {
		return IssueOpsRecord{}, fmt.Errorf("ownership completion requires id, exact fence, final head, and verification")
	}
	if err := validateHandoffCompleteRequest(req); err != nil {
		return IssueOpsRecord{}, err
	}
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		h := currentIssueOpsHandoff(record)
		actor := IssueOpsActor{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID, CWD: req.CWD}
		if h == nil || h.State != handoff.StateOwnerActive {
			return fmt.Errorf("ownership completion requires an active handoff")
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
		record.CycleState = IssueOpsCycleClosed
		CurrentOwnershipAttempt(record).ClosedAt = now
		record.Ownership.ActiveAttempt = 0
		h.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

// CompleteIssueOpsHandoffWithProjection sends only the durable
// completion notification. It never invokes Orca cleanup operations.
func CompleteIssueOpsHandoffWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffCompleteRequest, client IssueOpsWorkerDoneProjectionClient) (IssueOpsRecord, error) {
	record, err := CompleteIssueOpsHandoff(stateRoot, req)
	h := retainedCleanupHandoff(record)
	if err != nil || h == nil || h.WorkerDoneProjection == nil || client == nil {
		return record, err
	}
	projection := h.WorkerDoneProjection
	result, sendErr := client.SendWorkerDone(ctx, workerDoneRequestFromProjection(projection))
	return persistWorkerDoneProjectionOutcome(stateRoot, record.ID, sendErr, result)
}

func validateOwnershipCompletionWorkerEvidence(record IssueOpsRecord, req IssueOpsHandoffCompleteRequest) error {
	h := currentIssueOpsHandoff(record)
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

func ownershipCompletionProjection(record IssueOpsRecord, req IssueOpsHandoffCompleteRequest, now string) (model.IssueOpsExecutionHandoffWorkerDoneProjection, error) {
	h := currentIssueOpsHandoff(record)
	if h == nil || h.Orca == nil || h.OwnerSession == nil || strings.TrimSpace(h.Orca.WorkerMailboxHandle) == "" || strings.TrimSpace(h.CoordinatorMailboxHandle) == "" {
		return model.IssueOpsExecutionHandoffWorkerDoneProjection{}, fmt.Errorf("ownership completion requires sealed notification identities")
	}
	report := filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(req.TuringReportPath)))
	subject := fmt.Sprintf("IssueOps %s ownership work complete", record.ID)
	body := fmt.Sprintf("IssueOps %s ownership work completed at %s. Cleanup has not run and awaits a human decision in the source checkout.", record.ID, req.FinalHead)
	request := port.OrcaWorkerDoneRequest{FromHandle: h.Orca.WorkerMailboxHandle, ToHandle: h.CoordinatorMailboxHandle, Subject: subject, Body: body, TaskID: h.Orca.TaskID, DispatchID: h.Orca.DispatchID, ChangedFiles: append([]string(nil), req.ChangedFiles...), ReportPath: report}
	payload, err := json.Marshal(request)
	if err != nil {
		return model.IssueOpsExecutionHandoffWorkerDoneProjection{}, err
	}
	sum := sha256.Sum256(payload)
	return model.IssueOpsExecutionHandoffWorkerDoneProjection{Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch, State: workerDoneProjectionIntent, DiagnosticCode: "intent_persisted", PayloadSHA256: hex.EncodeToString(sum[:]), FromHandle: request.FromHandle, ToHandle: request.ToHandle, Subject: subject, Body: body, TaskID: request.TaskID, DispatchID: request.DispatchID, FinalHead: req.FinalHead, ChangedFiles: request.ChangedFiles, ReportPath: report, HostIdentity: h.OwnerSession.Host + "/" + h.OwnerSession.SessionID, StartedAt: now}, nil
}

func workerDoneRequestFromProjection(projection *model.IssueOpsExecutionHandoffWorkerDoneProjection) port.OrcaWorkerDoneRequest {
	return port.OrcaWorkerDoneRequest{
		FromHandle: projection.FromHandle, ToHandle: projection.ToHandle,
		Subject: projection.Subject, Body: projection.Body,
		TaskID: projection.TaskID, DispatchID: projection.DispatchID,
		ChangedFiles: append([]string(nil), projection.ChangedFiles...), ReportPath: projection.ReportPath,
	}
}

func persistWorkerDoneProjectionOutcome(stateRoot, id string, sendErr error, result port.OrcaWorkerDoneResult) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		h := retainedCleanupHandoff(record)
		if h == nil {
			return fmt.Errorf("worker-done outcome requires retained completion authority")
		}
		projection := h.WorkerDoneProjection
		if projection == nil || projection.State != workerDoneProjectionIntent {
			persisted = record
			return nil
		}
		projection.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		projection.Invoked = true
		if sendErr == nil {
			projection.State = workerDoneProjectionSent
			projection.DiagnosticCode = "sent"
			projection.MessageID = strings.TrimSpace(result.MessageID)
			projection.MessageSequence = result.Sequence
		} else {
			projection.State = workerDoneProjectionFailed
			projection.DiagnosticCode = "send_failed"
			var orcaErr *port.OrcaError
			if errors.As(sendErr, &orcaErr) {
				projection.Invoked = orcaErr.Invoked
				if workerDoneDiagnosticCodePattern.MatchString(strings.TrimSpace(orcaErr.Code)) {
					projection.DiagnosticCode = strings.TrimSpace(orcaErr.Code)
				}
			}
		}
		h.UpdatedAt = projection.CompletedAt
		record.UpdatedAt = projection.CompletedAt
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	if err != nil {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr == nil && retainedCleanupHandoff(current) != nil && retainedCleanupHandoff(current).WorkerDoneProjection != nil {
			return current, nil
		}
	}
	return persisted, err
}
