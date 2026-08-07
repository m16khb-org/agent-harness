package executioncmd

import (
	"context"
	"testing"

	"agent-harness/internal/adapter/issueops"
)

// 판정은 core가 하지만, CLI가 종결 표면을 실제로 넘기는지는 이 층에서만
// 확인된다. 주입이 빠지면 orca 모드 사이클의 task가 영원히 dispatched로
// 남는다(#130).
func TestActionDepsCarriesTheCompletionHandler(t *testing.T) {
	called := false
	deps := Deps{
		Complete: func(_ context.Context, _ string, request issueops.ExecutionCompleteRequest) (issueops.ExecutionResult, error) {
			called = true
			return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
		},
	}

	action := deps.actionDeps()
	if action.Complete == nil {
		t.Fatal("execution action dependencies must carry the completion handler")
	}
	if _, err := action.Complete(context.Background(), t.TempDir(), issueops.ExecutionCompleteRequest{ID: "io-198"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("the completion handler must reach the injected surface unchanged")
	}
}

// 주입되지 않은 호출자는 종전대로 동작한다.
func TestActionDepsWithoutCompletionHandlerLeavesItNil(t *testing.T) {
	if deps := (Deps{}).actionDeps(); deps.Complete != nil {
		t.Fatal("an uninjected completion handler must stay nil so routing fails closed")
	}
}
