package state

import (
	"fmt"
	"sort"
	"strings"
	"time"

	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
)

func (service *Service) Prune(maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error) {
	return service.prune("", maxAge, 0, confirm)
}

func (service *Service) PrunePrefix(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	if prefix == "" {
		return service.newPruneResult(maxAge, confirm), fmt.Errorf("prefix is required")
	}
	return service.prune(prefix, maxAge, maxRecords, confirm)
}

func (service *Service) prune(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	result := service.newPruneResult(maxAge, confirm)
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	if maxRecords < 0 {
		return result, fmt.Errorf("max records must be non-negative")
	}
	cutoff := service.now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	list, err := service.List()
	if err != nil {
		return result, err
	}
	result.Pruned, result.Kept = SelectPrune(list.Records, prefix, cutoff, maxRecords)
	for _, record := range result.Pruned {
		result.DeletedKeys = append(result.DeletedKeys, record.Key)
		if confirm {
			if err := service.Delete(record.Key); err != nil {
				return result, err
			}
		}
	}
	for _, record := range result.Kept {
		result.KeptKeys = append(result.KeptKeys, record.Key)
	}
	result.OK = true
	return result, nil
}

func (service *Service) newPruneResult(maxAge time.Duration, confirm bool) statecontract.StatePruneResult {
	return statecontract.StatePruneResult{
		OK:          false,
		StateDir:    service.stateDir(),
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []statecontract.StateListEntry{},
		KeptKeys:    []string{},
		Kept:        []statecontract.StateListEntry{},
	}
}

// SelectPrune separates records selected by prefix, age, and retained-count
// policy from records that must remain. Both outputs are sorted by key.
func SelectPrune(records []statecontract.StateListEntry, prefix string, cutoff time.Time, maxRecords int) ([]statecontract.StateListEntry, []statecontract.StateListEntry) {
	matching := make([]statecontract.StateListEntry, 0, len(records))
	kept := make([]statecontract.StateListEntry, 0, len(records))
	for _, record := range records {
		if prefix == "" || strings.HasPrefix(record.Key, prefix) {
			matching = append(matching, record)
			continue
		}
		kept = append(kept, record)
	}
	sort.Slice(matching, func(i, j int) bool {
		left, leftErr := statepath.ParseTime(matching[i].UpdatedAt)
		right, rightErr := statepath.ParseTime(matching[j].UpdatedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.Before(right)
		}
		return matching[i].Key < matching[j].Key
	})
	selected := make(map[string]bool, len(matching))
	for _, record := range matching {
		updatedAt, err := statepath.ParseTime(record.UpdatedAt)
		if err == nil && !updatedAt.IsZero() && updatedAt.Before(cutoff) {
			selected[record.Key] = true
		}
	}
	if maxRecords > 0 {
		retained := make([]statecontract.StateListEntry, 0, len(matching))
		for _, record := range matching {
			if !selected[record.Key] {
				retained = append(retained, record)
			}
		}
		for len(retained) > maxRecords {
			selected[retained[0].Key] = true
			retained = retained[1:]
		}
	}
	pruned := make([]statecontract.StateListEntry, 0, len(selected))
	for _, record := range matching {
		if selected[record.Key] {
			pruned = append(pruned, record)
		} else {
			kept = append(kept, record)
		}
	}
	sort.Slice(pruned, func(i, j int) bool { return pruned[i].Key < pruned[j].Key })
	sort.Slice(kept, func(i, j int) bool { return kept[i].Key < kept[j].Key })
	return pruned, kept
}
