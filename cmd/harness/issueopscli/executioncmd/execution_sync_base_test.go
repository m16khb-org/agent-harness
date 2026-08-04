package executioncmd

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestExecutionSyncBaseCLIMapsCompletionGeneration(t *testing.T) {
	var captured issueops.ExecutionSyncBaseRequest
	err := Run([]string{
		"sync-base", "--id", "io-sync-base-cli", "--completion-generation", "7", "--preview", "--cwd", "/worktree", "--json",
	}, Deps{
		StateRoot: func() string { return "/state" },
		syncBase: func(_ context.Context, stateRoot string, request issueops.ExecutionSyncBaseRequest, _ issueops.ExecutionSyncBaseDeps) (issueops.ExecutionSyncBaseResult, error) {
			if stateRoot != "/state" {
				t.Fatalf("state root=%q", stateRoot)
			}
			captured = request
			return issueops.ExecutionSyncBaseResult{OK: true, ID: request.ID, Mode: request.Mode}, nil
		},
		PrintJSON: func(any) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.ID != "io-sync-base-cli" || captured.Mode != issueops.ExecutionSyncBasePreview || captured.CompletionGeneration != 7 || captured.CWD != "/worktree" {
		t.Fatalf("captured request=%+v", captured)
	}
}

func TestExecutionSyncBaseCLIMutationRequiresProcessCWD(t *testing.T) {
	actual := t.TempDir()
	requested := t.TempDir()
	t.Chdir(actual)
	calls := 0
	err := Run([]string{
		"sync-base", "--id", "io-sync-base-cli", "--completion-generation", "7",
		"--apply", "--confirm", "--fingerprint", strings.Repeat("a", 64),
		"--host", "codex", "--session-id", "session", "--session-pid", "42",
		"--session-started-at", "2026-08-02T00:00:00Z", "--session-executable", "/opt/codex",
		"--cwd", requested, "--json",
	}, Deps{
		StateRoot: func() string { return "/state" },
		syncBase: func(_ context.Context, _ string, _ issueops.ExecutionSyncBaseRequest, _ issueops.ExecutionSyncBaseDeps) (issueops.ExecutionSyncBaseResult, error) {
			calls++
			return issueops.ExecutionSyncBaseResult{OK: true}, nil
		},
		PrintJSON:  func(any) error { return nil },
		PrintError: func(err error) error { return err },
	})
	if err == nil || !strings.Contains(err.Error(), "process working directory") {
		t.Fatalf("expected process cwd rejection, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("sync-base core was called before process cwd rejection: %d", calls)
	}
}

func TestExecutionSyncBaseCLIMutationAcceptsMatchingProcessCWD(t *testing.T) {
	requested := t.TempDir()
	t.Chdir(requested)
	calls := 0
	err := Run([]string{
		"sync-base", "--id", "io-sync-base-cli", "--completion-generation", "7",
		"--apply", "--confirm", "--fingerprint", strings.Repeat("a", 64),
		"--host", "codex", "--session-id", "session", "--session-pid", "42",
		"--session-started-at", "2026-08-02T00:00:00Z", "--session-executable", "/opt/codex",
		"--cwd", requested, "--json",
	}, Deps{
		StateRoot: func() string { return "/state" },
		syncBase: func(_ context.Context, _ string, request issueops.ExecutionSyncBaseRequest, _ issueops.ExecutionSyncBaseDeps) (issueops.ExecutionSyncBaseResult, error) {
			calls++
			if request.CWD != requested {
				t.Fatalf("sync-base request cwd=%q", request.CWD)
			}
			return issueops.ExecutionSyncBaseResult{OK: true}, nil
		},
		PrintJSON: func(any) error { return nil },
	})
	if err != nil || calls != 1 {
		t.Fatalf("matching process cwd err=%v calls=%d", err, calls)
	}
}
