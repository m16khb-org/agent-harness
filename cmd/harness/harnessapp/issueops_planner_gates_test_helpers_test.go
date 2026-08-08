package harnessapp

import (
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// seedPlannerGates는 Orca prepare가 owner를 띄우기 전에 요구하는 planner 소유
// 기록을 심는다(#319).
//
// owner는 이것들을 보충할 수 없다 — planner의 판단이고 owner packet의 commands
// map에 없다. 그래서 prepare가 없는 상태로 owner를 띄우지 않는다. 이 헬퍼가
// 있는 이유는 그 게이트를 우회하려는 것이 아니라, prepare 자체를 검증하는
// 테스트가 **유효한 출발 상태**에서 시작해야 하기 때문이다.
func seedPlannerGates(t *testing.T, stateRoot, id string) {
	t.Helper()
	// design review는 linked issue를 요구한다. Orca fixture는 issue URL을
	// BranchPrepare에만 담기도 하므로, record 쪽이 비어 있으면 거기서 가져온다.
	record, err := issueopscore.ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatalf("seed read: %v", err)
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		url := "https://github.com/example/repo/issues/199"
		if record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.IssueURL) != "" {
			url = record.BranchPrepare.IssueURL
		}
		if _, err := issueopscore.LinkIssueOpsIssue(stateRoot, id, url); err != nil {
			t.Fatalf("seed link issue: %v", err)
		}
	}
	if _, err := issueopscore.RecordIssueOpsIntent(stateRoot, id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest: "wiring fixture", InterpretedIntent: "verify the prepare wiring",
		SuccessCriteria: []string{"prepare returns a bound next command"},
	}); err != nil {
		t.Fatalf("seed intent: %v", err)
	}
	if _, err := issueopscore.RecordIssueOpsDesignReview(stateRoot, id, issueopscontract.IssueOpsDesignReviewRequest{
		ProblemSummary: "wiring fixture", ProposedDesign: "exercise the prepare path",
		Verification: []string{"design review checked alternatives and risks"}, Approved: true,
		RefactorPlan: "none", Alternatives: []string{"none"}, Risks: []string{"none"},
	}); err != nil {
		t.Fatalf("seed design review: %v", err)
	}
	if _, err := issueopscore.RecordIssueOpsDevilsAdvocateReview(stateRoot, id, issueopscontract.IssueOpsDevilsAdvocateReviewRequest{
		Verdict: "pass", Findings: []string{"fixture"},
	}); err != nil {
		t.Fatalf("seed devils advocate review: %v", err)
	}
}
