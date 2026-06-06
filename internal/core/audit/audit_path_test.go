package audit

import (
	"path/filepath"
	"testing"
)

func TestCommandAuditLogPathUsesExplicitEnvPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("HARNESS_AUDIT_LOG", path)
	t.Setenv("HARNESS_STATE_DIR", filepath.Join(t.TempDir(), "state"))

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
	t.Setenv("HARNESS_AUDIT_LOG", "")
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	got, err := commandAuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(stateDir, "audit", "command-policy.jsonl")
	if got != want {
		t.Fatalf("commandAuditLogPath() = %q, want %q", got, want)
	}
}
