package port

import executionissue "issueops/internal/contract/executionissue"

// 이슈 스냅샷 DTO는 계약이 소유한다. port는 인터페이스 시그니처에서 쓰기 위해
// 같은 이름으로 재노출만 한다.
type (
	ExecutionIssueSnapshotRequest        = executionissue.ExecutionIssueSnapshotRequest
	ExecutionIssueSnapshot               = executionissue.ExecutionIssueSnapshot
	ExecutionIssueSnapshotEvidence       = executionissue.ExecutionIssueSnapshotEvidence
	IssueProviderCreatePullRequestResult = executionissue.IssueProviderCreatePullRequestResult
	ExecutionIssueSnapshotReadFunc       = executionissue.ExecutionIssueSnapshotReadFunc
	ExecutionPrepareInvocation           = executionissue.ExecutionPrepareInvocation
)
