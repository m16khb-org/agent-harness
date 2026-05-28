package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCommandPolicyWritesRedactedJSONL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_AUDIT_LOG", filepath.Join(dir, "audit.jsonl"))
	record, err := AuditCommandPolicy(CommandPolicyRequest{WorkspaceRoot: dir, CWD: dir, Argv: []string{"echo", "token=secret-value"}, Timeout: "30s"})
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

func TestWorkerJobLifecycleIsNoShellStateOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	job, err := EnqueueWorkerJob("docs-refresh", "TOKEN=secret-value")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if job.Status != WorkerStatusQueued || !job.NoShell || strings.Contains(job.Payload, "secret-value") {
		t.Fatalf("unexpected job: %+v", job)
	}
	listed, err := ListWorkerJobs()
	if err != nil || len(listed.Jobs) != 1 {
		t.Fatalf("list jobs: %+v err=%v", listed, err)
	}
	cancelled, err := CancelWorkerJob(job.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != WorkerStatusCancelled {
		t.Fatalf("not cancelled: %+v", cancelled)
	}
}
