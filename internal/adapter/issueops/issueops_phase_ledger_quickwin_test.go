package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// IssueOpsStatus must backfill phases missing from a PARTIAL persisted ledger
// (e.g. a multi-phase forward jump that stamped only its endpoints), not only
// when the ledger is entirely empty — while preserving the real persisted
// entries rather than overwriting them with derived ones.
func TestIssueOpsStatusBackfillsPartialLedger(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-partial"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Persist a partial ledger: problem + plan stamped, grill absent.
	rec.Phase = IssueOpsPhasePlan
	rec.PhaseLedger = issueops.IssueOpsPhaseLedger{
		IssueOpsPhaseProblem: {Phase: IssueOpsPhaseProblem, EnteredAt: "2026-06-29T00:00:00Z", CompletedAt: "2026-06-29T00:01:00Z"},
		IssueOpsPhasePlan:    {Phase: IssueOpsPhasePlan, EnteredAt: "2026-06-29T00:02:00Z"},
	}
	if _, err := touchAndWriteIssueOps(stateRoot, rec); err != nil {
		t.Fatalf("write: %v", err)
	}

	status, err := readIssueOpsStatusForTest(stateRoot, rec.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, phase := range issueops.IssueOpsPhases {
		if _, ok := status.PhaseLedger[phase]; !ok {
			t.Fatalf("partial ledger must be backfilled; missing %s: %#v", phase, status.PhaseLedger)
		}
	}
	// The real persisted entry must win over the derived one.
	if got := status.PhaseLedger[IssueOpsPhaseProblem].CompletedAt; got != "2026-06-29T00:01:00Z" {
		t.Fatalf("persisted problem entry must be preserved, got CompletedAt=%q", got)
	}
}

// A forward transition that re-completes a previously-regressed phase must clear
// the stale-regression note so status no longer shows the phase as stale forever.
func TestStampForwardTransitionClearsStaleNote(t *testing.T) {
	ledger := markIssueOpsLedgerStale(issueops.IssueOpsPhaseLedger{}, "second-system effect", IssueOpsPhasePlan)
	if len(ledger[IssueOpsPhasePlan].Notes) == 0 {
		t.Fatal("precondition: plan must carry a stale note before re-completion")
	}

	ledger = stampIssueOpsForwardTransition(ledger, IssueOpsPhasePlan, IssueOpsPhaseCompatibilityReview, "2026-06-30T00:00:00Z")
	plan := ledger[IssueOpsPhasePlan]
	if plan.CompletedAt == "" {
		t.Fatalf("plan must be marked complete after the forward transition, got %#v", plan)
	}
	for _, n := range plan.Notes {
		if strings.HasPrefix(n, "stale:") {
			t.Fatalf("stale note must be cleared on legitimate re-completion, got notes=%v", plan.Notes)
		}
	}
}
