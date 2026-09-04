package issueopsrouting

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsroutingcontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

func (repository Repository) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate func(issueopsroutingcontract.Record) (issueopsroutingcontract.Record, bool, error),
) (issueopsroutingcontract.Record, error) {
	return repository.Store.Update(ctx, stateRoot, id, mutate)
}
