package issueops

import (
	"testing"

	"agent-harness/internal/contract/issueops"
)

func fullIntentForLedger() *issueops.IssueOpsIntentContract {
	return &issueops.IssueOpsIntentContract{
		RawRequest:        "do x",
		InterpretedIntent: "implement x",
		SuccessCriteria:   []string{"x works"},
		IntentClass:       "standard",
		RecordedAt:        "2026-06-29T00:00:00Z",
	}
}

func waivedPlanPrep() *issueops.IssueOpsPlanPrep {
	item := issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a for this small change"}
	return &issueops.IssueOpsPlanPrep{PriorDecisions: item, RelatedIssues: item, WebResearch: item, CodebaseSurvey: item, RecordedAt: "2026-06-29T00:00:00Z"}
}

func TestIssueOpsProblemReadinessNeedsOnlyIntent(t *testing.T) {
	if r := IssueOpsProblemReadiness(issueops.IssueOpsRecord{}); r.Ready {
		t.Fatalf("empty record should not complete problem: %#v", r)
	} else if !containsLedgerKey(r.Missing, "intent_contract") {
		t.Fatalf("expected intent_contract missing, got %#v", r.Missing)
	}
	rec := issueops.IssueOpsRecord{Intent: fullIntentForLedger()}
	if r := IssueOpsProblemReadiness(rec); !r.Ready {
		t.Fatalf("intent-complete record should complete problem regardless of issue_url/branch: %#v", r)
	}
}

func TestIssueOpsGrillReadinessRequiresArtifacts(t *testing.T) {
	rec := issueops.IssueOpsRecord{Intent: fullIntentForLedger()}
	r := IssueOpsGrillReadiness(rec)
	if r.Ready {
		t.Fatalf("bare grill should not be ready: %#v", r)
	}
	for _, want := range []string{"issue_url", "branch", "split_decision", "domain_review"} {
		if !containsLedgerKey(r.Missing, want) {
			t.Fatalf("grill missing should include %q, got %#v", want, r.Missing)
		}
	}

	complete := issueops.IssueOpsRecord{
		Intent:    fullIntentForLedger(),
		IssueURL:  "https://example/issues/1",
		Branch:    "1-x",
		PlanPrep:  waivedPlanPrep(),
		Decisions: []issueops.IssueOpsDecision{{Title: "no split", Kind: "scope", CreatedAt: "2026-06-29T00:00:00Z"}},
		DomainReview: &issueops.IssueOpsDomainReview{
			ModelFit:   "fits",
			ReviewedAt: "2026-06-29T00:00:00Z",
		},
	}
	if r := IssueOpsGrillReadiness(complete); !r.Ready {
		t.Fatalf("fully populated grill should be ready, got missing %#v", r.Missing)
	}
}

func TestIssueOpsGrillSplitDecisionAcceptsChildLink(t *testing.T) {
	rec := issueops.IssueOpsRecord{
		Intent:       fullIntentForLedger(),
		IssueURL:     "https://example/issues/1",
		Branch:       "1-x",
		PlanPrep:     waivedPlanPrep(),
		IssueLinks:   []issueops.IssueOpsIssueLink{{Type: "child", URL: "https://example/issues/2", CreatedAt: "2026-06-29T00:00:00Z"}},
		DomainReview: &issueops.IssueOpsDomainReview{ModelFit: "fits", ReviewedAt: "2026-06-29T00:00:00Z"},
	}
	if r := IssueOpsGrillReadiness(rec); !r.Ready {
		t.Fatalf("child issue link should satisfy split_decision, missing %#v", r.Missing)
	}
}

func TestIssueOpsPhaseCompletionDispatches(t *testing.T) {
	rec := issueops.IssueOpsRecord{Intent: fullIntentForLedger()}
	if r := IssueOpsPhaseCompletion(rec, IssueOpsPhaseProblem); !r.Ready {
		t.Fatalf("problem completion should be ready for intent-only record: %#v", r)
	}
	if r := IssueOpsPhaseCompletion(rec, IssueOpsPhaseGrill); r.Ready {
		t.Fatalf("grill completion should not be ready for intent-only record")
	}
}

func containsLedgerKey(keys []string, want string) bool {
	for _, key := range keys {
		if key == want {
			return true
		}
	}
	return false
}
