package workpoolcli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/workpool"
)

func TestRunWorkpoolLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	createOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"create", "--repo", repo, "--name", "cli lifecycle", "--size", "2", "--lease-ttl", "1h", "--json"})
	})
	var pool workpool.WorkPool
	unmarshalWorkpoolJSON(t, createOut, &pool)
	if !pool.OK || pool.ID == "" || pool.Status != "active" || pool.Size != 2 {
		t.Fatalf("unexpected created pool: %+v", pool)
	}

	for _, title := range []string{"first task", "second task", "third task"} {
		addOut := captureWorkpoolStdout(t, func() error {
			return Run([]string{"add-task", "--pool", pool.ID, "--title", title, "--instructions", "touch one scoped file", "--scope", "one file", "--acceptance", "evidence is recorded", "--json"})
		})
		var task workpool.WorkTask
		unmarshalWorkpoolJSON(t, addOut, &task)
		if !task.OK || task.PoolID != pool.ID || task.Status != "pending" {
			t.Fatalf("unexpected added task: %+v", task)
		}
	}

	claimOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"claim", "--pool", pool.ID, "--worker", "worker-a", "--json"})
	})
	var claim workpool.ClaimResult
	unmarshalWorkpoolJSON(t, claimOut, &claim)
	if !claim.OK || claim.Task.Status != "leased" || claim.Task.WorkerID != "worker-a" || claim.Prompt == "" {
		t.Fatalf("unexpected claim result: %+v", claim)
	}

	heartbeatOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"heartbeat", "--pool", pool.ID, "--task", claim.Task.ID, "--worker", "worker-a", "--json"})
	})
	var heartbeat workpool.WorkTask
	unmarshalWorkpoolJSON(t, heartbeatOut, &heartbeat)
	if heartbeat.Status != "leased" || heartbeat.WorkerID != "worker-a" {
		t.Fatalf("unexpected heartbeat result: %+v", heartbeat)
	}

	submitOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"submit", "--pool", pool.ID, "--task", claim.Task.ID, "--worker", "worker-a", "--evidence", "go test ./...", "--branch", "workpool/first", "--worktree", filepath.Join(repo, ".worktrees", "first"), "--json"})
	})
	var submitted workpool.WorkTask
	unmarshalWorkpoolJSON(t, submitOut, &submitted)
	if submitted.Status != "submitted" || len(submitted.Evidence) != 1 {
		t.Fatalf("unexpected submit result: %+v", submitted)
	}

	acceptOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"accept", "--pool", pool.ID, "--task", claim.Task.ID, "--evidence", "reviewed submitted evidence", "--json"})
	})
	var accepted workpool.WorkTask
	unmarshalWorkpoolJSON(t, acceptOut, &accepted)
	if accepted.Status != "accepted" {
		t.Fatalf("unexpected accept result: %+v", accepted)
	}

	statusOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"status", "--pool", pool.ID, "--json"})
	})
	var status struct {
		OK     bool                `json:"ok"`
		Pool   workpool.WorkPool   `json:"pool"`
		Tasks  []workpool.WorkTask `json:"tasks"`
		Counts map[string]int      `json:"counts"`
	}
	unmarshalWorkpoolJSON(t, statusOut, &status)
	if !status.OK || status.Pool.ID != pool.ID || len(status.Tasks) != 3 || status.Counts["accepted"] != 1 || status.Counts["pending"] != 2 {
		t.Fatalf("unexpected status result: %+v", status)
	}

	closeOut := captureWorkpoolStdout(t, func() error {
		return Run([]string{"close", "--pool", pool.ID, "--force", "--reason", "test force closes remaining pending work", "--json"})
	})
	var closed workpool.WorkPool
	unmarshalWorkpoolJSON(t, closeOut, &closed)
	if closed.Status != "closed" {
		t.Fatalf("unexpected close result: %+v", closed)
	}
}

func captureWorkpoolStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func unmarshalWorkpoolJSON(t *testing.T, out string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), target); err != nil {
		t.Fatalf("unmarshal workpool JSON: %v\n%s", err, out)
	}
}
