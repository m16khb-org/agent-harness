package policycli

import (
	auditadapter "issueops/internal/adapter/audit"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	AuditCommandPolicy = auditadapter.AuditCommandPolicy
}
