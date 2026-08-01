package issueopspublication

import (
	"context"
	"fmt"
	"strings"

	contract "agent-harness/internal/contract/issueopspublication"
	domain "agent-harness/internal/domain/issueopspublication"
)

type CreateService struct {
	repository Repository
	provider   Provider
	verifier   Verifier
}

func NewCreateService(repository Repository, provider Provider, verifier Verifier) *CreateService {
	return &CreateService{repository: repository, provider: provider, verifier: verifier}
}

func (s *CreateService) Create(ctx context.Context, command contract.CreateCommand) (contract.ProviderCreateResult, error) {
	if s == nil || s.repository == nil || s.provider == nil || s.verifier == nil {
		return contract.ProviderCreateResult{}, fmt.Errorf("publication create dependencies are required")
	}
	if !command.Confirm {
		prepared, err := s.repository.PreviewCreate(ctx, command)
		if err != nil {
			return contract.ProviderCreateResult{}, err
		}
		if _, err := domain.ValidateCreateEligibility(prepared.Eligibility); err != nil {
			return contract.ProviderCreateResult{}, err
		}
		result, _, err := s.provider.Create(ctx, prepared.Eligibility.Provider, prepared.Request)
		return result, err
	}

	intent, err := s.repository.BeginCreate(ctx, command)
	if err != nil {
		return contract.ProviderCreateResult{}, err
	}
	if _, err := domain.ValidateCreateEligibility(intent.Eligibility); err != nil {
		return contract.ProviderCreateResult{}, err
	}
	result, invocation, callErr := s.provider.Create(ctx, intent.Provider, intent.Request)
	if callErr != nil {
		_ = s.repository.RecordFailure(ctx, intent, invocation, result.URL, callErr)
		return result, fmt.Errorf("remote create outcome requires execution reconcile; creation was not retried: %w", callErr)
	}
	if strings.TrimSpace(result.URL) == "" {
		cause := fmt.Errorf("provider create returned no canonical URL")
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, "", cause)
		return result, cause
	}
	if err := s.verifier.VerifyLive(ctx, intent, result.URL); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
		return result, fmt.Errorf("provider returned a URL but durable verification requires execution reconcile: %w", err)
	}
	if _, err := s.repository.Complete(ctx, intent, result.URL, true); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
		return result, fmt.Errorf("provider succeeded but durable receipt requires execution reconcile: %w", err)
	}
	return result, nil
}
