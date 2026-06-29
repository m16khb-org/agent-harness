package issueops

import (
	"strings"
	"testing"
)

// plan entry now enforces grill completion: split_decision + domain_review on
// top of the existing intent/issue_url/plan_prep readiness.
func issueOpsGrillGateBaseRecord(t *testing.T, stateRoot, repo, branch string) string {
	t.Helper()
	rec, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, rec.ID)
	if _, err := LinkIssueOpsIssue(stateRoot, rec.ID, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPlanPrepForTest(t, stateRoot, rec.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, rec.ID, string(IssueOpsPhaseGrill)); err != nil {
		t.Fatalf("grill entry should pass once intent is present: %v", err)
	}
	return rec.ID
}

func TestEnterPlanRequiresDomainReview(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	id := issueOpsGrillGateBaseRecord(t, stateRoot, repo, "1-gate")
	// split_decision satisfied via a scope decision, but no domain review yet.
	if _, err := AddIssueOpsDecision(stateRoot, id, IssueOpsDecisionRecordRequest{Title: "no split", Body: "single work item", Kind: "scope"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, id, string(IssueOpsPhasePlan)); err == nil || !strings.Contains(err.Error(), "domain_review") {
		t.Fatalf("plan entry must require domain_review, got %v", err)
	}
	if _, err := RecordIssueOpsDomainReview(stateRoot, id, IssueOpsDomainReviewRequest{ModelFit: "fits the model"}); err != nil {
		t.Fatal(err)
	}
	rec, err := AdvanceIssueOpsPhase(stateRoot, id, string(IssueOpsPhasePlan))
	if err != nil || rec.Phase != IssueOpsPhasePlan {
		t.Fatalf("plan entry should pass once grill is complete: %+v err=%v", rec, err)
	}
}

func TestEnterPlanRequiresSplitDecision(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	id := issueOpsGrillGateBaseRecord(t, stateRoot, repo, "2-gate")
	// domain review present, but no split decision (no child link, no scope decision).
	if _, err := RecordIssueOpsDomainReview(stateRoot, id, IssueOpsDomainReviewRequest{ModelFit: "fits the model"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, id, string(IssueOpsPhasePlan)); err == nil || !strings.Contains(err.Error(), "split_decision") {
		t.Fatalf("plan entry must require split_decision, got %v", err)
	}
	if _, err := AddIssueOpsDecision(stateRoot, id, IssueOpsDecisionRecordRequest{Title: "no split", Body: "single work item", Kind: "scope"}); err != nil {
		t.Fatal(err)
	}
	rec, err := AdvanceIssueOpsPhase(stateRoot, id, string(IssueOpsPhasePlan))
	if err != nil || rec.Phase != IssueOpsPhasePlan {
		t.Fatalf("plan entry should pass once split_decision is recorded: %+v err=%v", rec, err)
	}
}
