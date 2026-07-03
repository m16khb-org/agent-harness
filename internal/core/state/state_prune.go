package state

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/state/statepath"
)

func StatePrune(maxAge time.Duration, confirm bool) (StatePruneResult, error) {
	dir := StateDir()
	result := StatePruneResult{
		OK:          false,
		StateDir:    dir,
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []StateListEntry{},
		KeptKeys:    []string{},
		Kept:        []StateListEntry{},
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	list, err := StateList()
	if err != nil {
		return result, err
	}
	for _, record := range list.Records {
		updatedAt, err := statepath.ParseTime(record.UpdatedAt)
		if err != nil || updatedAt.IsZero() || !updatedAt.Before(cutoff) {
			result.Kept = append(result.Kept, record)
			result.KeptKeys = append(result.KeptKeys, record.Key)
			continue
		}
		result.Pruned = append(result.Pruned, record)
		result.DeletedKeys = append(result.DeletedKeys, record.Key)
		if confirm {
			if err := os.Remove(statePath(dir, record.Key)); err != nil && !os.IsNotExist(err) {
				return result, err
			}
		}
	}
	sort.Strings(result.DeletedKeys)
	sort.Strings(result.KeptKeys)
	sort.Slice(result.Pruned, func(i, j int) bool { return result.Pruned[i].Key < result.Pruned[j].Key })
	sort.Slice(result.Kept, func(i, j int) bool { return result.Kept[i].Key < result.Kept[j].Key })
	result.OK = true
	return result, nil
}

func StatePrunePrefix(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (StatePruneResult, error) {
	dir := StateDir()
	result := StatePruneResult{
		OK:          false,
		StateDir:    dir,
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []StateListEntry{},
		KeptKeys:    []string{},
		Kept:        []StateListEntry{},
	}
	if prefix == "" {
		return result, fmt.Errorf("prefix is required")
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	if maxRecords < 0 {
		return result, fmt.Errorf("max records must be non-negative")
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	list, err := StateList()
	if err != nil {
		return result, err
	}
	matching := []StateListEntry{}
	for _, record := range list.Records {
		if strings.HasPrefix(record.Key, prefix) {
			matching = append(matching, record)
			continue
		}
		result.Kept = append(result.Kept, record)
		result.KeptKeys = append(result.KeptKeys, record.Key)
	}
	sort.Slice(matching, func(i, j int) bool {
		left, leftErr := statepath.ParseTime(matching[i].UpdatedAt)
		right, rightErr := statepath.ParseTime(matching[j].UpdatedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.Before(right)
		}
		return matching[i].Key < matching[j].Key
	})
	pruneKeys := map[string]bool{}
	for _, record := range matching {
		updatedAt, err := statepath.ParseTime(record.UpdatedAt)
		if err == nil && !updatedAt.IsZero() && updatedAt.Before(cutoff) {
			pruneKeys[record.Key] = true
		}
	}
	if maxRecords > 0 {
		keptMatching := []StateListEntry{}
		for _, record := range matching {
			if !pruneKeys[record.Key] {
				keptMatching = append(keptMatching, record)
			}
		}
		for len(keptMatching) > maxRecords {
			record := keptMatching[0]
			pruneKeys[record.Key] = true
			keptMatching = keptMatching[1:]
		}
	}
	for _, record := range matching {
		if pruneKeys[record.Key] {
			result.Pruned = append(result.Pruned, record)
			result.DeletedKeys = append(result.DeletedKeys, record.Key)
			if confirm {
				if err := os.Remove(statePath(dir, record.Key)); err != nil && !os.IsNotExist(err) {
					return result, err
				}
			}
			continue
		}
		result.Kept = append(result.Kept, record)
		result.KeptKeys = append(result.KeptKeys, record.Key)
	}
	sort.Strings(result.DeletedKeys)
	sort.Strings(result.KeptKeys)
	sort.Slice(result.Pruned, func(i, j int) bool { return result.Pruned[i].Key < result.Pruned[j].Key })
	sort.Slice(result.Kept, func(i, j int) bool { return result.Kept[i].Key < result.Kept[j].Key })
	result.OK = true
	return result, nil
}
