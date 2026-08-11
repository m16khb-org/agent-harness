package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// A cycle can reach plan/compatibility-review with an EMPTY PhaseLedger: the
// linking and compatibility-review recorders record artifacts without stamping
// the ledger. A Brooks regression then persists only the two stale plan/compat
// entries. After the fix IssueOpsStatus backfills the phases missing from a
// partial persisted ledger, so status still shows every phase (problem..pr)
// without persisting a derived ledger.
func TestRegressIssueOpsForReplanStatusBackfillsAllPhases(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-empty-ledger"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Advance to plan with NO ledger stamped (mimics the linking /
	// compatibility-review paths that record artifacts without a ledger).
	rec.Phase = IssueOpsPhasePlan
	rec.DesignReview = &issueops.IssueOpsDesignReview{ProblemSummary: "s", ProposedDesign: "d", Verification: []string{"v"}, Approved: true, ReviewedAt: "2026-06-29T00:00:00Z"}
	rec.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"gold-plating"}, RecordedAt: "2026-06-29T00:00:00Z", IssueReflectedAt: "2026-06-29T00:02:00Z"}
	rec.PlanPath = "/repo/plans/x.md"
	rec.PhaseLedger = nil
	if _, err := touchAndWriteIssueOps(stateRoot, rec); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	if _, err := RegressIssueOpsForReplan(stateRoot, rec.ID, "second-system effect: re-plan"); err != nil {
		t.Fatalf("regress: %v", err)
	}

	status, err := readIssueOpsStatusForTest(stateRoot, rec.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	// Status must backfill the partial persisted ledger, so it carries an entry
	// for EVERY phase (problem..pr/done), not just the two stale ones.
	for _, phase := range issueops.IssueOpsPhases {
		if _, ok := status.PhaseLedger[phase]; !ok {
			t.Fatalf("ledger after regress must contain an entry for every phase; missing %s: %#v", phase, status.PhaseLedger)
		}
	}

	// plan + compatibility-review remain, but marked stale (incomplete + note).
	for _, phase := range []issueops.IssueOpsPhase{IssueOpsPhasePlan, IssueOpsPhaseCompatibilityReview} {
		entry, ok := status.PhaseLedger[phase]
		if !ok {
			t.Fatalf("missing %s entry after regression", phase)
		}
		if entry.CompletedAt != "" {
			t.Fatalf("%s should be marked incomplete (stale) after regression: %#v", phase, entry)
		}
		staleNoted := false
		for _, n := range entry.Notes {
			if strings.Contains(n, "stale") {
				staleNoted = true
			}
		}
		if !staleNoted {
			t.Fatalf("%s should carry a stale note after regression: %#v", phase, entry.Notes)
		}
	}
}
