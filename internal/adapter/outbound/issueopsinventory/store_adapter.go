package issueopsinventory

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
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

func (repository Repository) Scan(
	ctx context.Context,
	stateRoot string,
) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error) {
	records, diagnostics, err := repository.Store.Scan(ctx, stateRoot)
	if err != nil {
		return nil, nil, err
	}
	result := make([]issueopsinventorycontract.RecordDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, issueopsinventorycontract.RecordDiagnostic{
			ID:   diagnostic.ID,
			Code: diagnostic.Code,
		})
	}
	return records, result, nil
}
