package issueopsrouting

import (
	"context"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
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
