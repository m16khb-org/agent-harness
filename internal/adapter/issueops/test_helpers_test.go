package issueops

import "testing"

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// recordIssueOpsProjectDocsReviewForTest는 publication 게이트가 요구하는
// project-doc 반영 판정을 기록한다. 테스트 사이클은 운영 문서를 건드리지
// 않으므로 no-change가 정확한 판정이다.
func recordIssueOpsProjectDocsReviewForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, id, IssueOpsProjectDocsReviewRequest{
		Verdict:  "no-change",
		Evidence: []string{"이 변경은 운영 문서에 남길 결정을 만들지 않는다"},
	}); err != nil {
		t.Fatal(err)
	}
}
