package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

// preparedDirectExecutionRecord는 direct로 실제 준비를 마친 lifecycle을 만든다.
// 워크트리가 디스크에 존재하고 execution record가 durable state에 있다 — 모드
// 전환이 마주하는 실제 상태다. 픽스처가 record만 손으로 채우면 워크스페이스
// 정리 게이트가 검증되지 않는다(#149·#154가 그렇게 뚫렸다).
//
// leaseStatus는 게이트가 상태 이름이 아니라 writer 유무로 판정하는지 보기 위해
// 호출자가 정한다.
func preparedDirectExecutionRecord(t *testing.T, leaseStatus issueops.LeaseStatus) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot, record := executionPrepareRecord(t)

	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: executionActor("claude", "fixture-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader}); err != nil {
		t.Fatalf("direct 준비 픽스처: %v", err)
	}

	prepared, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	prepared.Execution.Lease.Status = leaseStatus
	switch leaseStatus {
	case issueops.LeaseStatusClaimable:
		// validateWriteLease는 claimable에 홀더 부재와 토큰 해시 하나를 강제한다.
		// replace --reseed가 실제로 만드는 모양이다.
		prepared.Execution.Lease.Holder = nil
		prepared.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	case issueops.LeaseStatusReleased:
		prepared.Execution.Lease.Holder = nil
		prepared.Execution.Lease.ClaimTokenSHA256 = ""
	}
	written, err := WriteIssueOps(stateRoot, prepared)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}

// dropRemoteBranchFixture는 브랜치 이름이 어디에도 없는 상태를 만든다.
// executionPrepareRecord가 `refs/remotes/origin/<branch>`를 만들어 두는데 그것이
// IssueOps 정식 순서를 따랐을 때의 실제 상태이기 때문이다. orca 전환은 그
// 이름이 비어 있어야 성립하므로(#149·#154), orca 경로의 다른 계약을 보는
// 테스트는 여기서 그 ref를 지운다.
func dropRemoteBranchFixture(t *testing.T, repo, branch string) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(repo, "update-ref", "-d", "refs/remotes/origin/"+branch); code != 0 {
		t.Fatalf("drop the remote fixture ref: %s", stderr)
	}
}
