package state

import (
	"os"
	"path/filepath"

	"agent-harness/internal/core/sqlstore"
)

// StateMaintainResult reports one maintenance pass over the known store roots.
type StateMaintainResult struct {
	OK      bool                      `json:"ok"`
	Roots   []sqlstore.MaintainResult `json:"roots"`
	Skipped []string                  `json:"skipped,omitempty"`
}

// knownStoreRoots returns the store directories the harness owns: the state
// KV root plus the issueops, workpool, and worker subsystems. The worker root
// honors HARNESS_WORKER_DIR exactly like the worker package does.
func knownStoreRoots() []string {
	base := StateDir()
	workerRoot := filepath.Join(base, "worker")
	if dir := os.Getenv("HARNESS_WORKER_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			workerRoot = abs
		}
	}
	return []string{
		base,
		filepath.Join(base, "issueops"),
		filepath.Join(base, "workpool"),
		workerRoot,
	}
}

// StateMaintain runs sqlstore maintenance (WAL truncate + permission repair)
// on every known store root that already has a database. Roots without a
// store are skipped, never created — maintenance must not materialize state.
func StateMaintain() (StateMaintainResult, error) {
	result := StateMaintainResult{Roots: []sqlstore.MaintainResult{}}
	for _, dir := range knownStoreRoots() {
		if _, err := os.Stat(filepath.Join(dir, "harness.db")); err != nil {
			result.Skipped = append(result.Skipped, dir)
			continue
		}
		db, err := sqlstore.Open(dir)
		if err != nil {
			return result, err
		}
		maintained, err := db.Maintain()
		if err != nil {
			return result, err
		}
		result.Roots = append(result.Roots, maintained)
	}
	result.OK = true
	return result, nil
}
