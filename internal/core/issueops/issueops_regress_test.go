package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

// recordAtPhaseForRegressTest persists a started cycle at the given phase with a
// completed plan-phase ledger entry, so a Brooks regression has something to
// invalidate.
func recordAtPhaseForRegressTest(t *testing.T, phase IssueOpsPhase) (string, string) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-brooks"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	rec.Phase = phase
	rec.DesignReview = &model.IssueOpsDesignReview{ProblemSummary: "s", ProposedDesign: "d", Verification: []string{"v"}, Approved: true, ReviewedAt: "2026-06-29T00:00:00Z"}
	rec.PlanPath = "/repo/plans/x.md"
	rec.PhaseLedger = IssueOpsPhaseLedger{
		IssueOpsPhasePlan: IssueOpsPhaseLedgerEntry{Phase: IssueOpsPhasePlan, CompletedAt: "2026-06-29T00:01:00Z", Artifacts: []string{"plan_path"}},
	}
	if _, err := touchAndWriteIssueOps(stateRoot, rec); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	return stateRoot, rec.ID
}

func TestRegressIssueOpsForReplanFromPlan(t *testing.T) {
	stateRoot, id := recordAtPhaseForRegressTest(t, IssueOpsPhasePlan)

	if _, err := RegressIssueOpsForReplan(stateRoot, id, "  "); err == nil {
		t.Fatal("empty regression reason must be rejected")
	}

	out, err := RegressIssueOpsForReplan(stateRoot, id, "second-system effect: three cache backends for one need")
	if err != nil {
		t.Fatalf("regress: %v", err)
	}
	if out.Phase != IssueOpsPhaseGrill {
		t.Fatalf("brooks stop must regress to grill, got %s", out.Phase)
	}
	if out.DesignReview == nil || out.DesignReview.Approved {
		t.Fatalf("regression must clear design approval to force re-plan: %#v", out.DesignReview)
	}
	// audit: a scope decision captures the brooks stop reason
	found := false
	for _, d := range out.Decisions {
		if d.Kind == "scope" && strings.Contains(d.Body, "second-system effect") {
			found = true
		}
	}
	if !found {
		t.Fatalf("regression must record a scope decision with the stop reason: %#v", out.Decisions)
	}
	// rule 12: downstream plan ledger entry retained but marked stale
	planEntry, ok := out.PhaseLedger[IssueOpsPhasePlan]
	if !ok || planEntry.CompletedAt != "" {
		t.Fatalf("plan ledger entry should be marked incomplete (stale) after regression: %#v", planEntry)
	}
	staleNoted := false
	for _, n := range planEntry.Notes {
		if strings.Contains(n, "stale") {
			staleNoted = true
		}
	}
	if !staleNoted {
		t.Fatalf("plan ledger entry should carry a stale note: %#v", planEntry.Notes)
	}
}

func TestRegressIssueOpsForReplanRejectedOutsidePlanCompat(t *testing.T) {
	stateRoot, id := recordAtPhaseForRegressTest(t, IssueOpsPhaseProblem)
	if _, err := RegressIssueOpsForReplan(stateRoot, id, "too early"); err == nil {
		t.Fatal("regression from problem phase must be rejected")
	}
}
