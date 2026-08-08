package executioncmd

import (
	"context"
	"testing"

	"agent-harness/internal/adapter/issueops"
)

func TestExecutionReseedCLIMapsCompletionGeneration(t *testing.T) {
	stateRoot := t.TempDir()
	calls := 0
	err := Run([]string{
		"replace", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "5",
		"--completion-generation", "4", "--inventory-fingerprint", "inventory",
		"--reason", "functional HEAD moved", "--reseed", "--confirm", "--json",
	}, Deps{
		StateRoot: func() string { return stateRoot },
		Reseed: func(_ context.Context, gotRoot string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
			calls++
			if gotRoot != stateRoot || request.ExpectedGeneration != 5 || request.CompletionGeneration != 4 {
				t.Fatalf("reseed handler request=%+v state_root=%q", request, gotRoot)
			}
			return issueops.ExecutionReplaceResult{OK: true, ID: request.ID, Action: issueops.ExecutionReplaceReseed}, nil
		},
		PrintJSON: func(any) error { return nil },
	})
	if err != nil || calls != 1 {
		t.Fatalf("reseed CLI err=%v calls=%d", err, calls)
	}
}
