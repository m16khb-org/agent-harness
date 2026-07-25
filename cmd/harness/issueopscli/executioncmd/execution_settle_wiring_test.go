package executioncmd

import (
	"context"
	"testing"
)

// 판정은 core가 하지만, CLI가 종결 표면을 실제로 넘기는지는 이 층에서만
// 확인된다. 주입이 빠지면 orca 모드 사이클의 task가 영원히 dispatched로
// 남는다(#130).
func TestActionDepsCarriesTheOrcaTaskSettler(t *testing.T) {
	var settled []string
	deps := Deps{
		SettleOrcaTask: func(_ context.Context, taskID string) error {
			settled = append(settled, taskID)
			return nil
		},
	}

	action := deps.actionDeps()
	if action.SettleOrcaTask == nil {
		t.Fatal("execution action dependencies must carry the orca task settler")
	}
	if err := action.SettleOrcaTask(context.Background(), "task-130"); err != nil {
		t.Fatal(err)
	}
	if len(settled) != 1 || settled[0] != "task-130" {
		t.Fatalf("the settler must reach the injected surface unchanged: %+v", settled)
	}
}

// 주입되지 않은 호출자는 종전대로 동작한다.
func TestActionDepsWithoutSettlerLeavesItNil(t *testing.T) {
	if deps := (Deps{}).actionDeps(); deps.SettleOrcaTask != nil {
		t.Fatal("an uninjected settler must stay nil so completion skips settlement")
	}
}
