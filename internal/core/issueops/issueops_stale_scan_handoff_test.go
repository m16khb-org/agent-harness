package issueops

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/stalescan"
)

// TestStaleScanSurfacesHandoffInconsistencyAndNeverReleases proves Task F3
// detection end-to-end: a done-phase cycle with a non-terminal supervised
// handoff (the #2581 shape) is surfaced by ScanStaleIssueOpsCycles even though
// the non-done scan skips it, and --apply neither force-releases nor prunes it.
func TestStaleScanSurfacesHandoffInconsistencyAndNeverReleases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	repo := t.TempDir()
	worker := filepath.Clean(repo) + ".worktrees/1-demo"

	seed := IssueOpsRecord{ID: "io-9bab890c4d4f", Phase: IssueOpsPhaseDone, Repo: filepath.Clean(repo), Branch: "1-demo"}
	seed.ExecutionHandoff = validStrandedHandoff(filepath.Clean(repo), worker)
	seed.UpdatedAt = "2020-01-01T00:00:00Z" // old enough to trip any age/prune threshold
	if _, err := WriteIssueOps(stateRoot, seed); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	result := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{
		Repo:         filepath.Clean(repo),
		Apply:        true,
		MaxAge:       time.Hour,
		PruneDoneAge: time.Nanosecond, // would prune the done record if not guarded
	})

	var found *stalescan.Finding
	for i := range result.Findings {
		if result.Findings[i].Category == stalescan.CategoryHandoffInconsistent {
			found = &result.Findings[i]
		}
	}
	if found == nil {
		t.Fatalf("scan did not surface the handoff inconsistency: %#v", result.Findings)
	}
	if found.Releasable {
		t.Fatal("inconsistency finding must not be releasable")
	}
	if !strings.Contains(strings.Join(found.Reasons, ","), "handoff_nonterminal_on_terminal_phase") {
		t.Fatalf("finding missing the signal reason: %#v", found.Reasons)
	}

	// --apply must not have released or pruned the stranded record.
	for _, id := range result.Released {
		if id == seed.ID {
			t.Fatal("--apply force-released the stranded record (must be report-only)")
		}
	}
	if result.PrunedDone != 0 {
		t.Fatalf("--apply pruned a done record with a non-terminal handoff: %d", result.PrunedDone)
	}
	if !issueOpsRecordExists(t, stateRoot, seed.ID) {
		t.Fatal("stranded record must survive --apply (timeout != absence)")
	}
}
