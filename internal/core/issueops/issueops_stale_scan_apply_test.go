package issueops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWithIssueOpsLockPreservesLockFile verifies that withIssueOpsLock does NOT
// remove the .lock file after the critical section. The lock file must persist
// because flock locks are inode-based — deleting and recreating the file creates
// a new inode, breaking mutual exclusion across contenders. Orphaned lock files
// (with no matching .json) are cleaned by the off-hot-path stale scan instead.
func TestWithIssueOpsLockPreservesLockFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()
	id := NewIssueOpsID(repo, "lock-test")

	lockPath := filepath.Join(stateRoot, id+".lock")

	// Lock file must not exist before.
	if _, err := os.Stat(lockPath); err == nil {
		t.Fatalf("lock file should not exist before lock: %s", lockPath)
	}

	err := withIssueOpsLock(stateRoot, id, func() error {
		// While locked, the lock file must exist.
		if _, err := os.Stat(lockPath); err != nil {
			t.Errorf("lock file should exist while locked: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// After unlock, the lock file must still exist (inode must not be deleted).
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file must persist after unlock to preserve inode-based mutual exclusion, got stat err=%v", err)
	}
}

// TestWithIssueOpsLockPreservesLockFileOnError verifies that the lock file
// persists even when the critical section returns an error.
func TestWithIssueOpsLockPreservesLockFileOnError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()
	id := NewIssueOpsID(repo, "lock-err-test")

	lockPath := filepath.Join(stateRoot, id+".lock")

	_ = withIssueOpsLock(stateRoot, id, func() error {
		return os.ErrNotExist // any non-nil error
	})

	// After unlock (even with error), the lock file must still exist.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file must persist after unlock on error to preserve inode-based mutual exclusion, got stat err=%v", err)
	}
}

// TestIssueOpsGitWorktreeCleanupRemovesOrphanLockFiles verifies that orphaned
// .lock files (with no matching .json cycle file) are cleaned up during the
// worktree cleanup pass.
func TestIssueOpsGitWorktreeCleanupRemovesOrphanLockFiles(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()

	// Create an orphaned .lock file with no matching .json
	orphanLockID := "io-000000000001"
	orphanLockPath := filepath.Join(stateRoot, orphanLockID+".lock")
	if err := os.WriteFile(orphanLockPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Also create a legitimate .lock + .json pair (should not be touched).
	pairedID := "io-000000000002"
	pairedJSON := filepath.Join(stateRoot, pairedID+".json")
	pairedLock := filepath.Join(stateRoot, pairedID+".lock")
	if err := os.WriteFile(pairedJSON, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pairedLock, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Also create a non-cycle .json that is not an issueops record (wrong prefix).
	// Just to be sure the lock cleanup doesn't crash on random files.

	result := &IssueOpsStaleScanResult{OK: true, Repo: repo, Applied: true}
	issueOpsGitWorktreeCleanup(repo, result)

	// Orphan lock must be gone.
	if _, err := os.Stat(orphanLockPath); !os.IsNotExist(err) {
		t.Fatalf("orphan lock file should be removed, got stat err=%v", err)
	}

	// Paired lock must still exist (it has a matching .json).
	if _, err := os.Stat(pairedLock); err != nil {
		t.Fatalf("paired lock file should still exist, got stat err=%v", err)
	}

	// Paired JSON must still exist.
	if _, err := os.Stat(pairedJSON); err != nil {
		t.Fatalf("paired JSON file should still exist, got stat err=%v", err)
	}
}

// TestScanStalePruneDoneCycles verifies that done cycles older than PruneDoneAge
// are pruned (JSON + lock files deleted) when --apply is set.
func TestScanStalePruneDoneCycles(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	// Create a done cycle with an old timestamp.
	oldID := NewIssueOpsID(repo, "prune-old")
	oldTime := time.Now().Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano) // ~33 days ago
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:        true,
		ID:        oldID,
		Repo:      repo,
		Branch:    "prune-old",
		Phase:     IssueOpsPhaseDone,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}); err != nil {
		t.Fatal(err)
	}

	// Create a done cycle with a recent timestamp (should not be pruned).
	recentID := NewIssueOpsID(repo, "prune-recent")
	recentTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:        true,
		ID:        recentID,
		Repo:      repo,
		Branch:    "prune-recent",
		Phase:     IssueOpsPhaseDone,
		CreatedAt: recentTime,
		UpdatedAt: recentTime,
	}); err != nil {
		t.Fatal(err)
	}

	// Create a non-done cycle (should not be touched by prune).
	liveID := NewIssueOpsID(repo, "prune-live")
	liveTime := time.Now().Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:        true,
		ID:        liveID,
		Repo:      repo,
		Branch:    "prune-live",
		Phase:     IssueOpsPhasePlan,
		CreatedAt: liveTime,
		UpdatedAt: liveTime,
	}); err != nil {
		t.Fatal(err)
	}

	stateRoot := IssueOpsStateRoot()

	// Pre-create lock files for the old done cycle so we can verify they're removed.
	oldLockPath := filepath.Join(stateRoot, oldID+".lock")
	if err := os.WriteFile(oldLockPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{
		Repo:         repo,
		Apply:        true,
		PruneDoneAge: 720 * time.Hour, // 30 days
	})

	if res.PrunedDone != 1 {
		t.Fatalf("expected 1 pruned done cycle, got %d (errors=%v)", res.PrunedDone, res.Errors)
	}

	// Old done cycle JSON must be gone.
	oldJSON := filepath.Join(stateRoot, oldID+".json")
	if _, err := os.Stat(oldJSON); !os.IsNotExist(err) {
		t.Fatalf("old done cycle JSON should be removed, got stat err=%v", err)
	}

	// Old lock file must be gone.
	if _, err := os.Stat(oldLockPath); !os.IsNotExist(err) {
		t.Fatalf("old lock file should be removed, got stat err=%v", err)
	}

	// Recent done cycle must still exist.
	recentJSON := filepath.Join(stateRoot, recentID+".json")
	if _, err := os.Stat(recentJSON); err != nil {
		t.Fatalf("recent done cycle must still exist, got stat err=%v", err)
	}

	// Live non-done cycle must still exist.
	liveJSON := filepath.Join(stateRoot, liveID+".json")
	if _, err := os.Stat(liveJSON); err != nil {
		t.Fatalf("live cycle must still exist, got stat err=%v", err)
	}
}

