package workercli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestExportWrappersDelegateToWorkerCommands(t *testing.T) {
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())
	tests := []struct {
		name    string
		run     func([]string) error
		args    []string
		wantErr string
	}{
		{name: "Run", run: Run, wantErr: "missing worker subcommand"},
		{name: "RunEnqueue", run: RunEnqueue, wantErr: "worker job kind is required"},
		{name: "RunDraftWiki", run: RunDraftWiki, args: []string{"--bad"}, wantErr: "flag provided but not defined"},
		{name: "RunReadOnly", run: RunReadOnly, wantErr: "requires --read-only"},
		{name: "RunStatus", run: RunStatus, args: []string{"--id", "missing"}, wantErr: "no such file"},
		{name: "RunList", run: RunList, args: []string{"--bad"}, wantErr: "flag provided but not defined"},
		{name: "RunCancel", run: RunCancel, args: []string{"--id", "missing"}, wantErr: "no such file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "blank", in: " \t ", want: nil},
		{name: "trims and drops empties", in: " PATH,HOME,, SHELL ", want: []string{"PATH", "HOME", "SHELL"}},
		{name: "single value", in: "TOKEN", want: []string{"TOKEN"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitCSV(tt.in); !workerStringSlicesEqual(got, tt.want) {
				t.Fatalf("splitCSV(%q)=%v, want %v", tt.in, got, tt.want)
			}
		})
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

func TestRunWorkerCleanupStuckMarksDeadPIDJobsFailed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)
	now := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	job := core.WorkerJob{
		OK:           true,
		ID:           "job-stuck-cli",
		Kind:         "read-only",
		Status:       core.WorkerStatusRunning,
		CreatedAt:    now,
		UpdatedAt:    now,
		StartedAt:    now,
		PID:          99999999,
		WorkerDir:    dir,
		NoShell:      true,
		SafetyNotice: "running before crash",
	}
	body, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, job.ID+".json"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStatusVerifyStdout(t, func() error {
		return runWorker([]string{"cleanup-stuck", "--json"})
	})
	var result core.WorkerListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode cleanup-stuck JSON: %v\n%s", err, out)
	}
	if len(result.Jobs) != 1 || result.Jobs[0].ID != job.ID || result.Jobs[0].Status != core.WorkerStatusFailed {
		t.Fatalf("expected stuck job to be failed, got %#v", result.Jobs)
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

func workerStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
