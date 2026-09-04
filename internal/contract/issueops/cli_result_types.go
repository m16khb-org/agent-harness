// IssueOps CLI가 읽는 구현 검토 DTO다.
package issueops

// IssueOpsImplementationReviewRequest는 구현 diff에 대한 design-review 리뷰 기록이다.
type IssueOpsImplementationReviewRequest struct {
	Verdict        string
	Findings       []string
	Evidence       []string
	ReviewerHost   string
	ReviewerModel  string
	ReviewerEffort string
}

// IssueOpsProjectDocsReviewRequest는 publication 직전 project-doc 반영 판정이다.
type IssueOpsProjectDocsReviewRequest struct {
	Verdict  string
	Docs     []string
	Evidence []string
}

// IssueOpsSchemaEvidenceRequest는 스키마 변경 사이클의 실측 근거 기록이다.
type IssueOpsSchemaEvidenceRequest struct {
	Measurements    []string
	Sources         []string
	Waive           bool
	WaiverRationale string
}

// 설계 검토 증거 예시 문구는 CLI 도움말과 어댑터가 함께 쓰는 어휘다.
const IssueOpsDesignReviewEvidenceExample = "design review checked alternatives and risks"
