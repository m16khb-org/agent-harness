package issueopsinventory

import (
	"context"
	"fmt"
	"time"

	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
	issueopsinventorydomain "agent-harness/internal/domain/issueopsinventory"
)

type Service struct {
	repository Repository
	clock      Clock
	paths      PathNormalizer
}

func NewService(repository Repository, clock Clock, paths PathNormalizer) *Service {
	return &Service{repository: repository, clock: clock, paths: paths}
}

func (service *Service) ListCycles(
	ctx context.Context,
	stateRoot string,
	repo string,
) (issueopsinventorycontract.ListResult, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopsinventorycontract.ListResult{OK: false}, fmt.Errorf("issueops inventory dependencies are required")
	}
	records, diagnostics, err := service.repository.Scan(ctx, stateRoot)
	if err != nil {
		return issueopsinventorycontract.ListResult{OK: false}, err
	}
	repo = service.paths.Normalize(repo)
	result := issueopsinventorycontract.ListResult{
		OK:             true,
		GeneratedAt:    service.clock.Now().UTC().Format(time.RFC3339),
		ScannedRecords: len(records) + len(diagnostics),
		ReadErrors:     len(diagnostics),
		UnreadableIDs:  []string{},
		Diagnostics:    diagnostics,
		Entries:        []issueopsinventorycontract.ListEntry{},
	}
	for _, diagnostic := range diagnostics {
		result.UnreadableIDs = append(result.UnreadableIDs, diagnostic.ID)
	}
	for _, record := range records {
		if repo != "" && service.paths.Normalize(record.Repo) != repo {
			continue
		}
		result.Entries = append(result.Entries, issueopsinventorydomain.ProjectEntry(record))
	}
	return result, nil
}
