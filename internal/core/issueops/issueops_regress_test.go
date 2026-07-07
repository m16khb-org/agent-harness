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
	rec.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"gold-plating"}, RecordedAt: "2026-06-29T00:00:00Z", IssueReflectedAt: "2026-06-29T00:02:00Z"}
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
	if out.DevilsAdvocateReview != nil {
		t.Fatalf("regression must clear the devil's-advocate review so the gate re-fires: %#v", out.DevilsAdvocateReview)
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

func TestRegressRefusedForImplementPhaseParentWithChildren(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	if _, err := StartIssueOpsChild(stateRoot, IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-regress-implement",
		Title:              "regress implement child",
		TaskScope:          "prove implement parent still cannot regress",
		AcceptanceCriteria: []string{"existing phase precondition wins"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegressIssueOpsForReplan(stateRoot, parent.ID, "should still be refused from implement"); err == nil || !strings.Contains(err.Error(), "only applies from plan or compatibility-review") {
		t.Fatalf("implement parent should be refused by existing phase precondition, got %v", err)
	}
}

func TestRegressIssueOpsForReplanBlockedByActiveChildren(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := StartIssueOpsChild(stateRoot, IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-regress-active",
		Title:              "regress active child",
		TaskScope:          "prove active child blocks parent regress",
		AcceptanceCriteria: []string{"parent resolves child before replan"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.Phase = IssueOpsPhasePlan
	rec.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"scope drift"}, RecordedAt: "2026-07-07T00:00:00Z", IssueReflectedAt: "2026-07-07T00:01:00Z"}
	writeIssueOpsRecordForDelegationTest(t, stateRoot, rec)

	if _, err := RegressIssueOpsForReplan(stateRoot, parent.ID, "active child still owns delegated work"); err == nil || !strings.Contains(err.Error(), "children_active") {
		t.Fatalf("active children should block parent regress, got %v", err)
	}
	if _, err := DropIssueOpsChild(stateRoot, parent.ID, started.Child.ID, "delegated work abandoned before replan"); err != nil {
		t.Fatal(err)
	}
	out, err := RegressIssueOpsForReplan(stateRoot, parent.ID, "active children resolved before replan")
	if err != nil {
		t.Fatalf("regress should pass after child is dropped: %v", err)
	}
	if out.Phase != IssueOpsPhaseGrill {
		t.Fatalf("regress should still rewind to grill after child resolution, got %s", out.Phase)
	}
}

func TestRegressIssueOpsForReplanRequiresReflectedStop(t *testing.T) {
	// No devil's-advocate stop verdict → rejected.
	stateRoot, id := recordAtPhaseForRegressTest(t, IssueOpsPhasePlan)
	rec, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	rec.DevilsAdvocateReview = nil
	if _, err := touchAndWriteIssueOps(stateRoot, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := RegressIssueOpsForReplan(stateRoot, id, "reason"); err == nil || !strings.Contains(err.Error(), "stop verdict") {
		t.Fatalf("regress without a stop verdict must be rejected, got %v", err)
	}

	// Stop verdict recorded but findings not yet reflected to the issue → rejected.
	stateRoot2, id2 := recordAtPhaseForRegressTest(t, IssueOpsPhasePlan)
	rec2, err := ReadIssueOps(stateRoot2, id2)
	if err != nil {
		t.Fatal(err)
	}
	rec2.DevilsAdvocateReview.IssueReflectedAt = ""
	if _, err := touchAndWriteIssueOps(stateRoot2, rec2); err != nil {
		t.Fatal(err)
	}
	if _, err := RegressIssueOpsForReplan(stateRoot2, id2, "reason"); err == nil || !strings.Contains(err.Error(), "reflect the devil's-advocate findings") {
		t.Fatalf("regress before reflecting findings must be rejected, got %v", err)
	}
}
