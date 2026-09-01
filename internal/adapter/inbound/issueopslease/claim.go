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
		CWD: request.CWD, TokenFile: request.TokenFile, ClaimCurrentToken: request.ClaimCurrentToken, IssueBodySHA256: request.IssueBodySHA256, ContextPacketSHA256: request.ContextPacketSHA256,
	})
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, publicClaimError(err, request.Generation, request.ID)
	}
	return issueopscontract.ExecutionResult{OK: result.OK, ID: result.ID, Execution: toCoreExecution(result.Execution)}, nil
}

func publicClaimError(err error, generation uint64, id string) error {
	switch leasedomain.DenyCodeOf(err) {
	case leasedomain.DenyLeaseClaimable:
		return claimableDenyError(generation, id)
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

// claimableDenyError는 claim이 막힌 자리에 회복 경로를 붙인다.
//
// `execution release`는 lease를 `released`로 두는데 그 상태는 claimable이
// 아니다. 다른 세션이 문서대로 `claim --claim-current-token`을 실행하면
// 세대만 알려주는 거절을 받고 멈춘다. 실제 회복은 `replace --preview` 다음
// `--reseed --confirm`이고, 그 경로는 `execution status`의 next_command에만
// 있었다. `issueops-implement`의 회복 표는 `replace`를 "holder 교체·회수"로
// 설명하므로 살아 있는 홀더를 뺏는 상황으로 읽히지, 정상 반납 뒤의 인계로는
// 읽히지 않는다. 그래서 거절 자체가 첫 명령을 들고 있어야 한다.
func claimableDenyError(generation uint64, id string) error {
	return fmt.Errorf(
		"lease is not claimable at generation %d: a released or superseded lease must be reseeded before any session can claim it; run `agent-harness issueops execution replace --id %s --expected-generation %d --preview` and follow the next_command it renders",
		generation, id, generation,
	)
}
