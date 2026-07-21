package operationalhealth

import "testing"

func TestOwnershipCleanupPendingRetainsOperationalAuthority(t *testing.T) {
	cycle := Cycle{ID: "io-owner", Repo: "/tmp/repo", Branch: "1-owner", Phase: "done", HandoffState: "cleanup_pending_human_decision", WorktreePath: "/tmp/repo.worktrees/1-owner", OrcaWorktreeID: "wt-owner"}
	if got := EvaluateCycleAuthority(cycle, Options{}); got != AuthorityPreserved {
		t.Fatalf("cleanup-pending authority = %s", got)
	}
	cycle.OrcaWorktreeID = ""
	if got := EvaluateCycleAuthority(cycle, Options{}); got != AuthorityUnknown {
		t.Fatalf("incomplete cleanup-pending authority = %s", got)
	}
}
