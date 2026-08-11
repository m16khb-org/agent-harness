package issueopsdecision

import (
	"context"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
)

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate func(issueopsdecisioncontract.Record) (issueopsdecisioncontract.Record, error),
) (issueopsdecisioncontract.Record, error) {
	return repository.Store.Update(
		ctx,
		stateRoot,
		id,
		func(record issueopsdecisioncontract.Record) (issueopsdecisioncontract.Record, bool, error) {
			record, err := mutate(record)
			return record, err == nil, err
		},
	)
}
