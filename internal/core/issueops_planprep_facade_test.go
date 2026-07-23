package core

import (
	"path/filepath"
	"testing"
)

func TestRecordIssueOpsPlanPrepFacade(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	rec, err := StartIssueOps(root, IssueOpsStartRequest{Repo: t.TempDir(), Branch: "3-x"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	out, err := RecordIssueOpsPlanPrep(root, rec.ID, IssueOpsPlanPrepRequest{
		PriorDecisions: IssueOpsPlanPrepItemRequest{Evidence: []string{"ADR"}},
		RelatedIssues:  IssueOpsPlanPrepItemRequest{Evidence: []string{"remote score: #1(0.9)"}},
		WebResearch:    IssueOpsPlanPrepItemRequest{WaiveReason: "internal only"},
		CodebaseSurvey: IssueOpsPlanPrepItemRequest{Evidence: []string{"rg sweep of touched packages"}},
	})
	if err != nil {
		t.Fatalf("plan-prep: %v", err)
	}
	if out.PlanPrep == nil || out.PlanPrep.RelatedIssues.Status != "evidence" {
		t.Fatalf("unexpected: %#v", out.PlanPrep)
	}
}
