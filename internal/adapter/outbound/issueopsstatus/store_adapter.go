package issueopsstatus

import (
	"context"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"
)

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsstatuscontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}
