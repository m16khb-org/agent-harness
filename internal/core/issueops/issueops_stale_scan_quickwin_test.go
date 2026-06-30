package issueops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPruneDoneCyclesPreservesLockFile guards Fix #1: pruneDoneCycles must
// remove ONLY the aged done-cycle .json, never the .lock. Deleting a live .lock
// between lock/unlock cycles splits the flock inode and breaks mutual exclusion
// (see issueops_lock_unix.go). The orphan-lock sweep reclaims the lock on a
// later run once the .json is gone.
//
// Before Fix #1 this test fails (pruneDoneCycles also did os.Remove(lockPath));
// after the fix it passes.
func TestPruneDoneCyclesPreservesLockFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()

	// Aged done cycle (~33 days old) that is past the retention threshold.
	id := NewIssueOpsID(repo, "quickwin-prune")
	oldTime := time.Now().Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(stateRoot, IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    "quickwin-prune",
		Phase:     IssueOpsPhaseDone,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}); err != nil {
		t.Fatal(err)
	}

	// Pre-create the lock file as a concurrent contender would.
	lockPath := filepath.Join(stateRoot, id+".lock")
	if err := os.WriteFile(lockPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	result := &IssueOpsStaleScanResult{OK: true, Repo: repo, Applied: true}
	pruneDoneCycles(repo, 720*time.Hour, result)

	if result.PrunedDone != 1 {
		t.Fatalf("expected 1 pruned done cycle, got %d (errors=%v)", result.PrunedDone, result.Errors)
	}

	// The .json must be removed.
	jsonPath := filepath.Join(stateRoot, id+".json")
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Fatalf("pruned done cycle .json should be removed, got stat err=%v", err)
	}

	// The .lock must NOT be removed (inode-preservation invariant).
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("pruneDoneCycles must leave the .lock in place (only the .json is pruned), got stat err=%v", err)
	}
}

// TestScanStaleApplySweepsOrphanLockWithoutReleases guards Fix #14: the
// orphan-.lock sweep in issueOpsGitWorktreeCleanup must run whenever --apply is
// set, independent of whether any cycle was released this pass. With zero
// releasable cycles and an orphan io-*.lock whose .json is absent, the orphan
// lock must still be swept.
//
// Before Fix #14 the sweep only ran inside `if req.Apply && len(result.Released)
// > 0`, so with zero releases the orphan lock survived and this test fails;
// after the fix it passes.
func TestScanStaleApplySweepsOrphanLockWithoutReleases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()

	// Orphan lock with no matching .json and no cycles at all → zero releases.
	orphanLockPath := filepath.Join(stateRoot, "io-000000000099.lock")
	if err := os.WriteFile(orphanLockPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: true})

	if len(res.Released) != 0 {
		t.Fatalf("expected zero releasable cycles, got released=%v", res.Released)
	}

	// The orphan lock must be swept even though nothing was released.
	if _, err := os.Stat(orphanLockPath); !os.IsNotExist(err) {
		t.Fatalf("orphan lock must be swept when --apply is set with zero releases, got stat err=%v", err)
	}
}
