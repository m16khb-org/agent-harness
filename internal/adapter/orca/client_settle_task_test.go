package orca

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"agent-harness/internal/port"
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

func TestClientSettleTaskRejectsMissingRunIdentityBeforeInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	err := NewClient(runner).SettleTask(context.Background(), "", "task-130")
	var typed *port.OrcaError
	if !errors.As(err, &typed) || typed.Code != "run_identity_invalid" || typed.Invoked || len(runner.calls) != 0 {
		t.Fatalf("missing Run identity error=%v calls=%#v", err, runner.calls)
	}
}
