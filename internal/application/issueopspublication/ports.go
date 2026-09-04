package issueopspublication

import (
	"context"

	contract "issueops/internal/contract/issueopspublication"
)

type Repository interface {
	PreviewCreate(context.Context, contract.CreateCommand) (contract.PreparedCreate, error)
	BeginCreate(context.Context, contract.CreateCommand) (contract.Intent, error)
	LoadIntent(context.Context, string) (contract.Intent, error)
	MarkRetry(context.Context, contract.Intent) (contract.Intent, error)
	RecordFailure(context.Context, contract.Intent, contract.InvocationState, string, error) error
	Complete(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error)
	CompleteNotInvoked(context.Context, contract.Intent, error) (contract.RecordSnapshot, error)
	Latest(context.Context, string) (contract.RecordSnapshot, error)
}

type Provider interface {
	Create(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error)
	Inspect(context.Context, contract.Intent) (contract.Inventory, bool, error)
}

type Verifier interface {
	VerifyCandidate(context.Context, contract.Intent, contract.Candidate) error
	VerifyLive(context.Context, contract.Intent, string) error
}
