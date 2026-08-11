// IssueOps CLI가 읽는 구현 검토 DTO다.
package issueops

// IssueOpsImplementationReviewRequest는 구현 diff에 대한 brooks 리뷰 기록이다.
type IssueOpsImplementationReviewRequest struct {
	Verdict        string
	Findings       []string
	Evidence       []string
	ReviewerHost   string
	ReviewerModel  string
	ReviewerEffort string
}

// 설계 검토 증거 예시 문구는 CLI 도움말과 어댑터가 함께 쓰는 어휘다.
const IssueOpsDesignReviewEvidenceExample = "design review checked alternatives and risks"
