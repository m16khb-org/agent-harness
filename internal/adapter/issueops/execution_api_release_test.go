package issueops

import (
	"context"
	"errors"
	"testing"
)

func TestExecuteExecutionReleaseRequiresInjectedHandler(t *testing.T) {
	_, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{
		Action: ExecutionActionRelease,
		ID:     "io-release-handler",
	}, ExecutionActionDependencies{})
	if !errors.Is(err, ErrReleaseHandlerUnavailable) {
		t.Fatalf("release error=%v, want unavailable handler", err)
	}
}

func TestExecuteExecutionReleaseUsesInjectedHandlerOnce(t *testing.T) {
	called := 0
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{
		Action:     ExecutionActionRelease,
		ID:         "io-release-handler",
		Generation: 3,
		CWD:        "/canonical/worktree",
	}, ExecutionActionDependencies{Release: func(_ context.Context, stateRoot string, request ExecutionReleaseRequest) (ExecutionResult, error) {
		called++
		if stateRoot == "" || request.ID != "io-release-handler" || request.Generation != 3 || request.CWD != "/canonical/worktree" {
			t.Fatalf("unexpected injected release request: root=%q request=%+v", stateRoot, request)
		}
		return ExecutionResult{OK: true, ID: request.ID}, nil
	}})
	if err != nil {
		t.Fatalf("execute release: %v", err)
	}
	got, ok := result.(ExecutionResult)
	if !ok || !got.OK || got.ID != "io-release-handler" || called != 1 {
		t.Fatalf("result=%#v called=%d", result, called)
	}
}
