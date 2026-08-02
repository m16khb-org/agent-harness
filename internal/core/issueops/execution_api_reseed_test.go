package issueops

import (
	"context"
	"errors"
	"strings"
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

func TestExecutionReseedNextCommandRendersModeSpecificRecovery(t *testing.T) {
	direct := ExecutionReseedNextCommand("io-direct", 2, "direct", "/tmp/lease-2.token")
	for _, want := range []string{"execution claim", "--generation 2", "/tmp/lease-2.token"} {
		if !strings.Contains(direct, want) {
			t.Fatalf("direct reseed next command %q does not contain %q", direct, want)
		}
	}
	orca := ExecutionReseedNextCommand("io-orca", 3, "orca", "/tmp/ignored.token")
	if !strings.Contains(orca, "execution resume") || strings.Contains(orca, "/tmp/ignored.token") {
		t.Fatalf("Orca reseed next command = %q", orca)
	}
	if got := ExecutionReseedNextCommand("io-unknown", 1, "unknown", "/tmp/token"); got != "" {
		t.Fatalf("unknown mode next command = %q, want empty", got)
	}
}
