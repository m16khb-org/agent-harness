package issueopspublication

import (
	"context"
	"errors"
	"fmt"

	application "agent-harness/internal/application/issueopspublication"
	contract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/port"
)

type ProviderCreateFunc func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, error)
type ProviderInspectFunc func(context.Context, contract.Intent) (contract.Inventory, bool, error)

type ProviderGateway struct {
	create  ProviderCreateFunc
	inspect ProviderInspectFunc
}

func NewProviderGateway(create ProviderCreateFunc, inspect ProviderInspectFunc) *ProviderGateway {
	return &ProviderGateway{create: create, inspect: inspect}
}

func (g *ProviderGateway) Create(ctx context.Context, provider string, request contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
	if g == nil || g.create == nil {
		return contract.ProviderCreateResult{}, contract.InvocationUnknown, fmt.Errorf("publication provider create bridge is required")
	}
	result, err := g.create(ctx, provider, request.Clone())
	invocation := contract.InvocationUnknown
	if typed, ok := errors.AsType[*port.IssueProviderCreateError](err); ok && !typed.Invoked {
		invocation = contract.InvocationNotInvokedProven
	}
	return result, invocation, err
}

func (g *ProviderGateway) Inspect(ctx context.Context, intent contract.Intent) (contract.Inventory, bool, error) {
	if g == nil || g.inspect == nil {
		return contract.Inventory{}, false, fmt.Errorf("publication provider inventory bridge is required")
	}
	inventory, attempted, err := g.inspect(ctx, intent.Clone())
	return inventory.Clone(), attempted, err
}

var _ application.Provider = (*ProviderGateway)(nil)
