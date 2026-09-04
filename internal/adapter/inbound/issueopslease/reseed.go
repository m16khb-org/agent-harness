package issueopslease

import (
	"context"
	"errors"
	issueopscontract "issueops/internal/contract/issueops"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ReseedHandler struct{ service *leaseapp.ReseedService }

func NewReseedHandler(service *leaseapp.ReseedService) issueopscontract.ExecutionReseedHandler {
	return ReseedHandler{service: service}.Handle
}

func (h ReseedHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionReseedRequest) (issueopscontract.ExecutionReplaceResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionReplaceResult{ID: request.ID, Action: issueopscontract.ExecutionReplaceReseed}, issueopscontract.ErrReseedHandlerUnavailable
	}
	result, err := h.service.Reseed(ctx, leaseapp.ReseedRequest{
		ID: request.ID, ExpectedGeneration: request.ExpectedGeneration, CompletionGeneration: request.CompletionGeneration, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor),
		CWD: request.CWD, InventoryFingerprint: request.InventoryFingerprint, Reason: request.Reason, Confirm: request.Confirm,
	})
	if err != nil {
		return issueopscontract.ExecutionReplaceResult{ID: request.ID, Action: issueopscontract.ExecutionReplaceReseed}, publicReseedError(err)
	}
	response := issueopscontract.ExecutionReplaceResult{
		OK: true, ID: result.ID, Action: issueopscontract.ExecutionReplaceReseed, Execution: toCoreExecution(result.Execution),
		ClaimTokenPath: result.Receipt.ClaimTokenPath, IssueBodySHA256: result.Receipt.IssueBodySHA256,
		ContextPacketPath: result.Receipt.ContextPacketPath, ContextPacketSHA256: result.Receipt.ContextPacketSHA256,
		OwnerPromptPath: result.Receipt.OwnerPromptPath, OwnerPromptSHA256: result.Receipt.OwnerPromptSHA256,
	}
	response.NextCommand = executionReseedNextCommand(
		result.ID, result.Execution.Lease.Generation, result.Execution.Mode, result.Receipt.ClaimTokenPath,
	)
	return response, nil
}

func publicReseedError(err error) error {
	if leasedomain.DenyCodeOf(err) != "" {
		return errors.Unwrap(err)
	}
	failure, ok := errors.AsType[*leasecontract.Failure](err)
	if !ok {
		return err
	}
	return failure.Cause
}
