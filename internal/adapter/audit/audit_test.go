package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	policy "agent-harness/internal/contract/policy"
)

func TestAuditCommandPolicyWritesRedactedJSONL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_AUDIT_LOG", filepath.Join(dir, "audit.jsonl"))
	record, err := AuditCommandPolicy(policy.CommandPolicyRequest{WorkspaceRoot: dir, CWD: dir, Argv: []string{"echo", "token=secret-value"}, Timeout: "30s"})
	if err != nil {
		t.Fatalf("audit policy: %v", err)
	}
	if record.LogPath == "" || record.Policy.Allowed {
		t.Fatalf("expected denied audited policy with path: %+v", record)
	}
	b, err := os.ReadFile(record.LogPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if strings.Contains(string(b), "secret-value") || !strings.Contains(string(b), "redacted") {
		t.Fatalf("audit log was not redacted: %s", string(b))
	}
}
