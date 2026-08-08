package orca

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

const (
	fencedRunID  = "run_c137f557d022"
	fencedTaskID = "task_8971e683a744"
)

func fencedUpdateArgv() []string {
	return []string{"orca", "orchestration", "task-update", "--id", fencedTaskID, "--status", "completed", "--run", fencedRunID, "--json"}
}

func fencedBindArgv() []string {
	return []string{"orca", "orchestration", "run-use", "--id", fencedRunID, "--json"}
}

// TestClientSettleTaskRebindsTheSealedRunWhenFenced는 #325를 고정한다.
//
// Orca는 task mutation을 Run 단위로 격리하고 호출 terminal이 그 Run의 current
// consumer인지 인증한다. coordinator가 다른 Run에 바인딩돼 있으면 completion의
// task settle이 consumer_fenced로 실패하고, durable lifecycle completion이
// 성공한 뒤에도 Orca task가 열린 채 남았다.
//
// 실측(relay 0.1.0+66c426c5173c):
//
//	task-update --run A  → consumer_fenced ("bound to B, not A")
//	run-use --id A       → ok
//	task-update --run A  → ok
func TestClientSettleTaskRebindsTheSealedRunWhenFenced(t *testing.T) {
	runner := newFakeRunner(t)
	// bind가 authority를 회복하는 것을 재현한다. bind 전의 update는 fence,
	// 그 뒤의 update는 성공이어야 한다 — map 기반 fake로는 그 순서를 표현할
	// 수 없으므로 bind 호출을 관측해 응답을 바꾼다.
	update := strings.Join(fencedUpdateArgv(), " ")
	bind := strings.Join(fencedBindArgv(), " ")
	runner.responses[update] = CommandOutput{Invoked: true, Stdout: []byte(
		`{"ok":false,"error":{"code":"consumer_fenced","message":"This coordinator terminal is bound to run_other, not ` + fencedRunID + `."}}`)}
	runner.responses[bind] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{}}`)}
	rebinding := &rebindingRunner{inner: runner, bindKey: bind, updateKey: update,
		afterBind: CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{}}`)}}

	if err := NewClient(rebinding).SettleTask(context.Background(), fencedRunID, fencedTaskID); err != nil {
		t.Fatalf("sealed Run에 다시 바인딩한 뒤에는 수렴해야 한다: %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("update → bind → update 정확히 세 번이어야 한다: %#v", runner.calls)
	}
	if !slices.Equal(runner.calls[0], fencedUpdateArgv()) || !slices.Equal(runner.calls[1], fencedBindArgv()) || !slices.Equal(runner.calls[2], fencedUpdateArgv()) {
		t.Fatalf("호출 순서가 계약과 다르다: %#v", runner.calls)
	}
}

// TestClientSettleTaskRetriesTheRebindExactlyOnce는 재시도가 한 번뿐임을
// 고정한다. 반복하면 fence가 풀리지 않는 상황에서 무한히 매달린다.
func TestClientSettleTaskRetriesTheRebindExactlyOnce(t *testing.T) {
	runner := newFakeRunner(t)
	fenced := `{"ok":false,"error":{"code":"consumer_fenced","message":"still fenced"}}`
	runner.responses[strings.Join(fencedUpdateArgv(), " ")] = CommandOutput{Invoked: true, Stdout: []byte(fenced)}
	runner.responses[strings.Join(fencedBindArgv(), " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{}}`)}

	err := NewClient(runner).SettleTask(context.Background(), fencedRunID, fencedTaskID)
	if err == nil {
		t.Fatal("바인딩 후에도 fence가 남으면 실패로 보고해야 한다")
	}
	if len(runner.calls) != 3 {
		t.Fatalf("재시도는 정확히 한 번이어야 한다: %d회 호출 %#v", len(runner.calls), runner.calls)
	}
}

// TestClientSettleTaskDoesNotRebindOnUnrelatedFailures는 rebind가 fence
// 전용임을 고정한다. 다른 오류에 바인딩을 바꾸면 부작용만 남는다.
func TestClientSettleTaskDoesNotRebindOnUnrelatedFailures(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses[strings.Join(fencedUpdateArgv(), " ")] = CommandOutput{Invoked: true, Stdout: []byte(
		`{"ok":false,"error":{"code":"task_not_found","message":"no such task"}}`)}

	err := NewClient(runner).SettleTask(context.Background(), fencedRunID, fencedTaskID)
	if err == nil {
		t.Fatal("무관한 오류는 그대로 보고해야 한다")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("fence가 아니면 바인딩을 건드리지 않아야 한다: %#v", runner.calls)
	}
}

// TestClientSettleTaskReportsTheOriginalFenceWhenRebindFails는 바인딩 자체가
// 실패하면 원래 fence 오류를 잃지 않음을 고정한다.
func TestClientSettleTaskReportsTheOriginalFenceWhenRebindFails(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses[strings.Join(fencedUpdateArgv(), " ")] = CommandOutput{Invoked: true, Stdout: []byte(
		`{"ok":false,"error":{"code":"consumer_fenced","message":"bound elsewhere"}}`)}
	runner.responses[strings.Join(fencedBindArgv(), " ")] = CommandOutput{Invoked: true, Stdout: []byte(
		`{"ok":false,"error":{"code":"run_not_found","message":"no such run"}}`)}

	err := NewClient(runner).SettleTask(context.Background(), fencedRunID, fencedTaskID)
	var typed *port.OrcaError
	if !errors.As(err, &typed) || typed.Code != "consumer_fenced" {
		t.Fatalf("바인딩 실패는 원래 fence 진단을 가리면 안 된다: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("바인딩이 실패하면 재시도하지 않아야 한다: %#v", runner.calls)
	}
}

// rebindingRunner는 bind 호출 이후의 task-update 응답을 바꿔, 바인딩이
// authority를 회복하는 실제 동작을 재현한다.
type rebindingRunner struct {
	inner     *fakeRunner
	bindKey   string
	updateKey string
	afterBind CommandOutput
	bound     bool
}

func (r *rebindingRunner) LookPath(file string) (string, error) { return r.inner.LookPath(file) }

func (r *rebindingRunner) Run(ctx context.Context, cwd string, timeout time.Duration, argv []string) (CommandOutput, error) {
	key := strings.Join(argv, " ")
	if r.bound && key == r.updateKey {
		r.inner.calls = append(r.inner.calls, append([]string(nil), argv...))
		return r.afterBind, nil
	}
	output, err := r.inner.Run(ctx, cwd, timeout, argv)
	if key == r.bindKey && err == nil {
		r.bound = true
	}
	return output, err
}
