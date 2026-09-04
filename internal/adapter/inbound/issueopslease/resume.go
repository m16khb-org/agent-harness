package issueopslease

import (
	"context"
	"errors"
	issueopscontract "issueops/internal/contract/issueops"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ResumeHandler struct{ service *leaseapp.ResumeService }

func NewResumeHandler(service *leaseapp.ResumeService) issueopscontract.ExecutionResumeHandler {
	return ResumeHandler{service: service}.Handle
}

func (h ResumeHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionResumeRequest) (issueopscontract.ExecutionResumeResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionResumeResult{ID: request.ID}, issueopscontract.ErrResumeHandlerUnavailable
	}
	result, err := h.service.Resume(ctx, leaseapp.ResumeRequest{ID: request.ID, ExpectedGeneration: request.ExpectedGeneration, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor), CWD: request.CWD, Confirm: request.Confirm})
	if err != nil {
		return issueopscontract.ExecutionResumeResult{ID: request.ID}, publicResumeError(err)
	}
	artifacts := result.Receipt.Artifacts
	return issueopscontract.ExecutionResumeResult{OK: result.OK, ID: result.ID, ResumeDisposition: string(result.Disposition), Execution: toCoreExecution(result.Receipt.Execution), ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256, NextCommand: resumeNextCommand(result.ID, result.Receipt.Execution.Lease.Generation, artifacts)}, nil
}

func publicResumeError(err error) error {
	if leasedomain.DenyCodeOf(err) != "" {
		if cause := errors.Unwrap(err); cause != nil {
			return cause
		}
	}
	failure, ok := errors.AsType[*leasecontract.Failure](err)
	if !ok {
		return err
	}
	return failure.Cause
}

func resumeNextCommand(id string, generation uint64, artifacts leasecontract.ResumeArtifacts) string {
	return executionResumeNextCommand(id, generation, artifacts.ClaimTokenPath, artifacts.IssueBodySHA256, artifacts.ContextPacketSHA256)
}
