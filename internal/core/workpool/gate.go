package workpool

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func ParentGateMissing(parentCycleID string) ([]string, []string) {
	parentCycleID = strings.TrimSpace(parentCycleID)
	if parentCycleID == "" {
		return nil, nil
	}
	pools, err := linkedPoolsForParent(parentCycleID)
	if err != nil {
		return []string{"pools_complete"}, []string{"failed to scan linked work pools: " + err.Error()}
	}
	missing := []string{}
	warnings := []string{}
	for _, pool := range pools {
		incomplete, err := poolIncomplete(pool)
		if err != nil {
			missing = append(missing, "pool_incomplete:"+pool.ID)
			warnings = append(warnings, "failed to inspect linked work pool "+pool.ID+": "+err.Error())
			continue
		}
		if incomplete {
			missing = append(missing, "pool_incomplete:"+pool.ID)
		}
	}
	sort.Strings(missing)
	return missing, warnings
}

func linkedPoolsForParent(parentCycleID string) ([]WorkPool, error) {
	entries, err := os.ReadDir(StateRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	pools := []WorkPool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "wp-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		poolID := strings.TrimSuffix(name, ".json")
		pool, err := ReadPool(poolID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", poolID, err)
		}
		if strings.TrimSpace(pool.ParentCycleID) == parentCycleID {
			pools = append(pools, pool)
		}
	}
	sort.Slice(pools, func(i, j int) bool {
		return pools[i].ID < pools[j].ID
	})
	return pools, nil
}

func poolIncomplete(pool WorkPool) (bool, error) {
	if strings.TrimSpace(pool.Status) != "closed" {
		return true, nil
	}
	return false, nil
}
