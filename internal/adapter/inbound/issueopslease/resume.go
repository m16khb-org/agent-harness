package issueopslease

import (
	"context"
	"errors"

	"agent-harness/internal/adapter/issueops"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ResumeHandler struct{ service *leaseapp.ResumeService }

func NewResumeHandler(service *leaseapp.ResumeService) issueops.ExecutionResumeHandler {
	return ResumeHandler{service: service}.Handle
}

func (h ResumeHandler) Handle(ctx context.Context, _ string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
	if h.service == nil {
		return issueops.ExecutionResumeResult{ID: request.ID}, issueops.ErrResumeHandlerUnavailable
	}
	result, err := h.service.Resume(ctx, leaseapp.ResumeRequest{ID: request.ID, ExpectedGeneration: request.ExpectedGeneration, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor), CWD: request.CWD, Confirm: request.Confirm})
	if err != nil {
		return issueops.ExecutionResumeResult{ID: request.ID}, publicResumeError(err)
	}
	artifacts := result.Receipt.Artifacts
	return issueops.ExecutionResumeResult{OK: result.OK, ID: result.ID, ResumeDisposition: string(result.Disposition), Execution: toCoreExecution(result.Receipt.Execution), ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256, NextCommand: resumeNextCommand(result.ID, result.Receipt.Execution.Lease.Generation, artifacts)}, nil
}

func publicResumeError(err error) error {
	if leasedomain.DenyCodeOf(err) != "" {
		if cause := errors.Unwrap(err); cause != nil {
			return cause
		}
	}
	var failure *leasecontract.Failure
	if !errors.As(err, &failure) {
		return err
	}
	return failure.Cause
}

func resumeNextCommand(id string, generation uint64, artifacts leasecontract.ResumeArtifacts) string {
	return issueops.ExecutionResumeNextCommand(id, generation, artifacts.ClaimTokenPath, artifacts.IssueBodySHA256, artifacts.ContextPacketSHA256)
}
