package intentdesign

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func planPrepMemStore() (Store, *model.IssueOpsRecord) {
	rec := &model.IssueOpsRecord{ID: "io-1"}
	store := Store{
		Read: func(_, _ string) (model.IssueOpsRecord, error) { return *rec, nil },
		TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			*rec = r
			return r, nil
		},
		PlanReadiness: func(r model.IssueOpsRecord) model.IssueOpsReadiness {
			return model.IssueOpsReadiness{Ready: true}
		},
	}
	return store, rec
}

func TestRecordPlanPrepStoresEvidenceAndWaive(t *testing.T) {
	store, rec := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.IssueOpsPlanPrepRequest{
		PriorDecisions: model.IssueOpsPlanPrepItemRequest{Evidence: []string{".issueops/ADR.md"}},
		RelatedIssues:  model.IssueOpsPlanPrepItemRequest{Evidence: []string{"remote score: selected=#1(0.9)"}},
		WebResearch:    model.IssueOpsPlanPrepItemRequest{WaiveReason: "내부 전용 변경"},
		CodebaseSurvey: model.IssueOpsPlanPrepItemRequest{Evidence: []string{"rg gate: issueops_readiness.go, plan_prep.go 관련 심볼 전수 확인"}},
	})
	if err != nil {
		t.Fatalf("RecordPlanPrep error: %v", err)
	}
	if rec.PlanPrep == nil || rec.PlanPrep.PriorDecisions.Status != "evidence" || rec.PlanPrep.WebResearch.Status != "waived" {
		t.Fatalf("unexpected plan_prep: %#v", rec.PlanPrep)
	}
	if rec.PlanPrep.CodebaseSurvey.Status != "evidence" {
		t.Fatalf("codebase_survey not persisted: %#v", rec.PlanPrep)
	}
}

func TestRecordPlanPrepRejectsMissingCodebaseSurvey(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.IssueOpsPlanPrepRequest{
		PriorDecisions: model.IssueOpsPlanPrepItemRequest{Evidence: []string{"a"}},
		RelatedIssues:  model.IssueOpsPlanPrepItemRequest{Evidence: []string{"b"}},
		WebResearch:    model.IssueOpsPlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("codebase_survey with neither evidence nor waive must error")
	}
}

func TestRecordPlanPrepRejectsBothEvidenceAndWaive(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.IssueOpsPlanPrepRequest{
		PriorDecisions: model.IssueOpsPlanPrepItemRequest{Evidence: []string{"a"}, WaiveReason: "b"},
		RelatedIssues:  model.IssueOpsPlanPrepItemRequest{Evidence: []string{"a"}},
		WebResearch:    model.IssueOpsPlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("evidence + waive on one item must error")
	}
}

func TestRecordPlanPrepRejectsEmptyItem(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.IssueOpsPlanPrepRequest{
		PriorDecisions: model.IssueOpsPlanPrepItemRequest{},
		RelatedIssues:  model.IssueOpsPlanPrepItemRequest{Evidence: []string{"a"}},
		WebResearch:    model.IssueOpsPlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("item with neither evidence nor waive must error")
	}
}

func TestRecordIntentPersistsIntentClass(t *testing.T) {
	store, rec := planPrepMemStore()
	_, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "raw ask",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
		IntentClass:       "architecture",
	})
	if err != nil {
		t.Fatalf("RecordIntent error: %v", err)
	}
	if rec.Intent == nil || rec.Intent.IntentClass != "architecture" {
		t.Fatalf("intent_class not persisted: %#v", rec.Intent)
	}
}

func TestRecordIntentRejectsUnknownIntentClass(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "raw ask",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
		IntentClass:       "bogus",
	})
	if err == nil {
		t.Fatal("unknown intent_class must error")
	}
}
