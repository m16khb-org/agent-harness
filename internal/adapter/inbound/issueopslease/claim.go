package issueopslease

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"context"
	"errors"
	"fmt"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ClaimHandler struct{ service *leaseapp.ClaimService }

func NewClaimHandler(service *leaseapp.ClaimService) issueopscontract.ExecutionClaimHandler {
	handler := ClaimHandler{service: service}
	return handler.Handle
}

func (h ClaimHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionClaimRequest, _ issueopscontract.ExecutionClaimDependencies) (issueopscontract.ExecutionResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, issueopscontract.ErrClaimHandlerUnavailable
	}
	result, err := h.service.Claim(ctx, leaseapp.ClaimRequest{
		ID: request.ID, Generation: request.Generation, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor),
		CWD: request.CWD, TokenFile: request.TokenFile, IssueBodySHA256: request.IssueBodySHA256, ContextPacketSHA256: request.ContextPacketSHA256,
	})
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, publicClaimError(err, request.Generation)
	}
	return issueopscontract.ExecutionResult{OK: result.OK, ID: result.ID, Execution: toCoreExecution(result.Execution)}, nil
}

func publicClaimError(err error, generation uint64) error {
	switch leasedomain.DenyCodeOf(err) {
	case leasedomain.DenyLeaseClaimable:
		return fmt.Errorf("lease is not claimable at generation %d", generation)
	case leasedomain.DenyCanonicalCWD:
		return fmt.Errorf("claim cwd must be the canonical worktree")
	case leasedomain.DenyClaimToken:
		return fmt.Errorf("claim token does not match the current generation")
	}
	failure, ok := errors.AsType[*leasecontract.Failure](err)
	if !ok {
		return err
	}
	return failure.Cause
}
