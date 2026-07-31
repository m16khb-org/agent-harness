package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/issueops/model"
)

// revoke는 응답 없는 홀더에게서 제3자가 lease를 뺏는 경로다. 그런데 요청자가
// 홀더인지 보지 않아 홀더 자신이 호출할 수 있고, 그러면 나갈 문이 전부 막힌다:
// release는 active를, reseed는 released/claimable을, finalize는 홀더 dead를,
// claim은 claimable을 요구한다. 그 세션이 죽어야만 풀린다(이슈 #170).
//
// 이 세션에서 실제로 겪었고 Claude Code 재시작을 강제당했다.
func TestExecutionRevokeRefusesTheLiveHolderItself(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-self-revoke")
	holder := executionActor("claude", "self-revoke-session")
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: holder,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: holder, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "switching modes", Confirm: true,
		Actor: holder, CWD: fixture.record.Repo,
	})
	if err == nil {
		t.Fatal("살아 있는 홀더의 자기-revoke는 나갈 문이 없는 상태를 만든다")
	}
	if !strings.Contains(err.Error(), "release") {
		t.Fatalf("거부만 하고 해소 경로를 주지 않으면 사용자가 갇힌다: %v", err)
	}
	// 거부는 상태를 바꾸지 않는다 — 여전히 홀더가 release할 수 있어야 한다.
	if _, err := ReleaseExecution(stateRoot, ExecutionReleaseRequest{
		ID: fixture.record.ID, Generation: 1, Actor: holder, CWD: fixture.worktree,
	}); err != nil {
		t.Fatalf("거부 뒤에도 정상 반납 경로가 살아 있어야 한다: %v", err)
	}
}

// 죽은 홀더를 제3자가 뺏는 것이 revoke의 존재 이유다. 자기-revoke를 막는 것이
// 그 경로까지 막으면 crash 복구가 불가능해진다.
func TestExecutionRevokeStillTakesOverADeadHolder(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-dead-takeover")
	activateExecutionFixtureWithDeadHolder(t, stateRoot, &fixture)
	requester := executionActor("claude", "takeover-session")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "holder crashed", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatalf("죽은 홀더를 뺏는 경로가 막히면 crash 복구가 불가능해진다: %v", err)
	}
	if result.Execution.Lease.Status != model.LeaseStatusRevoking {
		t.Fatalf("revoke는 revoking으로 전이해야 한다: %+v", result.Execution)
	}
}

// 제3자가 살아 있는 홀더에게서 뺏는 것도 그대로다. 홀더가 응답하지 않지만
// 프로세스는 살아 있는 경우가 여기에 해당한다.
func TestExecutionRevokeStillTakesOverFromAnotherSession(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-third-party")
	holder := executionActor("codex", "unresponsive-session")
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: holder,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	requester := executionActor("claude", "third-party-session")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner is unresponsive", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	}); err != nil {
		t.Fatalf("제3자 revoke는 그대로 동작해야 한다: %v", err)
	}
}

// prepare는 워크스페이스 준비 명령이고 lease 획득은 claim의 일이다. 그런데
// 준비된 실행에 lease writer가 없는 상태에서 prepare가 ok:true를 주면 호출자는
// lease를 잡았다고 믿고, 다음 mutation이 write_lease_required로 막히고서야
// 드러난다(이슈 #170).
func TestPrepareDoesNotReportSuccessWithoutAWriter(t *testing.T) {
	for _, status := range []model.LeaseStatus{model.LeaseStatusReleased, model.LeaseStatusClaimable} {
		t.Run(string(status), func(t *testing.T) {
			stateRoot, record := preparedExecutionWithLeaseStatus(t, status)
			result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
				Actor: executionActor("claude", "reclaim-session"), OwnerHost: "claude",
			}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
			if err == nil {
				t.Fatalf("writer 없는 lease에 ok:true를 주면 거짓 성공이다: %+v", result)
			}
			if result.OK {
				t.Fatalf("writer가 없으면 성공이 아니다: %+v", result)
			}
			if strings.TrimSpace(result.NextCommand) == "" {
				t.Fatalf("거부만 하고 해소 경로를 주지 않으면 사용자가 갇힌다: %+v", result)
			}
		})
	}
}

