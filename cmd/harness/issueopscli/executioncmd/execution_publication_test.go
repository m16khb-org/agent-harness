package executioncmd

import (
	"context"
	"reflect"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestActionDepsPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueops.RemotePullRequestReconcileHandler(func(context.Context, string, issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
		invoked++
		return issueops.ExecutionReconcileResult{}, nil
	})

	deps := (Deps{Publication: issueops.RemotePublicationHandlers{Reconcile: handler}}).actionDeps()
	if deps.RemoteReconcile == nil {
		t.Fatal("publication reconcile handler was not propagated")
	}
	if reflect.ValueOf(deps.RemoteReconcile).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("publication reconcile handler changed during execution dependency mapping")
	}
	if invoked != 0 {
		t.Fatalf("publication reconcile handler invoked during propagation: %d", invoked)
	}
}
