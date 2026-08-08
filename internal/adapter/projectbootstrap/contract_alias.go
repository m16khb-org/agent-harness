package projectbootstrap

import projectbootstrapcontract "agent-harness/internal/contract/projectbootstrap"

// 부트스트랩 요청·결과는 계약 DTO다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	ProjectDocsBootstrapRequest = projectbootstrapcontract.ProjectDocsBootstrapRequest
	ProjectDocsBootstrapResult  = projectbootstrapcontract.ProjectDocsBootstrapResult
)
