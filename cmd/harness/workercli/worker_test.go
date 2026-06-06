package workercli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunWorkerRoutesUsageAndUnknownSubcommands(t *testing.T) {
	if err := runWorker(nil); err == nil || !strings.Contains(err.Error(), "missing worker subcommand") {
		t.Fatalf("expected missing worker subcommand error, got %v", err)
	}

	if err := runWorker([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown worker subcommand "unknown"`) {
		t.Fatalf("expected unknown worker subcommand error, got %v", err)
	}
}

func TestRunWorkerRunExecutesReadOnlyCommandAsJSON(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())

	out := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{
			"run",
			"--read-only",
			"--kind", "read-only",
			"--workspace-root", repo,
			"--cwd", repo,
			"--json",
			"--",
			"git", "status", "--short",
		})
	})

	var job core.WorkerJob
	if err := json.Unmarshal([]byte(out), &job); err != nil {
		t.Fatalf("decode worker run JSON: %v\n%s", err, out)
	}
	if !job.OK || job.Status != core.WorkerStatusSucceeded {
		t.Fatalf("expected succeeded worker job, got %#v", job)
	}
	if job.Result == nil || !job.Result.Policy.Allowed {
		t.Fatalf("expected allowed command result, got %#v", job.Result)
	}
}

func TestRunWorkerRunRequiresReadOnlyAndSurfacesDeniedPolicy(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())

	if err := runWorker([]string{"run", "--workspace-root", repo, "--", "git", "status", "--short"}); err == nil || !strings.Contains(err.Error(), "requires --read-only") {
		t.Fatalf("expected read-only requirement error, got %v", err)
	}

	err := runWorker([]string{
		"run",
		"--read-only",
		"--workspace-root", repo,
		"--cwd", repo,
		"--",
		"sh", "-c", "true",
	})
	var denied core.PolicyDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected policy denied error, got %T %v", err, err)
	}
}

func TestRunWorkerLifecycleCommands(t *testing.T) {
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())

	enqueueOut := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{"enqueue", "--kind", "docs-refresh", "--payload", "TOKEN=secret-value", "--json"})
	})
	var queued core.WorkerJob
	if err := json.Unmarshal([]byte(enqueueOut), &queued); err != nil {
		t.Fatalf("decode enqueue JSON: %v\n%s", err, enqueueOut)
	}
	if queued.ID == "" || queued.Status != core.WorkerStatusQueued {
		t.Fatalf("expected queued worker job, got %#v", queued)
	}
	if strings.Contains(queued.Payload, "secret-value") {
		t.Fatalf("worker payload must be redacted, got %q", queued.Payload)
	}

	statusOut := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{"status", "--id", queued.ID, "--json"})
	})
	var status core.WorkerJob
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("decode status JSON: %v\n%s", err, statusOut)
	}
	if status.ID != queued.ID || status.Status != core.WorkerStatusQueued {
		t.Fatalf("expected queued status for %s, got %#v", queued.ID, status)
	}

	listOut := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{"list", "--json"})
	})
	var listed core.WorkerListResult
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("decode list JSON: %v\n%s", err, listOut)
	}
	if len(listed.Jobs) != 1 || listed.Jobs[0].ID != queued.ID {
		t.Fatalf("expected one listed job %s, got %#v", queued.ID, listed.Jobs)
	}

	cancelOut := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{"cancel", "--id", queued.ID, "--json"})
	})
	var cancelled core.WorkerJob
	if err := json.Unmarshal([]byte(cancelOut), &cancelled); err != nil {
		t.Fatalf("decode cancel JSON: %v\n%s", err, cancelOut)
	}
	if cancelled.ID != queued.ID || cancelled.Status != core.WorkerStatusCancelled {
		t.Fatalf("expected cancelled job %s, got %#v", queued.ID, cancelled)
	}
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer r.Close()
	os.Stdout = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	if callErr != nil {
		t.Fatalf("call failed: %v", callErr)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return out.String()
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}
