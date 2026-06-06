package core

import (
	"agent-harness/internal/core/audit"
	"agent-harness/internal/core/policy"
)

type CommandAuditRecord = audit.CommandAuditRecord

func AuditCommandPolicy(req policy.CommandPolicyRequest) (CommandAuditRecord, error) {
	return audit.AuditCommandPolicy(req)
}
