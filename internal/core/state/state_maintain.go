package state

import (
	"fmt"
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

// knownStoreRoots returns fixed store directories the harness owns. Project
// stores are discovered separately so lifecycle-only namespaces are not
// reported as skipped or materialized by maintenance.
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
		filepath.Join(base, "issueops_v1"),
		workerRoot,
		filepath.Join(base, "loop"),
	}
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

// StateMaintain runs sqlstore maintenance (WAL truncate + permission repair)
// on every known store root that already has a database. Roots without a
// store are skipped, never created — maintenance must not materialize state.
func StateMaintain() (StateMaintainResult, error) {
	result := StateMaintainResult{Roots: []sqlstore.MaintainResult{}}
	roots := knownStoreRoots()
	projectRoots, err := projectStoreRoots()
	if err != nil {
		return result, err
	}
	roots = append(roots, projectRoots...)
	for _, dir := range roots {
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
