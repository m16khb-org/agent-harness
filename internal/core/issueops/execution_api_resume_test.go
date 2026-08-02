package issueops

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestExecutionActionResumeFailsClosedWithoutHandler(t *testing.T) {
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{Action: ExecutionActionResume, ID: "io-resume", Confirm: true}, ExecutionActionDependencies{})
	if !errors.Is(err, ErrResumeHandlerUnavailable) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExecutionActionResumePrioritizesConfirmBeforeMissingHandler(t *testing.T) {
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{Action: ExecutionActionResume, ID: "io-resume"}, ExecutionActionDependencies{})
	if err == nil || err.Error() != "execution resume requires confirm" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExecutionActionResumeUsesInjectedHandlerExactlyOnce(t *testing.T) {
	calls := 0
	request := ExecutionActionRequest{Action: ExecutionActionResume, ID: "io-resume", ExpectedGeneration: 5, CWD: "/repo.worktrees/193", Confirm: true}
	want := ExecutionResumeResult{OK: true, ID: request.ID, ClaimTokenPath: "token", IssueBodySHA256: "issue", ContextPacketPath: "packet", ContextPacketSHA256: "packet-sha", OwnerPromptPath: "prompt", OwnerPromptSHA256: "prompt-sha", NextCommand: "claim"}
	result, err := ExecuteExecution(context.Background(), t.TempDir(), request, ExecutionActionDependencies{Resume: func(_ context.Context, stateRoot string, got ExecutionResumeRequest) (ExecutionResumeResult, error) {
		calls++
		if stateRoot == "" || got.ID != request.ID || got.ExpectedGeneration != request.ExpectedGeneration || got.CWD != request.CWD || !got.Confirm {
			t.Fatalf("handler request=%+v state_root=%q", got, stateRoot)
		}
		return want, nil
	}})
	if err != nil || calls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, calls, err)
	}
	if got, ok := result.(ExecutionResumeResult); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v", result)
	}
}

func TestExecutionActionResumePrioritizesConfirmBeforeMutationGuardAndInvalidActor(t *testing.T) {
	stateRoot := t.TempDir()

	calls := 0
	result, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionResume,
		ID:     "io-resume",
		Actor:  issueops.NativeActor{},
	}, ExecutionActionDependencies{Resume: func(context.Context, string, ExecutionResumeRequest) (ExecutionResumeResult, error) {
		calls++
		return ExecutionResumeResult{}, fmt.Errorf("handler must not run")
	}})
	if err == nil || err.Error() != "execution resume requires confirm" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if calls != 0 {
		t.Fatalf("resume handler calls=%d", calls)
	}
}
