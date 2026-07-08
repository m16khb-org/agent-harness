package issueops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWithIssueOpsLockSpanDBPersists verifies that withIssueOpsLock creates the
// state root's span-lock database and does NOT remove it after the critical
// section, so later contenders keep locking the same database.
func TestWithIssueOpsLockSpanDBPersists(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()
	id := NewIssueOpsID(repo, "lock-test")

	lockDB := filepath.Join(stateRoot, "harness.lock.db")

	err := withIssueOpsLock(stateRoot, id, func() error {
		// While locked, the span lock database must exist.
		if _, err := os.Stat(lockDB); err != nil {
			t.Errorf("span lock db should exist while locked: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// After unlock, the span lock database must still exist.
	if _, err := os.Stat(lockDB); err != nil {
		t.Fatalf("span lock db must persist after unlock, got stat err=%v", err)
	}
}

// TestWithIssueOpsLockSpanDBPersistsOnError verifies that the span lock
// database persists even when the critical section returns an error.
func TestWithIssueOpsLockSpanDBPersistsOnError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()
	id := NewIssueOpsID(repo, "lock-err-test")

	_ = withIssueOpsLock(stateRoot, id, func() error {
		return os.ErrNotExist // any non-nil error
	})

	if _, err := os.Stat(filepath.Join(stateRoot, "harness.lock.db")); err != nil {
		t.Fatalf("span lock db must persist after unlock on error, got stat err=%v", err)
	}
}

// issueOpsRecordExists reports whether a cycle record exists in the store.
func issueOpsRecordExists(t *testing.T, stateRoot, id string) bool {
	t.Helper()
	_, err := ReadIssueOps(stateRoot, id)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	t.Fatalf("read %s: %v", id, err)
	return false
}

// TestScanStalePruneDoneCycles verifies that done cycles older than PruneDoneAge
// are pruned (records deleted) when --apply is set.
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

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{
		Repo:         repo,
		Apply:        true,
		PruneDoneAge: 720 * time.Hour, // 30 days
	})

	if res.PrunedDone != 1 {
		t.Fatalf("expected 1 pruned done cycle, got %d (errors=%v)", res.PrunedDone, res.Errors)
	}

	// Old done cycle record must be gone.
	if issueOpsRecordExists(t, stateRoot, oldID) {
		t.Fatalf("old done cycle record should be removed")
	}

	// Recent done cycle must still exist.
	if !issueOpsRecordExists(t, stateRoot, recentID) {
		t.Fatalf("recent done cycle must still exist")
	}

	// Live non-done cycle must still exist.
	if !issueOpsRecordExists(t, stateRoot, liveID) {
		t.Fatalf("live cycle must still exist")
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
	if !issueOpsRecordExists(t, IssueOpsStateRoot(), oldID) {
		t.Fatalf("dry-run must not delete done cycle")
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

// seedStaleBindingFixture creates three bindings for repo: a primary binding
// on a live plan-phase cycle, a scoped binding on a done cycle, and a scoped
// binding whose cycle record does not exist.
func seedStaleBindingFixture(t *testing.T, repo string) (liveID, doneID, ghostID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	liveID = NewIssueOpsID(repo, "1-live")
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK: true, ID: liveID, Repo: repo, Branch: "1-live",
		Phase: IssueOpsPhasePlan, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	doneID = NewIssueOpsID(repo, "2-done")
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK: true, ID: doneID, Repo: repo, Branch: "2-done",
		Phase: IssueOpsPhaseDone, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	ghostID = "io-eeeeeeeeeeee"
	if err := BindIssueOpsSession(repo, liveID, "1-live", ""); err != nil {
		t.Fatal(err)
	}
	if err := BindScopedIssueOpsSession(repo, doneID, "2-done", ""); err != nil {
		t.Fatal(err)
	}
	if err := BindScopedIssueOpsSession(repo, ghostID, "3-ghost", ""); err != nil {
		t.Fatal(err)
	}
	return liveID, doneID, ghostID
}

// TestScanStaleDryRunReportsBindingsWithoutDelete asserts the dry-run scan
// reports stale session bindings but deletes nothing.
func TestScanStaleDryRunReportsBindingsWithoutDelete(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	liveID, doneID, ghostID := seedStaleBindingFixture(t, repo)

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: false})
	if len(res.StaleBindings) != 2 {
		t.Fatalf("expected 2 stale bindings reported, got %+v", res.StaleBindings)
	}
	if res.PrunedBindings != 0 {
		t.Fatalf("dry-run must not delete bindings, got %d", res.PrunedBindings)
	}
	if b, err := ReadScopedIssueOpsSession(repo, doneID); err != nil || b.CycleID != doneID {
		t.Fatalf("dry-run must keep done-cycle binding, got %+v err=%v", b, err)
	}
	if b, err := ReadScopedIssueOpsSession(repo, ghostID); err != nil || b.CycleID != ghostID {
		t.Fatalf("dry-run must keep ghost binding, got %+v err=%v", b, err)
	}
	if b, err := ReadIssueOpsSession(repo); err != nil || b.CycleID != liveID {
		t.Fatalf("live binding must survive, got %+v err=%v", b, err)
	}
}

// TestScanStaleApplyPrunesOrphanSessionBindings asserts --apply deletes
// bindings whose cycle is done or absent while preserving live bindings.
func TestScanStaleApplyPrunesOrphanSessionBindings(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	liveID, doneID, ghostID := seedStaleBindingFixture(t, repo)

	res := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: true})
	if res.PrunedBindings != 2 {
		t.Fatalf("expected 2 pruned bindings, got %+v (stale=%v errors=%v)", res.PrunedBindings, res.StaleBindings, res.Errors)
	}
	if b, err := ReadScopedIssueOpsSession(repo, doneID); err != nil || b.CycleID != "" {
		t.Fatalf("done-cycle binding must be pruned, got %+v err=%v", b, err)
	}
	if b, err := ReadScopedIssueOpsSession(repo, ghostID); err != nil || b.CycleID != "" {
		t.Fatalf("ghost binding must be pruned, got %+v err=%v", b, err)
	}
	if b, err := ReadIssueOpsSession(repo); err != nil || b.CycleID != liveID {
		t.Fatalf("live binding must survive apply, got %+v err=%v", b, err)
	}
	// Bindings for other repos must never be touched.
	otherRepo := t.TempDir()
	otherID := NewIssueOpsID(otherRepo, "9-other")
	nowOther := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := writeIssueOps(IssueOpsStateRoot(), IssueOpsRecord{
		OK: true, ID: otherID, Repo: otherRepo, Branch: "9-other",
		Phase: IssueOpsPhasePlan, CreatedAt: nowOther, UpdatedAt: nowOther,
	}); err != nil {
		t.Fatal(err)
	}
	if err := BindIssueOpsSession(otherRepo, otherID, "9-other", ""); err != nil {
		t.Fatal(err)
	}
	res = ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo, Apply: true})
	if b, err := ReadIssueOpsSession(otherRepo); err != nil || b.CycleID != otherID {
		t.Fatalf("other-repo binding must be untouched, got %+v err=%v", b, err)
	}
}
