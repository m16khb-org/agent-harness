package issueops

import (
	"reflect"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func fullIntentForLedger() *model.IssueOpsIntentContract {
	return &model.IssueOpsIntentContract{
		RawRequest:        "do x",
		InterpretedIntent: "implement x",
		SuccessCriteria:   []string{"x works"},
		IntentClass:       "standard",
		RecordedAt:        "2026-06-29T00:00:00Z",
	}
}

func waivedPlanPrep() *model.IssueOpsPlanPrep {
	item := model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a for this small change"}
	return &model.IssueOpsPlanPrep{PriorDecisions: item, RelatedIssues: item, WebResearch: item, RecordedAt: "2026-06-29T00:00:00Z"}
}

func TestIssueOpsProblemReadinessNeedsOnlyIntent(t *testing.T) {
	if r := IssueOpsProblemReadiness(IssueOpsRecord{}); r.Ready {
		t.Fatalf("empty record should not complete problem: %#v", r)
	} else if !containsLedgerKey(r.Missing, "intent_contract") {
		t.Fatalf("expected intent_contract missing, got %#v", r.Missing)
	}
	rec := IssueOpsRecord{Intent: fullIntentForLedger()}
	if r := IssueOpsProblemReadiness(rec); !r.Ready {
		t.Fatalf("intent-complete record should complete problem regardless of issue_url/branch: %#v", r)
	}
}

func TestIssueOpsGrillReadinessRequiresArtifacts(t *testing.T) {
	rec := IssueOpsRecord{Intent: fullIntentForLedger()}
	r := IssueOpsGrillReadiness(rec)
	if r.Ready {
		t.Fatalf("bare grill should not be ready: %#v", r)
	}
	for _, want := range []string{"issue_url", "branch", "split_decision", "domain_review"} {
		if !containsLedgerKey(r.Missing, want) {
			t.Fatalf("grill missing should include %q, got %#v", want, r.Missing)
		}
	}

	complete := IssueOpsRecord{
		Intent:    fullIntentForLedger(),
		IssueURL:  "https://example/issues/1",
		Branch:    "1-x",
		PlanPrep:  waivedPlanPrep(),
		Decisions: []model.IssueOpsDecision{{Title: "no split", Kind: "scope", CreatedAt: "2026-06-29T00:00:00Z"}},
		DomainReview: &model.IssueOpsDomainReview{
			ModelFit:   "fits",
			ReviewedAt: "2026-06-29T00:00:00Z",
		},
	}
	if r := IssueOpsGrillReadiness(complete); !r.Ready {
		t.Fatalf("fully populated grill should be ready, got missing %#v", r.Missing)
	}
}

func TestIssueOpsGrillSplitDecisionAcceptsChildLink(t *testing.T) {
	rec := IssueOpsRecord{
		Intent:       fullIntentForLedger(),
		IssueURL:     "https://example/issues/1",
		Branch:       "1-x",
		PlanPrep:     waivedPlanPrep(),
		IssueLinks:   []model.IssueOpsIssueLink{{Type: "child", URL: "https://example/issues/2", CreatedAt: "2026-06-29T00:00:00Z"}},
		DomainReview: &model.IssueOpsDomainReview{ModelFit: "fits", ReviewedAt: "2026-06-29T00:00:00Z"},
	}
	if r := IssueOpsGrillReadiness(rec); !r.Ready {
		t.Fatalf("child issue link should satisfy split_decision, missing %#v", r.Missing)
	}
}

func TestIssueOpsPhaseCompletionDispatches(t *testing.T) {
	rec := IssueOpsRecord{Intent: fullIntentForLedger()}
	if r := IssueOpsPhaseCompletion(rec, IssueOpsPhaseProblem); !r.Ready {
		t.Fatalf("problem completion should be ready for intent-only record: %#v", r)
	}
	if r := IssueOpsPhaseCompletion(rec, IssueOpsPhaseGrill); r.Ready {
		t.Fatalf("grill completion should not be ready for intent-only record")
	}
}

func TestDeriveIssueOpsPhaseLedgerIsDeterministicAndSentinel(t *testing.T) {
	rec := IssueOpsRecord{Phase: IssueOpsPhaseGrill, Intent: fullIntentForLedger()}
	a := DeriveIssueOpsPhaseLedger(rec)
	b := DeriveIssueOpsPhaseLedger(rec)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("derivation must be deterministic:\n a=%#v\n b=%#v", a, b)
	}
	problem, ok := a[IssueOpsPhaseProblem]
	if !ok || problem.Phase != IssueOpsPhaseProblem {
		t.Fatalf("expected a problem ledger entry keyed by its phase, got %#v", a)
	}
	for phase, entry := range a {
		if entry.Phase != phase {
			t.Fatalf("entry phase %q must equal map key %q", entry.Phase, phase)
		}
		if isWallClock(entry.EnteredAt) || isWallClock(entry.CompletedAt) {
			t.Fatalf("derived entry must use sentinel, not wall-clock: %#v", entry)
		}
	}
	// problem is complete (intent present) -> derived entry has no missing keys
	// and records the artifacts that satisfied it.
	if len(problem.Missing) != 0 || len(problem.Artifacts) == 0 {
		t.Fatalf("problem should derive as complete (no missing, artifacts populated): %#v", problem)
	}
	// grill is incomplete (no issue_url/branch/etc.) -> missing keys recorded.
	if grill, ok := a[IssueOpsPhaseGrill]; !ok || len(grill.Missing) == 0 {
		t.Fatalf("grill should derive as incomplete with missing keys: %#v", a[IssueOpsPhaseGrill])
	}
}

func TestIssueOpsPhaseLedgerIndexesChildrenComplete(t *testing.T) {
	keys := issueOpsPhaseArtifactKeys(IssueOpsPhasePR)
	if !containsLedgerKey(keys, "children_complete") {
		t.Fatalf("pr artifact set should include children_complete, got %#v", keys)
	}
	if containsLedgerKey(issueOpsPhaseArtifactKeys(IssueOpsPhaseImplement), "children_complete") {
		t.Fatalf("implement artifact set must not include children_complete")
	}
}

func containsLedgerKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

// isWallClock treats any non-empty timestamp as a wall-clock value; derived
// entries must use the empty sentinel.
func isWallClock(ts string) bool { return ts != "" }
