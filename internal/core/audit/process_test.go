package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditProcessExecutionWritesBoundedRedactedJSONL(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	record, err := AuditProcessExecution(ProcessExecutionRequest{
		Name:       "codex-hooks-list",
		Executable: "/usr/local/bin/codex",
		Argv:       []string{"/usr/local/bin/codex", "token=process-secret"},
		CWD:        stateDir,
		Timeout:    15 * time.Second,
		EnvPolicy:  "codex_hooks_list_v1",
		EnvKeys:    []string{"CODEX_HOME", "HOME"},
		Outcome:    "failed",
		Diagnostic: strings.Repeat("x", processDiagnosticLimit+20) + " token=diagnostic-secret",
		StartedAt:  time.Now().Add(-time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.AuditLogID == "" || record.OK || len(record.Diagnostic) > processDiagnosticLimit+3 {
		t.Fatalf("unexpected process audit record: %+v", record)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "audit", "process-execution.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "process-secret") || strings.Contains(string(data), "diagnostic-secret") || !strings.Contains(string(data), "redacted") {
		t.Fatalf("process audit was not redacted: %s", data)
	}
}

func TestAuditProcessExecutionAssignsUniqueIDsToConsecutiveRecords(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	req := ProcessExecutionRequest{
		Name:      "codex-hooks-list",
		Argv:      []string{"/usr/local/bin/codex", "app-server", "--stdio"},
		CWD:       "/tmp/worker",
		Timeout:   15 * time.Second,
		EnvPolicy: "codex_hooks_list_v1",
		Outcome:   "success",
	}
	first, err := AuditProcessExecution(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AuditProcessExecution(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.AuditLogID == second.AuditLogID {
		t.Fatalf("consecutive process audits reused id %q", first.AuditLogID)
	}
}
