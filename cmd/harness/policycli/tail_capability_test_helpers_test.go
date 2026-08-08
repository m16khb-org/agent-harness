package policycli

import (
	auditadapter "agent-harness/internal/adapter/audit"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	AuditCommandPolicy = auditadapter.AuditCommandPolicy
}
