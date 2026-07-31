package orca

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// SettleTask는 완료된 사이클의 task를 terminal 상태로 옮긴다. 어떤 status가
// 종결인지는 Orca 쪽 지식이므로 호출자가 아니라 이 계층이 정하며, 그 값이
// operationalhealth 분류기의 면제 조건과 일치해야 residue가 사라진다(#130).
func TestClientSettleTaskSendsTheCompletedStatus(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "orchestration", "task-update", "--id", "task-130", "--status", "completed", "--run", "run_issueops_1", "--json"}
	runner.responses[strings.Join(argv, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{}}`)}

	if err := NewClient(runner).SettleTask(context.Background(), "run_issueops_1", "task-130"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || !slices.Equal(runner.calls[0], argv) {
		t.Fatalf("settle must project exactly the completed task-update: %#v", runner.calls)
	}
}

func TestClientSettleTaskResolvesLegacyBindingFromOneExplicitRun(t *testing.T) {
	runner := newFakeRunner(t)
	list := []string{"orca", "orchestration", "task-list", "--brief", "--run", "run_issueops_1", "--json"}
	update := []string{"orca", "orchestration", "task-update", "--id", "task-130", "--status", "completed", "--run", "run_issueops_1", "--json"}
	runner.responses[strings.Join(list, " ")] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[{"id":"task-130","status":"completed"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses[strings.Join(update, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{}}`)}

	if err := NewClient(runner).SettleTask(context.Background(), "", "task-130"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 3 || !slices.Equal(runner.calls[1], list) || !slices.Equal(runner.calls[2], update) {
		t.Fatalf("legacy task Run resolution calls = %#v", runner.calls)
	}
}
