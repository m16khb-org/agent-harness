package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/port"
)

const (
	workerDoneProjectionIntent = "intent"
	workerDoneProjectionSent   = "sent"
	workerDoneProjectionFailed = "failed"
)

var workerDoneDiagnosticCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`)
var persistedWorkerMailboxHandlePattern = regexp.MustCompile(`^term[_-][A-Za-z0-9_-]+$`)

type IssueOpsWorkerDoneProjectionClient interface {
	SendWorkerDone(context.Context, port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error)
}

type issueOpsHandoffProjectionHooks struct {
	BeforeLockedRevalidation              func()
	AfterDurableSubmitAndProjectionIntent func(IssueOpsRecord) error
}

func FinishIssueOpsHandoffWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffFinishRequest, client IssueOpsWorkerDoneProjectionClient) (IssueOpsRecord, error) {
	return finishIssueOpsHandoffWithProjection(ctx, stateRoot, req, client, issueOpsHandoffProjectionHooks{})
}

func finishIssueOpsHandoffWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffFinishRequest, client IssueOpsWorkerDoneProjectionClient, hooks issueOpsHandoffProjectionHooks) (IssueOpsRecord, error) {
	req = normalizeHandoffFinishRequest(req)
	if err := validateHandoffFinishRequest(req); err != nil {
		return IssueOpsRecord{}, err
	}
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if err := validateHandoffResultIdentity(validated, req); err != nil {
		return IssueOpsRecord{}, err
	}
	if req.Outcome != handoff.OutcomeCompleted {
		return finishIssueOpsHandoffWithoutProjection(stateRoot, req)
	}
	if validated.ExecutionHandoff.State == handoff.StateClaimed {
		if err := validateHandoffContextSource(validated); err != nil {
			return IssueOpsRecord{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	finishRequest := projectionFinishRequest(req, now)
	if _, err := handoff.Finish(validated, finishRequest); err != nil {
		return IssueOpsRecord{}, err
	}
	if hooks.BeforeLockedRevalidation != nil {
		hooks.BeforeLockedRevalidation()
	}

	var persisted IssueOpsRecord
	winner := false
	err = withIssueOpsLock(ctx, stateRoot, req.ID, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, req.ID)
		if readErr != nil {
			return readErr
		}
		existingProjection := record.ExecutionHandoff.WorkerDoneProjection
		if existingProjection != nil && !retryableWorkerDoneProjection(existingProjection) {
			if _, finishErr := handoff.Finish(record, finishRequest); finishErr != nil {
				return finishErr
			}
			persisted = record
			return nil
		}
		if existingProjection == nil && !reflect.DeepEqual(record, validated) {
			return fmt.Errorf("handoff changed before atomic submitted projection intent")
		}
		if record.ExecutionHandoff.State == handoff.StateClaimed {
			if contextErr := validateHandoffContextSource(record); contextErr != nil {
				return contextErr
			}
		}
		record, readErr = handoff.Finish(record, finishRequest)
		if readErr != nil {
			return readErr
		}
		_, projection, code, preconditionErr := buildWorkerDoneProjection(record)
		if client == nil && preconditionErr == nil {
			code = "projection_dependency_unavailable"
			preconditionErr = fmt.Errorf("Orca worker_done projection dependency is unavailable")
		}
		if preconditionErr != nil {
			terminalizeWorkerDoneProjection(&projection, code)
		}
		record.ExecutionHandoff.WorkerDoneProjection = &projection
		record.ExecutionHandoff.UpdatedAt = projection.StartedAt
		if projection.CompletedAt != "" {
			record.ExecutionHandoff.UpdatedAt = projection.CompletedAt
		}
		record.UpdatedAt = record.ExecutionHandoff.UpdatedAt
		persisted, readErr = writeIssueOps(stateRoot, record)
		winner = readErr == nil && projection.State == workerDoneProjectionIntent
		return readErr
	})
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if hooks.AfterDurableSubmitAndProjectionIntent != nil {
		if err := hooks.AfterDurableSubmitAndProjectionIntent(persisted); err != nil {
			return persisted, err
		}
	}
	if !winner {
		return persisted, nil
	}
	request := workerDoneRequestFromProjection(persisted.ExecutionHandoff.WorkerDoneProjection)
	result, sendErr := client.SendWorkerDone(ctx, request)
	return persistWorkerDoneProjectionOutcome(stateRoot, persisted.ID, sendErr, result)
}

// retryableWorkerDoneProjection permits only a proven pre-invocation adapter
// validation failure to be rebuilt after a local compatibility repair. Any
// projection that may have reached Orca stays terminal to avoid duplicate
// coordinator notifications.
func retryableWorkerDoneProjection(projection *model.IssueOpsExecutionHandoffWorkerDoneProjection) bool {
	return projection != nil && projection.State == workerDoneProjectionFailed && !projection.Invoked && projection.DiagnosticCode == "worker_done_invalid"
}

func projectionFinishRequest(req IssueOpsHandoffFinishRequest, now string) handoff.FinishRequest {
	return handoff.FinishRequest{
		Fence:  handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
		Worker: model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
		Result: model.IssueOpsExecutionHandoffResult{
			Outcome: req.Outcome, FinalHead: req.FinalHead, ChangedFiles: req.ChangedFiles, TuringReportPath: req.TuringReportPath,
			Verification: req.Verification, CleanupReceipts: req.CleanupReceipts, EvidenceDigest: req.EvidenceDigest, TaskID: req.TaskID, DispatchID: req.DispatchID,
		}, Now: now,
	}
}

func buildWorkerDoneProjection(record IssueOpsRecord) (port.OrcaWorkerDoneRequest, model.IssueOpsExecutionHandoffWorkerDoneProjection, string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	projection := model.IssueOpsExecutionHandoffWorkerDoneProjection{State: workerDoneProjectionFailed, DiagnosticCode: "projection_precondition_failed", StartedAt: now, CompletedAt: now}
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateSubmitted || h.Result == nil || h.Orca == nil || h.WorkerSession == nil {
		return port.OrcaWorkerDoneRequest{}, projection, "submitted_evidence_incomplete", fmt.Errorf("submitted completed handoff evidence is incomplete")
	}
	if h.CompletedAt != "" {
		now = h.CompletedAt
		projection.StartedAt = now
		projection.CompletedAt = now
	}
	projection.Attempt = h.Attempt
	projection.OwnershipEpoch = h.OwnershipEpoch
	if err := validateHandoffAcceptEvidence(record, IssueOpsHandoffAcceptRequest{ID: record.ID, Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch, ContextSHA256: h.ContextSHA256, FinalHead: h.Result.FinalHead}); err != nil {
		return port.OrcaWorkerDoneRequest{}, projection, "worker_evidence_invalid", err
	}
	from := strings.TrimSpace(h.Orca.WorkerMailboxHandle)
	to := strings.TrimSpace(h.CoordinatorMailboxHandle)
	if !concreteOrcaTerminalHandlePattern.MatchString(to) || len(to) > 256 {
		return port.OrcaWorkerDoneRequest{}, projection, "coordinator_recipient_invalid", fmt.Errorf("sealed coordinator recipient is invalid")
	}
	if !persistedWorkerMailboxHandlePattern.MatchString(from) || len(from) > 256 || from == to {
		return port.OrcaWorkerDoneRequest{}, projection, "worker_mailbox_invalid", fmt.Errorf("sealed worker mailbox identity is invalid")
	}
	if h.Orca.TaskID == "" || h.Result.TaskID != h.Orca.TaskID || h.Orca.DispatchID == "" || h.Result.DispatchID != h.Orca.DispatchID {
		return port.OrcaWorkerDoneRequest{}, projection, "worker_message_identity_invalid", fmt.Errorf("persisted task and dispatch identities are incomplete")
	}
	for _, path := range h.Result.ChangedFiles {
		if strings.ContainsRune(path, ',') {
			return port.OrcaWorkerDoneRequest{}, projection, "changed_files_unrepresentable", fmt.Errorf("changed_files contains an Orca CSV delimiter")
		}
	}
	reportPath := filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(h.Result.TuringReportPath)))
	if !filepath.IsAbs(reportPath) || !pathutil.PathWithin(reportPath, h.WorkerRoot) {
		return port.OrcaWorkerDoneRequest{}, projection, "report_path_invalid", fmt.Errorf("Turing report path is outside the worker root")
	}
	hostIdentity := h.WorkerSession.Host + "/" + h.WorkerSession.SessionID
	if h.WorkerSession.AgentID != "" {
		hostIdentity += "/" + h.WorkerSession.AgentID
	}
	subject := fmt.Sprintf("IssueOps %s completed", record.ID)
	body := fmt.Sprintf("IssueOps %s completed at %s. Verification evidence is persisted for attempt %d on %s. The coordinator can inspect the submitted durable record.", record.ID, h.Result.FinalHead, h.Attempt, h.WorkerSession.Host)
	request := port.OrcaWorkerDoneRequest{
		FromHandle: from, ToHandle: to, Subject: subject, Body: body,
		TaskID: h.Result.TaskID, DispatchID: h.Result.DispatchID,
		ChangedFiles: append([]string(nil), h.Result.ChangedFiles...), ReportPath: reportPath,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return port.OrcaWorkerDoneRequest{}, projection, "payload_encode_failed", err
	}
	sum := sha256.Sum256(payload)
	projection = model.IssueOpsExecutionHandoffWorkerDoneProjection{
		Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch,
		State: workerDoneProjectionIntent, DiagnosticCode: "intent_persisted",
		PayloadSHA256: hex.EncodeToString(sum[:]), FromHandle: from, ToHandle: to,
		Subject: subject, Body: body, TaskID: h.Result.TaskID, DispatchID: h.Result.DispatchID,
		FinalHead: h.Result.FinalHead, ChangedFiles: append([]string(nil), h.Result.ChangedFiles...),
		ReportPath: reportPath, HostIdentity: hostIdentity, StartedAt: now,
	}
	return request, projection, "", nil
}

func terminalizeWorkerDoneProjection(projection *model.IssueOpsExecutionHandoffWorkerDoneProjection, code string) {
	if !workerDoneDiagnosticCodePattern.MatchString(code) {
		code = "projection_precondition_failed"
	}
	projection.State = workerDoneProjectionFailed
	projection.Invoked = false
	projection.DiagnosticCode = code
	if projection.StartedAt == "" {
		projection.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	projection.CompletedAt = projection.StartedAt
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
		projection := record.ExecutionHandoff.WorkerDoneProjection
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
		record.ExecutionHandoff.UpdatedAt = projection.CompletedAt
		record.UpdatedAt = projection.CompletedAt
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	if err != nil {
		current, readErr := ReadIssueOps(stateRoot, id)
		if readErr == nil && current.ExecutionHandoff != nil && current.ExecutionHandoff.WorkerDoneProjection != nil {
			return current, nil
		}
	}
	return persisted, err
}
