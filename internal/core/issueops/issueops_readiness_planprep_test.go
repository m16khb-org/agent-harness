package issueops

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func baseIntentRecord(class string) IssueOpsRecord {
	return IssueOpsRecord{
		IssueURL: "https://github.com/o/r/issues/1",
		Intent: &model.IssueOpsIntentContract{
			RawRequest:        "raw user ask",
			InterpretedIntent: "agent reframed interpretation that differs",
			SuccessCriteria:   []string{"gate works"},
			IntentClass:       class,
		},
	}
}

func planPrepHasMissing(missing []string, key string) bool {
	for _, m := range missing {
		if m == key {
			return true
		}
	}
	return false
}

func TestPlanReadinessRequiresPlanPrepForNonTrivial(t *testing.T) {
	ready := IssueOpsPlanReadiness(baseIntentRecord("standard"))
	for _, key := range []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research"} {
		if !planPrepHasMissing(ready.Missing, key) {
			t.Fatalf("standard cycle without plan_prep should miss %s: %#v", key, ready.Missing)
		}
	}
	if ready.Ready {
		t.Fatal("standard cycle without plan_prep should not be ready")
	}
}

func TestPlanReadinessSkipsPlanPrepForTrivial(t *testing.T) {
	ready := IssueOpsPlanReadiness(baseIntentRecord("trivial"))
	if !ready.Ready {
		t.Fatalf("trivial cycle should be ready without plan_prep: %#v", ready.Missing)
	}
}

func TestPlanReadinessAcceptsEvidenceAndWaive(t *testing.T) {
	rec := baseIntentRecord("standard")
	rec.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{".agent-harness/ADR.md#gate"}},
		RelatedIssues:  model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"remote score: selected=#12(0.81), threshold=0.70"}},
		WebResearch:    model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "순수 내부 리팩토링이라 외부 근거 불필요"},
	}
	ready := IssueOpsPlanReadiness(rec)
	if !ready.Ready {
		t.Fatalf("evidence+waive should satisfy plan-prep gate: %#v", ready.Missing)
	}
}

func TestPlanReadinessRejectsEmptyStatusItem(t *testing.T) {
	rec := baseIntentRecord("standard")
	rec.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"adr"}},
		RelatedIssues:  model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"},
		WebResearch:    model.IssueOpsPlanPrepItem{Status: "evidence"}, // evidence missing
	}
	ready := IssueOpsPlanReadiness(rec)
	if !planPrepHasMissing(ready.Missing, "plan_prep_web_research") {
		t.Fatalf("web research with empty evidence must be missing: %#v", ready.Missing)
	}
	if planPrepHasMissing(ready.Missing, "plan_prep_decisions") || planPrepHasMissing(ready.Missing, "plan_prep_related_issues") {
		t.Fatalf("valid items must not be missing: %#v", ready.Missing)
	}
}
