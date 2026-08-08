package policycli

import (
	auditcontract "agent-harness/internal/contract/audit"
	policycontract "agent-harness/internal/contract/policy"
)

// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	AuditCommandPolicy func(req policycontract.CommandPolicyRequest) (auditcontract.CommandAuditRecord, error)
)
