package issueops

import (
	"context"
	"errors"
	"testing"
)

func TestExecuteExecutionReseedRequiresInjectedHandler(t *testing.T) {
	_, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{
		Action:        ExecutionActionReplace,
		ReplaceAction: ExecutionReplaceReseed,
		ID:            "io-reseed-handler",
	}, ExecutionActionDependencies{})
	if !errors.Is(err, ErrReseedHandlerUnavailable) {
		t.Fatalf("reseed error=%v, want unavailable handler", err)
	}
}

func TestExecuteExecutionReseedUsesInjectedHandlerOnce(t *testing.T) {
	called := 0
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{
		Action:               ExecutionActionReplace,
		ReplaceAction:        ExecutionReplaceReseed,
		ID:                   "io-reseed-handler",
		ExpectedGeneration:   3,
		InventoryFingerprint: "inventory",
		Reason:               "holderless recovery",
		CWD:                  "/canonical/worktree",
		Confirm:              true,
	}, ExecutionActionDependencies{Reseed: func(_ context.Context, stateRoot string, request ExecutionReseedRequest) (ExecutionReplaceResult, error) {
		called++
		if stateRoot == "" || request.ID != "io-reseed-handler" || request.ExpectedGeneration != 3 || request.InventoryFingerprint != "inventory" || request.Reason != "holderless recovery" || request.CWD != "/canonical/worktree" || !request.Confirm {
			t.Fatalf("unexpected injected reseed request: root=%q request=%+v", stateRoot, request)
		}
		return ExecutionReplaceResult{OK: true, ID: request.ID, Action: ExecutionReplaceReseed}, nil
	}})
	if err != nil {
		t.Fatalf("execute reseed: %v", err)
	}
	got, ok := result.(ExecutionReplaceResult)
	if !ok || !got.OK || got.ID != "io-reseed-handler" || called != 1 {
		t.Fatalf("result=%#v called=%d", result, called)
	}
}
