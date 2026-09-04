package audit

import (
	"path/filepath"
	"testing"
)

func TestCommandAuditLogPathUsesExplicitEnvPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("ISSUEOPS_AUDIT_LOG", path)
	t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(t.TempDir(), "state"))

	got, err := commandAuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("commandAuditLogPath() = %q, want %q", got, path)
	}
}

func TestCommandAuditLogPathUsesStateDirFallback(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_AUDIT_LOG", "")
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)

	got, err := commandAuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(stateDir, "audit", "command-policy.jsonl")
	if got != want {
		t.Fatalf("commandAuditLogPath() = %q, want %q", got, want)
	}
}
