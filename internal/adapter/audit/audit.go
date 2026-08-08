package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	policydomain "agent-harness/internal/contract/policy"
)

// CommandAuditRecord는 policy 결정을 append-only로 남긴 redacted 기록이다.
type CommandAuditRecord struct {
	OK          bool                                 `json:"ok"`
	Kind        string                               `json:"kind"`
	AuditLogID  string                               `json:"audit_log_id"`
	GeneratedAt string                               `json:"generated_at"`
	LogPath     string                               `json:"log_path,omitempty"`
	Policy      policydomain.CommandPolicyEvaluation `json:"policy"`
}

// AuditCommandPolicy는 명령 요청을 평가해 redacted policy 결정을 JSONL audit
// log에 append한다. 명령 자체를 실행하지는 않는다.
func AuditCommandPolicy(req policydomain.CommandPolicyRequest) (CommandAuditRecord, error) {
	evaluation := EvaluateCommandPolicy(req)
	record := CommandAuditRecord{
		OK:          evaluation.Allowed,
		Kind:        "command_policy_audit",
		AuditLogID:  evaluation.AuditLogID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Policy:      evaluation,
	}
	path, err := commandAuditLogPath()
	if err != nil {
		return record, err
	}
	record.LogPath = path
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return record, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return record, err
	}
	defer f.Close()
	b, err := json.Marshal(record)
	if err != nil {
		return record, err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return record, err
	}
	return record, nil
}

func commandAuditLogPath() (string, error) {
	if path := os.Getenv("HARNESS_AUDIT_LOG"); path != "" {
		return filepath.Abs(path)
	}
	dir := os.Getenv("HARNESS_STATE_DIR")
	if dir == "" {
		base, err := os.UserHomeDir()
		if err != nil || base == "" {
			return "", fmt.Errorf("resolve home for audit log: %w", err)
		}
		dir = filepath.Join(base, ".local", "state", "agent-harness")
	}
	return filepath.Join(dir, "audit", "command-policy.jsonl"), nil
}
