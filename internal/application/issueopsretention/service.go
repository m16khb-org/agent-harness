package issueopsretention

import (
	"context"
	"errors"
	"fmt"
	"time"

	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
	issueopsretentiondomain "agent-harness/internal/domain/issueopsretention"
)

type Service struct {
	repository Repository
	clock      Clock
}

const unreadableReportLimit = 20

var ErrIncomplete = errors.New("issueops retention incomplete")

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
		StateRoot:             stateRoot,
		MaxAge:                maxAge.String(),
		DryRun:                !confirm,
		Pruned:                []string{},
		Kept:                  []string{},
		Unreadable:            []string{},
		UnreadableDiagnostics: []issueopsretentioncontract.UnreadableRecordDiagnostic{},
		Failed:                []string{},
		DeleteDiagnostics:     []issueopsretentioncontract.DeleteFailureDiagnostic{},
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
			result.ReadErrors++
			if len(result.Unreadable) < unreadableReportLimit {
				result.Unreadable = append(result.Unreadable, id)
				result.UnreadableDiagnostics = append(result.UnreadableDiagnostics, issueopsretentioncontract.UnreadableRecordDiagnostic{
					ID:   id,
					Code: "read_failed",
				})
			}
			continue
		}
		if !issueopsretentiondomain.IsPrunable(record, cutoff) {
			result.Kept = append(result.Kept, id)
			continue
		}
		if confirm {
			if err := service.repository.DeleteIfUnchanged(ctx, stateRoot, id, record); err != nil {
				result.DeleteErrors++
				if len(result.Failed) < unreadableReportLimit {
					result.Failed = append(result.Failed, id)
					result.DeleteDiagnostics = append(
						result.DeleteDiagnostics,
						issueopsretentioncontract.DeleteFailureDiagnostic{ID: id, Code: "delete_failed"},
					)
				}
				result.Error = ErrIncomplete.Error()
				return result, fmt.Errorf("%w: delete %s: %v", ErrIncomplete, id, err)
			}
		}
		result.Pruned = append(result.Pruned, id)
	}
	if result.ReadErrors > 0 {
		result.Error = ErrIncomplete.Error()
		return result, fmt.Errorf("%w: %d unreadable record(s)", ErrIncomplete, result.ReadErrors)
	}
	result.OK = true
	return result, nil
}
