package issueopsretention

import (
	"context"
	"fmt"
	"time"

	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
	issueopsretentiondomain "agent-harness/internal/domain/issueopsretention"
)

type Service struct {
	repository Repository
	clock      Clock
}

func NewService(repository Repository, clock Clock) *Service {
	return &Service{repository: repository, clock: clock}
}

func (service *Service) Prune(
	ctx context.Context,
	stateRoot string,
	maxAge time.Duration,
	confirm bool,
) (issueopsretentioncontract.Result, error) {
	result := issueopsretentioncontract.Result{
		StateRoot: stateRoot,
		MaxAge:    maxAge.String(),
		DryRun:    !confirm,
		Pruned:    []string{},
		Kept:      []string{},
	}
	if service == nil || service.repository == nil || service.clock == nil {
		return result, fmt.Errorf("issueops retention dependencies are required")
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	cutoff := service.clock.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	ids, err := service.repository.ListIDs(ctx, stateRoot)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		record, err := service.repository.ReadUnchecked(ctx, stateRoot, id)
		if err != nil {
			return result, err
		}
		if !issueopsretentiondomain.IsPrunable(record, cutoff) {
			result.Kept = append(result.Kept, id)
			continue
		}
		if confirm {
			if err := service.repository.Delete(ctx, stateRoot, id); err != nil {
				return result, err
			}
		}
		result.Pruned = append(result.Pruned, id)
	}
	result.OK = true
	return result, nil
}
