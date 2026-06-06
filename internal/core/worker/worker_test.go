package worker

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"agent-harness/internal/core/policy"
)

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
	cancelledAgain, err := CancelWorkerJob(job.ID)
	if err != nil {
		t.Fatalf("cancel again: %v", err)
	}
	if cancelledAgain.Status != WorkerStatusCancelled {
		t.Fatalf("second cancel should be no-op: %+v", cancelledAgain)
	}
}

func TestWorkerJobInvalidAndListErrorBranches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	if _, err := EnqueueWorkerJob("", "payload"); err == nil || !strings.Contains(err.Error(), "worker job kind is required") {
		t.Fatalf("empty kind error = %v", err)
	}
	if _, err := EnqueueWorkerJob("../bad", "payload"); err == nil || !strings.Contains(err.Error(), "invalid worker job kind") {
		t.Fatalf("invalid kind error = %v", err)
	}
	if _, err := ReadWorkerJob("../bad"); err == nil || !strings.Contains(err.Error(), "invalid worker job id") {
		t.Fatalf("invalid read id error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err := ListWorkerJobs()
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(listed.Jobs) != 0 {
		t.Fatalf("broken/non-json jobs should be skipped: %+v", listed)
	}
	if _, err := ReadWorkerJob("broken"); err == nil {
		t.Fatalf("broken job JSON should fail to read")
	}
}

func TestWorkerCancelRejectsNonQueuedJobs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	job, err := EnqueueWorkerJob("cancel-status", "payload")
	if err != nil {
		t.Fatal(err)
	}
	job.Status = WorkerStatusSucceeded
	if err := writeWorkerJob(job); err != nil {
		t.Fatal(err)
	}
	cancelled, err := CancelWorkerJob(job.ID)
	if err == nil || !strings.Contains(err.Error(), "cannot be cancelled from status succeeded") {
		t.Fatalf("cancel succeeded job error = %v", err)
	}
	if cancelled.Status != WorkerStatusSucceeded {
		t.Fatalf("cancel should preserve succeeded status: %+v", cancelled)
	}
}

func TestWorkerDirUsesStateDirFallback(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", "")
	t.Setenv("HARNESS_STATE_DIR", state)
	dir, err := workerDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(state, "worker") {
		t.Fatalf("workerDir = %q, want state worker dir", dir)
	}
}

func TestWorkerRunReadOnlyJobExecutesPolicyAllowedCommand(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("worker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	job, err := RunReadOnlyWorkerJob("read-only", "", policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"cat", "note.txt"},
		Timeout:       "30s",
	})
	if err != nil {
		t.Fatalf("worker run: %v", err)
	}
	if job.Status != WorkerStatusSucceeded || job.Result == nil || job.Result.Stdout != "worker\n" {
		t.Fatalf("unexpected worker run job: %+v", job)
	}
	stored, err := ReadWorkerJob(job.ID)
	if err != nil {
		t.Fatalf("read stored job: %v", err)
	}
	if stored.Status != WorkerStatusSucceeded || stored.Result == nil || stored.Result.ExitCode != 0 {
		t.Fatalf("stored job missing result: %+v", stored)
	}
}

func TestWorkerRunReadOnlyTimeoutFailsJobWithBoundedStderr(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	fifo := filepath.Join(root, "blocked.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := RunReadOnlyWorkerJob("read-only-timeout", "", policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"cat", "blocked.fifo"},
		Timeout:       "20ms",
	})
	if err != nil {
		t.Fatalf("worker run timeout: %v", err)
	}
	if job.Status != WorkerStatusFailed || job.Result == nil || !job.Result.TimedOut || job.Result.ExitCode != 124 {
		t.Fatalf("expected timed-out failed job: %+v", job)
	}
	if !strings.Contains(job.Result.Stderr, "command timed out") || len(job.Result.Stderr) > 4096 {
		t.Fatalf("unexpected timeout stderr: %q", job.Result.Stderr)
	}
	stored, err := ReadWorkerJob(job.ID)
	if err != nil {
		t.Fatalf("read stored job: %v", err)
	}
	if stored.Status != WorkerStatusFailed || stored.Result == nil || !stored.Result.TimedOut {
		t.Fatalf("stored timeout job missing result: %+v", stored)
	}
}

func TestWorkerRunReadOnlyDeniedCommandFailsJobWithoutMarker(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	marker := filepath.Join(root, "marker")
	job, err := RunReadOnlyWorkerJob("read-only-denied", "", policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"touch", "marker"},
		Timeout:       "30s",
	})
	if err != nil {
		t.Fatalf("worker run denied: %v", err)
	}
	if job.Status != WorkerStatusFailed || job.Result == nil || job.Result.Executed || job.Result.ExitCode != 3 {
		t.Fatalf("expected denied failed job without execution: %+v", job)
	}
	if !strings.Contains(strings.Join(job.Result.Policy.DenyReasons, ","), "write_not_allowed") {
		t.Fatalf("missing write denial reason: %+v", job.Result.Policy.DenyReasons)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("denied command created marker or unexpected stat error: %v", err)
	}
}
