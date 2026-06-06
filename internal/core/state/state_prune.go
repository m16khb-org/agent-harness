package state

import (
	"fmt"
	"os"
	"sort"
	"time"
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
		updatedAt, err := parseStateTime(record.UpdatedAt)
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