func TestReleasedDirectRecoveryRendersFiniteCommandChain(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	actor := executionActor("codex", "released-direct-recovery")
	prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: actor, OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReleaseExecution(stateRoot, ExecutionReleaseRequest{
		ID: record.ID, Generation: prepared.Execution.Lease.Generation,
		Actor: actor, CWD: prepared.Execution.Workspace.Root,
	}); err != nil {
		t.Fatal(err)
	}

	status, err := StatusExecution(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status.NextCommand, "execution replace") ||
		!strings.Contains(status.NextCommand, "--preview") {
		t.Fatalf("released status must start recovery with a replacement preview: %q", status.NextCommand)
	}

	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview,
		ExpectedGeneration: prepared.Execution.Lease.Generation,
		Actor:              actor, CWD: record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"--reseed",
		"--expected-generation 1",
		"--inventory-fingerprint " + preview.InventoryFingerprint,
		"--confirm",
	} {
		if !strings.Contains(preview.NextCommand, fragment) {
			t.Fatalf("replacement preview next command %q does not contain %q", preview.NextCommand, fragment)
		}
	}

	reseeded, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed,
		ExpectedGeneration:   prepared.Execution.Lease.Generation,
		InventoryFingerprint: preview.InventoryFingerprint,
		Reason:               "released direct holder recovery",
		Actor:                actor, CWD: record.Repo, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reseeded.NextCommand, "execution claim") ||
		!strings.Contains(reseeded.NextCommand, reseeded.ClaimTokenPath) {
		t.Fatalf("reseed must hand over the exact claim command: %q", reseeded.NextCommand)
	}
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: reseeded.Execution.Lease.Generation,
		Actor: actor, CWD: prepared.Execution.Workspace.Root,
		TokenFile: reseeded.ClaimTokenPath,
	}); err != nil {
		t.Fatalf("rendered recovery chain did not restore the writer: %v", err)
	}
}

// 홀더가 요청자와 같으면 지금처럼 멱등 성공한다. lease 검사가 그 경로까지
// 막으면 정상 재호출이 실패로 바뀐다.
func TestPrepareStaysIdempotentForTheActiveHolder(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	actor := executionActor("claude", "idempotent-session")
	first, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: actor, OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: actor, OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("같은 홀더의 재호출은 멱등이어야 한다: %v", err)
	}
	if !second.OK || second.Execution == nil || second.Execution.Lease.Generation != first.Execution.Lease.Generation {
		t.Fatalf("멱등 호출이 같은 generation을 돌려줘야 한다: %+v", second.Execution)
	}
}

// 다른 세션이 holder여도 prepare는 성공한다. prepare는 워크스페이스 준비 명령이고
// lease를 잡지 않으므로 홀더가 누구인지는 그 결과를 바꾸지 않는다 — 그 세션이
// 실제로 쓰려 하면 가드와 core의 lease 검사가 막는다.
//
// writer-없음 검사가 이 경로까지 삼키면 안 된다. active에는 writer가 있고,
// "그 writer가 나인가"는 다른 질문이다.
func TestPrepareStaysAvailableWhileAnotherSessionHolds(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: executionActor("claude", "owner-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader}); err != nil {
		t.Fatal(err)
	}
	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: executionActor("claude", "observer-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil || !result.OK {
		t.Fatalf("active lease는 writer가 있으므로 prepare가 막히지 않는다: %v %+v", err, result)
	}
}

// preview는 상태 조회다. writer가 없어도 무엇이 준비돼 있는지 보여줘야 한다 —
// 그러지 않으면 갇힌 상태를 진단할 수단이 사라진다.
func TestPreparePreviewStaysAvailableWithoutAWriter(t *testing.T) {
	stateRoot, record := preparedExecutionWithLeaseStatus(t, model.LeaseStatusReleased)
	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: false,
		Actor: executionActor("claude", "preview-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil || !result.OK {
		t.Fatalf("preview는 writer 유무와 무관하게 상태를 보여줘야 한다: %v %+v", err, result)
	}
}

// preparedExecutionWithLeaseStatus는 direct로 준비를 마친 뒤 lease만 지정한
// writer-없음 상태로 바꾼다. release나 reseed 뒤의 실제 상태다.
func preparedExecutionWithLeaseStatus(t *testing.T, status model.LeaseStatus) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := executionPrepareRecord(t)
	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
		Actor: executionActor("claude", "fixture-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader}); err != nil {
		t.Fatal(err)
	}
	prepared, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	prepared.Execution.Lease.Status = status
	prepared.Execution.Lease.Holder = nil
	if status == model.LeaseStatusClaimable {
		// validateWriteLease가 claimable에 토큰 해시를 강제한다. reseed가 실제로
		// 만드는 모양이다.
		prepared.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	} else {
		prepared.Execution.Lease.ClaimTokenSHA256 = ""
	}
	written, err := WriteIssueOps(stateRoot, prepared)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}
