// Package audit는 audit capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package audit

import policycontract "agent-harness/internal/contract/policy"

// CommandAuditRecord는 policy 결정을 append-only로 남긴 redacted 기록이다.
type CommandAuditRecord struct {
	OK          bool                                   `json:"ok"`
	Kind        string                                 `json:"kind"`
	AuditLogID  string                                 `json:"audit_log_id"`
	GeneratedAt string                                 `json:"generated_at"`
	LogPath     string                                 `json:"log_path,omitempty"`
	Policy      policycontract.CommandPolicyEvaluation `json:"policy"`
}
