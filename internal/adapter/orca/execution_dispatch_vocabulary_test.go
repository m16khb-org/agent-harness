package orca

import (
	"context"
	"testing"

	"issueops/internal/port"
)

func dispatchVocabularyRequest() port.ExecutionOrcaOwnerInventoryRequest {
	return port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	}
}

// dispatch 상태는 liveness 판정의 입력이 아니다.
//
// 그 어휘를 확인할 방법이 없어서다. orca CLI에는 dispatch 상태를 설정하는 명령이
// 없고, dispatch-show --help도 상태 목록을 내지 않는다(2026-07-26 재확인).
// 그래서 task 어휘를 빌려 쓰고 있었는데, 그 차용이 옳은지 물어볼 곳이 없다.
//
// 여기서 고정하는 것은 "빌려 쓰지 않는다"이다. 종결된 task와 죽은 terminal 위에
// 남은 dispatch 행은 소유자를 잃을 worker가 아니므로 abandon을 막을 이유가 없다.
// DispatchStatus는 계속 보고한다 — 게이트 판정에서 빠질 뿐 진단에는 필요하다.
func TestExecutionOwnerInventoryDoesNotDeriveLivenessFromDispatchStatus(t *testing.T) {
	for _, status := range []string{"dispatched", "pending", "ready", "blocked", "running", "queued", ""} {
		t.Run("dispatch/"+status, func(t *testing.T) {
			client := &executionFake{
				tasks:    []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
				dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: status},
			}
			got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest())
			if err != nil {
				t.Fatal(err)
			}
			if got.TaskLive {
				t.Fatalf("dispatch status %q must not decide liveness; the task is completed and no terminal is live: %#v", status, got)
			}
			if got.DispatchStatus != status {
				t.Fatalf("dispatch status must still be reported for diagnosis: got %q want %q", got.DispatchStatus, status)
			}
		})
	}
}

// AC-03 — 게이트의 fail-closed 성질은 dispatch 어휘가 아니라 확인된 두 증거에서
// 온다: task 어휘와 terminal의 connected/writable 관측. 그 둘은 그대로 막는다.
func TestExecutionOwnerInventoryKeepsVerifiedLivenessSignals(t *testing.T) {
	t.Run("live terminal blocks even when task and dispatch are terminal", func(t *testing.T) {
		client := &executionFake{
			terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
			tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
			dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "completed"},
		}
		got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest())
		if err != nil || !got.TerminalLive {
			t.Fatalf("a connected writable terminal must keep the owner non-quiescent: got=%#v err=%v", got, err)
		}
	})

	for _, status := range []string{"dispatched", "pending", "ready", "blocked", "running", ""} {
		t.Run("live task status/"+status, func(t *testing.T) {
			client := &executionFake{
				tasks:    []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: status}},
				dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "completed"},
			}
			got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest())
			if err != nil {
				t.Fatal(err)
			}
			if !got.TaskLive {
				t.Fatalf("task status %q is outside the terminal vocabulary and must stay live: %#v", status, got)
			}
		})
	}
}

// dispatch 행 자체의 관측은 계속 fail-closed다. 상태 어휘를 판정에서 뺀 것이지
// dispatch 조회를 뺀 것이 아니다 — 여기가 무너지면 A 결정이 검출력이 아니라
// 안전성을 깎은 것이 된다.
func TestExecutionOwnerInventoryKeepsDispatchObservationFailClosed(t *testing.T) {
	t.Run("dispatch absent while the task is present", func(t *testing.T) {
		client := &executionFake{tasks: []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}}}
		if got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest()); err == nil {
			t.Fatalf("a task row without its dispatch row is ambiguous and must fail closed: %#v", got)
		}
	})

	// 거울상도 같은 모순이다. 상태 어휘를 판정에서 뺀 뒤로는 dispatch 상태가
	// 이 조합을 우연히 막아주지 않으므로, 행의 존재만으로 막아야 한다.
	for _, status := range []string{"dispatched", "completed", "failed", ""} {
		t.Run("task absent while the dispatch row remains/"+status, func(t *testing.T) {
			client := &executionFake{
				dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: status},
			}
			if got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest()); err == nil {
				t.Fatalf("a dispatch row whose task vanished is ambiguous and must fail closed: %#v", got)
			}
		})
	}

	for _, tc := range []struct {
		name     string
		dispatch port.OrcaDispatch
	}{
		{name: "dispatch id changed", dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-other", TaskID: "task-69", Status: "completed"}},
		{name: "dispatch task binding changed", dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-other", Status: "completed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionFake{
				tasks:    []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
				dispatch: tc.dispatch,
			}
			if got, err := NewExecutionClient(client).InspectOwner(context.Background(), dispatchVocabularyRequest()); err == nil {
				t.Fatalf("dispatch identity drift must fail closed: %#v", got)
			}
		})
	}
}
