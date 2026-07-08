package state

import (
	"os"
	"path/filepath"
	"time"

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

const storeMaintainSentinel = ".last-store-maintain"

// MaybeMaintainStateStores runs StateMaintain at most once per minInterval,
// gated by a stat-only sentinel file's mtime in the state root. This mirrors
// the MaybeDetectStuckWorkerJobs amortization pattern: maintenance is cheap
// (checkpoint + chmod) but unnecessary on every session start, so a sentinel
// keeps it to at most once per interval. Returns ran=false when skipped.
// Best-effort: the sentinel is touched even on error so a transient failure
// cannot make every session re-run maintenance.
func MaybeMaintainStateStores(minInterval time.Duration) (StateMaintainResult, bool, error) {
	dir := StateDir()
	sentinel := filepath.Join(dir, storeMaintainSentinel)
	if info, statErr := os.Stat(sentinel); statErr == nil && time.Since(info.ModTime()) < minInterval {
		return StateMaintainResult{OK: true, Roots: []sqlstore.MaintainResult{}, Skipped: knownStoreRoots()}, false, nil
	}
	result, err := StateMaintain()
	if mkErr := os.MkdirAll(dir, 0o700); mkErr == nil {
		if f, oErr := os.OpenFile(sentinel, os.O_CREATE|os.O_WRONLY, 0o600); oErr == nil {
			_ = f.Close()
		}
		now := time.Now()
		_ = os.Chtimes(sentinel, now, now)
	}
	return result, true, err
}
