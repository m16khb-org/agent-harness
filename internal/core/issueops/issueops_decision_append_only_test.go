package issueops

import (
	"context"
	"reflect"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/issueops/model"
)

// decision add를 owner mutation allowlist에 넣는 근거는 그것이 append-only라는 것이다
// (#158). phase나 lease나 execution을 건드리는 명령이라면 넣지 않았을 것이므로, 그
// 전제를 테스트로 고정한다 — 나중에 이 함수가 다른 필드를 만지게 되면 여기서 걸린다.
func TestDecisionAddTouchesOnlyTheDecisionList(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	before, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}

	after, err := AddIssueOpsDecision(stateRoot, record.ID, IssueOpsDecisionRecordRequest{
		Title: "계약 변경", Body: "근거", Kind: "architecture",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(after.Decisions) != len(before.Decisions)+1 {
		t.Fatalf("결정 하나가 append되어야 한다: before=%d after=%d", len(before.Decisions), len(after.Decisions))
	}
	if !reflect.DeepEqual(before.Execution, after.Execution) {
		t.Fatalf("execution을 건드리면 allowlist에 넣을 근거가 무너진다: before=%#v after=%#v", before.Execution, after.Execution)
	}
	if after.Phase != before.Phase {
		t.Fatalf("phase가 바뀌면 안 된다: %q → %q", before.Phase, after.Phase)
	}
	// 기존 결정은 보존된다 — append이지 덮어쓰기가 아니다.
	for i := range before.Decisions {
		if !reflect.DeepEqual(before.Decisions[i], after.Decisions[i]) {
			t.Fatalf("기존 결정 %d가 바뀌었다: %#v → %#v", i, before.Decisions[i], after.Decisions[i])
		}
	}
}

// lease가 active인 레코드에서도 append-only 계약이 같다. allowlist 추가가 lease
// 상태에 따라 다르게 동작하면 안 된다.
func TestDecisionAddKeepsActiveLeaseUntouched(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "decision-session"),
	}, ExecutionPrepareDependencies{Direct: gitworktree.New()})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Execution == nil || prepared.Execution.Lease.Status != model.LeaseStatusActive {
		t.Fatalf("픽스처가 active lease를 가져야 한다: %#v", prepared.Execution)
	}
	before, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}

	// lease가 active면 actor가 필수다 — allowlist가 요구하는 규율과 같다.
	holder := prepared.Execution.Lease.Holder
	after, err := AddIssueOpsDecisionWithActor(stateRoot, record.ID, IssueOpsDecisionRecordRequest{
		Title: "구현 중 결정", Body: "근거", Kind: "implementation",
	}, IssueOpsActor{
		Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID,
		CWD:                   prepared.Execution.Workspace.Root,
		NativeProcessAncestry: []model.NativeProcessReceipt{*holder.SessionProcess},
	})
	if err != nil {
		t.Fatal(err)
	}
	if before.Execution == nil || after.Execution == nil {
		t.Fatalf("픽스처가 execution을 가져야 한다: before=%#v after=%#v", before.Execution, after.Execution)
	}
	if !reflect.DeepEqual(before.Execution.Lease, after.Execution.Lease) {
		t.Fatalf("lease를 건드리면 안 된다: before=%#v after=%#v", before.Execution.Lease, after.Execution.Lease)
	}
}
