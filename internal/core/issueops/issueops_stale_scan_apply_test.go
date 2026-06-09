package issueops

import (
	"path/filepath"
	"testing"
	"time"
)

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
