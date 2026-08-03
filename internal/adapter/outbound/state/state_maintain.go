package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
	stateapplication "agent-harness/internal/application/state"
	statecontract "agent-harness/internal/contract/state"
)

const storeMaintainSentinel = ".last-store-maintain"

func knownStoreRoots() []string {
	base := StateDir()
	workerRoot := filepath.Join(base, "worker")
	if dir := os.Getenv("HARNESS_WORKER_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			workerRoot = abs
		}
	}
	return []string{base, filepath.Join(base, "issueops_v1"), workerRoot, filepath.Join(base, "loop")}
}

func projectStoreRoots() ([]string, error) {
	projectsDir := filepath.Join(StateDir(), "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("discover project stores %s: %w", projectsDir, err)
	}
	roots := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, entry.Name())
		info, err := os.Lstat(filepath.Join(dir, "harness.db"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("discover project store %s: %w", dir, err)
		}
		if info.Mode().IsRegular() {
			roots = append(roots, dir)
		}
	}
	return roots, nil
}

func allStoreRoots() ([]string, error) {
	roots := knownStoreRoots()
	projectRoots, err := projectStoreRoots()
	if err != nil {
		return nil, err
	}
	return append(roots, projectRoots...), nil
}

func storeExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "harness.db"))
	return err == nil
}

func maintainStore(root string) (statecontract.StoreMaintainResult, error) {
	database, err := sqlstore.Open(root)
	if err != nil {
		return statecontract.StoreMaintainResult{}, err
	}
	return database.Maintain()
}

func sentinelModified() (time.Time, bool) {
	info, err := os.Stat(filepath.Join(StateDir(), storeMaintainSentinel))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func touchSentinel(now time.Time) {
	dir := StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	path := filepath.Join(dir, storeMaintainSentinel)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_ = file.Close()
	_ = os.Chtimes(path, now, now)
}

func maintenanceService() *stateapplication.MaintenanceService {
	return stateapplication.NewMaintenanceService(stateapplication.MaintenanceDependencies{
		KnownRoots:       knownStoreRoots,
		AllRoots:         allStoreRoots,
		StoreExists:      storeExists,
		MaintainStore:    maintainStore,
		SentinelModified: sentinelModified,
		TouchSentinel:    touchSentinel,
	})
}

func StateMaintain() (statecontract.StateMaintainResult, error) {
	return maintenanceService().Maintain()
}

func MaybeMaintainStateStores(minInterval time.Duration) (statecontract.StateMaintainResult, bool, error) {
	return maintenanceService().MaybeMaintain(minInterval)
}
