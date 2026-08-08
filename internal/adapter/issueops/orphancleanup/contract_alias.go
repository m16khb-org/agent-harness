package orphancleanup

import orphancontract "agent-harness/internal/contract/issueopsorphancleanup"

// 요청·결과 DTO는 계약이 소유한다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	Request      = orphancontract.Request
	ApplyRequest = orphancontract.ApplyRequest
	Result       = orphancontract.Result
)
