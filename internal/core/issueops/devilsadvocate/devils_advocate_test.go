package devilsadvocate

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestValidateVerdicts(t *testing.T) {
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatalf("pass should validate: %v", err)
	}
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "bogus"}); err == nil {
		t.Fatal("unknown verdict must fail")
	}
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "stop"}); err == nil {
		t.Fatal("stop without findings/waiver must fail")
	}
	got, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "stop", Findings: []string{"gold-plating", "gold-plating", "  "}})
	if err != nil {
		t.Fatalf("stop with findings should validate: %v", err)
	}
	if len(got.Findings) != 1 || got.ReviewerPattern != "devils-advocate-review" || got.RecordedAt == "" {
		t.Fatalf("findings should be cleaned/deduped and stamped: %+v", got)
	}
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "revise", Waived: true}); err == nil {
		t.Fatal("waive without rationale must fail")
	}
	waived, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "revise", Waived: true, WaiverRationale: "scoped follow-up issue filed"})
	if err != nil || !waived.Waived {
		t.Fatalf("waived revise should validate: %+v %v", waived, err)
	}
}

func TestRecordPersistsReview(t *testing.T) {
	var written model.IssueOpsRecord
	store := Store{
		Read: func(_, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id}, nil
		},
		TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			written = r
			return r, nil
		},
	}
	rec, err := Record(store, "root", "io-1", model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.DevilsAdvocateReview == nil || rec.DevilsAdvocateReview.Verdict != "pass" {
		t.Fatalf("review not persisted: %+v", rec.DevilsAdvocateReview)
	}
	if written.DevilsAdvocateReview == nil {
		t.Fatal("touch-write must receive the review")
	}
}
