package issueopslease

import (
	"context"
	"fmt"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ClaimRequest struct {
	ID                  string
	Generation          uint64
	Actor               leasedomain.Actor
	Ancestry            []leasedomain.ProcessReceipt
	CWD                 string
	TokenFile           string
	IssueBodySHA256     string
	ContextPacketSHA256 string
}

type ClaimResult struct {
	OK        bool
	ID        string
	Execution leasecontract.Execution
}

type ClaimService struct {
	repository ClaimRepository
	clock      Clock
	inspect    ProcessInspector
	preflight  ClaimContextPreflight
}

func NewClaimService(repository ClaimRepository, clock Clock, inspect ProcessInspector, preflight ClaimContextPreflight) *ClaimService {
	return &ClaimService{repository: repository, clock: clock, inspect: inspect, preflight: preflight}
}

func (s *ClaimService) Claim(ctx context.Context, request ClaimRequest) (ClaimResult, error) {
	if s.preflight == nil {
		return ClaimResult{ID: request.ID}, fmt.Errorf("claim context preflight is required")
	}
	validate, err := s.preflight.Preflight(ctx, ClaimPreflightRequest{
		ID: request.ID, Generation: request.Generation, IssueBodySHA256: request.IssueBodySHA256, ContextPacketSHA256: request.ContextPacketSHA256,
	})
	if err != nil {
		return ClaimResult{ID: request.ID}, err
	}
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.inspect)
	if err != nil {
		return ClaimResult{ID: request.ID}, err
	}
	after, err := s.repository.Claim(ctx, ClaimRepositoryRequest{
		ID: request.ID, Generation: request.Generation, Actor: actor, CWD: request.CWD, TokenFile: request.TokenFile, ValidateRecord: validate, Clock: s.clock,
	})
	if err != nil {
		return ClaimResult{ID: request.ID}, err
	}
	return ClaimResult{OK: true, ID: request.ID, Execution: after.Execution}, nil
}
