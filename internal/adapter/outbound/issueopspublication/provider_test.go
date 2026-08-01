package issueopspublication

import (
	"context"
	"errors"
	"testing"

	contract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/port"
)

func TestProviderGatewayClassifiesInvocation(t *testing.T) {
	tests := []struct {
		name        string
		providerErr error
		want        contract.InvocationState
	}{
		{name: "success", want: contract.InvocationUnknown},
		{name: "not invoked", providerErr: &port.IssueProviderCreateError{Invoked: false, Err: errors.New("preflight")}, want: contract.InvocationNotInvokedProven},
		{name: "ambiguous", providerErr: &port.IssueProviderCreateError{Invoked: true, Err: errors.New("timeout")}, want: contract.InvocationUnknown},
		{name: "untyped", providerErr: errors.New("transport"), want: contract.InvocationUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := NewProviderGateway(
				func(_ context.Context, provider string, request contract.ProviderCreateRequest) (contract.ProviderCreateResult, error) {
					if provider != "github" || request.Title != "title" {
						t.Fatalf("provider=%q request=%#v", provider, request)
					}
					return contract.ProviderCreateResult{URL: "url"}, test.providerErr
				},
				nil,
			)
			result, invocation, err := gateway.Create(context.Background(), "github", contract.ProviderCreateRequest{Title: "title"})
			if result.URL != "url" || invocation != test.want || err != test.providerErr {
				t.Fatalf("result=%#v invocation=%q err=%v", result, invocation, err)
			}
		})
	}
}

func TestProviderGatewayPreservesInventoryAttemptedTruthAndCopiesCandidates(t *testing.T) {
	labels := []string{"enhancement"}
	gateway := NewProviderGateway(nil, func(_ context.Context, intent contract.Intent) (contract.Inventory, bool, error) {
		if intent.OperationID != "op-1" {
			t.Fatalf("intent=%#v", intent)
		}
		return contract.Inventory{Candidates: []contract.Candidate{{URL: "url", Labels: labels}}}, false, errors.New("inventory unavailable")
	})

	inventory, attempted, err := gateway.Inspect(context.Background(), contract.Intent{OperationID: "op-1"})
	if attempted || err == nil || err.Error() != "inventory unavailable" || inventory.Candidates[0].URL != "url" {
		t.Fatalf("inventory=%#v attempted=%v err=%v", inventory, attempted, err)
	}
	labels[0] = "changed"
	if inventory.Candidates[0].Labels[0] != "enhancement" {
		t.Fatalf("inventory alias leaked: %#v", inventory)
	}
}

func TestProviderGatewayRejectsMissingOperation(t *testing.T) {
	gateway := NewProviderGateway(nil, nil)
	if _, _, err := gateway.Create(context.Background(), "github", contract.ProviderCreateRequest{}); err == nil || err.Error() != "publication provider create bridge is required" {
		t.Fatalf("create err=%v", err)
	}
	if _, _, err := gateway.Inspect(context.Background(), contract.Intent{}); err == nil || err.Error() != "publication provider inventory bridge is required" {
		t.Fatalf("inspect err=%v", err)
	}
}
