package issueops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPruneDoneCyclesPreservesSpanLockDB guards the sqlite-era analogue of the
// old lock-preservation invariant: pruneDoneCycles must delete ONLY the aged
// done-cycle record, never the state root's span-lock database that concurrent
// contenders are locking on.
func TestPruneDoneCyclesPreservesSpanLockDB(t *testing.T) {
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

	// Touch the span lock once so the lock database exists, as a concurrent
	// contender would have left it.
	if err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	result := &IssueOpsStaleScanResult{OK: true, Repo: repo, Applied: true}
	pruneDoneCycles(repo, 720*time.Hour, result)

	if result.PrunedDone != 1 {
		t.Fatalf("expected 1 pruned done cycle, got %d (errors=%v)", result.PrunedDone, result.Errors)
	}

	// The record must be removed.
	if issueOpsRecordExists(t, stateRoot, id) {
		t.Fatalf("pruned done cycle record should be removed")
	}

	// The span lock database must NOT be removed.
	if _, err := os.Stat(filepath.Join(stateRoot, "harness.lock.db")); err != nil {
		t.Fatalf("pruneDoneCycles must leave the span lock db in place, got stat err=%v", err)
	}
}
