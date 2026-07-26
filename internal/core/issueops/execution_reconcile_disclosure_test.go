package issueops

import (
	"context"
	"testing"

	"agent-harness/internal/port"
)

// pendingOrcaIntentFixture는 외부 mutation이 모호하게 끝나 pending intent가 남은
// 상태를 만든다. Invoked:true인 실패가 그 상태의 정식 입구다.
func pendingOrcaIntentFixture(t *testing.T) (string, IssueOpsRecord, *executionOrcaFake) {
	t.Helper()
	stateRoot, record := orcaPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true}
	}
	_, _ = PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "disclosure-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})

	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution == nil || pending.Execution.Pending == nil {
		t.Fatalf("픽스처가 pending intent를 남기지 못했다: %#v", pending.Execution)
	}
	return stateRoot, record, fake
}

// reconcile --preview는 pending intent의 kind만 보고 상수 코드를 돌려준다. orca를
// 조회하지 않는다. 그런데 출력에 그 구분이 없어서, preview 결과를 "orca 자원이
// 이런 상태다"라는 관측 증거로 읽게 된다.
//
// #99의 "auto 모드 preview가 orca worktree를 실생성한다"는 의혹이 이 오독에서
// 나왔다. 시스템은 자기가 무엇을 확인했고 무엇을 확인하지 않았는지 알면서
// 말하지 않는다(이슈 #154).
func TestReconcilePreviewDeclaresItDidNotInspectOrca(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	inspectCalls := 0
	fake.inspect = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		inspectCalls++
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
	}

	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Preview: true, CWD: record.Repo,
		Actor: executionActor("codex", "disclosure-session"),
	}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}

	if inspectCalls != 0 {
		t.Fatalf("preview가 실제로 orca를 조회하면 이 계약 자체가 다른 문제다: inspectCalls=%d", inspectCalls)
	}
	if result.ExternalStateInspected {
		t.Fatal("preview는 orca를 조회하지 않는다. 조회했다고 표시하면 결과가 관측 증거로 오독된다")
	}
}

// confirm 경로는 실제로 조회하므로 그렇게 표시한다. 이 구분이 있어야 결과를
// 증거로 쓸 수 있는지 판단할 수 있다.
func TestReconcileConfirmDeclaresItInspectedOrca(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{
			Candidates: []port.ExecutionOrcaIntentReceipt{successfulExecutionOrcaIntentReceipt(t, request)},
		}, nil
	}

	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, CWD: record.Repo,
		Actor: executionActor("codex", "disclosure-session"),
	}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ExternalStateInspected {
		t.Fatalf("confirm은 orca를 실제로 조회하므로 그렇게 표시해야 한다: %+v", result)
	}
}
