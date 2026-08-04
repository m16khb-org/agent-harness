package executioncmd

import (
	"context"
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
