package issueopsstatus

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
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
