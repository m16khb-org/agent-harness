package issueopsinventory

import (
	"context"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsinventorycontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}