// TestScanStalePruneDoneRequiresApply verifies that --prune-done has no effect
// when --apply is not set (dry-run mode).
func TestScanStalePruneDoneRequiresApply(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	oldID := NewIssueOpsID(repo, "prune-dry")
	oldTime := time.Now().Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:        true,
		ID:        oldID,
		Repo:      repo,
		Branch:    "prune-dry",
		Phase:     IssueOpsPhaseDone,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}); err != nil {
		t.Fatal(err)
	}

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{
		Repo:         repo,
		Apply:        false, // dry run
		PruneDoneAge: 720 * time.Hour,
	})

	if res.PrunedDone != 0 {
		t.Fatalf("dry-run should not prune, got %d", res.PrunedDone)
	}

	// Old done cycle must still exist.
	stateRoot := IssueOpsStateRoot()
	oldJSON := filepath.Join(stateRoot, oldID+".json")
	if _, err := os.Stat(oldJSON); err != nil {
		t.Fatalf("dry-run must not delete done cycle, got stat err=%v", err)
	}
}

// TestParseIssueOpsTime validates the time parsing helper.
func TestParseIssueOpsTime(t *testing.T) {
	ref := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input string
		zero  bool
	}{
		{"nano", ref.Format(time.RFC3339Nano), false},
		{"second", ref.Format(time.RFC3339), false},
		{"empty", "", true},
		{"garbage", "not-a-time", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := parseIssueOpsTime(tt.input)
			if tt.zero && !ts.IsZero() {
				t.Fatalf("expected zero time for %q, got %v", tt.input, ts)
			}
			if !tt.zero && ts.IsZero() {
				t.Fatalf("expected non-zero time for %q", tt.input)
			}
		})
	}
}

// TestScanStaleResultIncludesPrunedDoneField verifies that PrunedDoneAge is
// wired through and the result struct carries the PrunedDone field.
func TestScanStaleResultIncludesPrunedDoneField(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{
		Repo:         repo,
		Apply:        true,
		PruneDoneAge: 1 * time.Hour,
	})
	// No done cycles exist, so PrunedDone must be 0.
	if res.PrunedDone != 0 {
		t.Fatalf("expected PrunedDone=0 for empty state, got %d", res.PrunedDone)
	}
}

// TestScanStaleApplyReleasesConfirmedStaleAfterReprobe guards SCENARIO 4's fix:
// the --apply path re-reads and re-classifies each releasable finding immediately
// before force-releasing. A genuinely stale cycle (implement phase, worktree
// directory deleted) must still be released after that re-probe (no regression),
// and the record must end in the done phase.
func TestScanStaleApplyReleasesConfirmedStaleAfterReprobe(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	branch := "1-stale"
	id := NewIssueOpsID(repo, branch)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:           true,
		ID:           id,
		Repo:         repo,
		Branch:       branch,
		Phase:        IssueOpsPhaseImplement,
		WorktreePath: filepath.Join(repo, "..", "deleted.worktrees", branch), // never created
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatal(err)
	}

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: true})
	if !res.OK {
		t.Fatalf("scan should succeed, got %+v", res)
	}
	if len(res.Released) != 1 || res.Released[0] != id {
		t.Fatalf("confirmed-stale cycle should be released after re-probe, got released=%v findings=%+v errors=%v", res.Released, res.Findings, res.Errors)
	}
	after, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Phase != IssueOpsPhaseDone {
		t.Fatalf("released cycle must be in done phase, got %q", after.Phase)
	}
}

// TestScanStaleApplyDoesNotReleaseLiveCycle guards that a non-stale cycle (no
// worktree expected, not aged out) is never force-released by --apply. The
// re-probe must not widen releasing to live work.
func TestScanStaleApplyDoesNotReleaseLiveCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	branch := "2-live"
	id := NewIssueOpsID(repo, branch)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    branch,
		Phase:     IssueOpsPhasePlan, // does not expect a worktree
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: true})
	if len(res.Released) != 0 {
		t.Fatalf("live plan-phase cycle must not be released, got released=%v", res.Released)
	}
	after, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Phase != IssueOpsPhasePlan {
		t.Fatalf("live cycle phase must be untouched, got %q", after.Phase)
	}
}
