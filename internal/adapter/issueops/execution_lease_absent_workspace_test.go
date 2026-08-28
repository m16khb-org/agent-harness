package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueops "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// TestWorkspaceSnapshotAcceptsAnAbsentWorktree는 #435를 고정한다.
//
// canonical worktree가 없어진 상태에서 lease가 active로 남으면 회수 경로가
// 전부 닫혀 있었다. replace는 worktree 실재를 요구하고, abandon은 lease가
// terminal이기를 요구하며, terminal로 만들려면 replace가 필요하다.
//
// 그런데 부재는 quiescence의 **약한** 증거가 아니라 가장 강한 증거다.
// 존재하지 않는 디렉터리에는 프로세스가 cwd를 둘 수 없고 쓸 것도 없다.
// snapshot의 목적이 "replace 도중 워크스페이스가 움직이지 않았음"을 증명하는
// 것이므로, 부재도 봉인 가능한 관측이어야 한다.
//
// 실측: io-2ffbd9a6739c와 io-c26802f00c2b가 holder 프로세스도 worktree도 없이
// lease만 active로 남아 영구히 회수 불가였다.
func TestWorkspaceSnapshotAcceptsAnAbsentWorktree(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "reclaimed-worktree")
	workspace := issueops.Workspace{Root: absent, Branch: "293-cleanup", BaseHead: "abc123"}

	got, err := workspaceSnapshot(workspace)
	if err != nil {
		t.Fatalf("부재한 worktree는 봉인 가능한 관측이어야 한다: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("부재도 fingerprint를 내야 한다")
	}

	// 부재라는 사실이 결속돼야 한다. 같은 경로가 되살아나면 다른 값이 나와
	// finalize가 stale로 멈춘다.
	if err := os.MkdirAll(absent, 0o755); err != nil {
		t.Fatal(err)
	}
	revived, reviveErr := workspaceSnapshot(workspace)
	if reviveErr == nil && revived == got {
		t.Fatal("worktree가 되살아나면 fingerprint가 달라져야 한다")
	}
}

// TestWorkspaceSnapshotStillRefusesAnUnidentifiedPath는 완화가 "부재"에만
// 적용됨을 고정한다. symlink나 파일이 그 경로를 차지한 것은 부재가 아니라
// 정체 불명이며, 그것을 quiescence로 인정하면 안 된다.
func TestWorkspaceSnapshotStillRefusesAnUnidentifiedPath(t *testing.T) {
	root := t.TempDir()

	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceSnapshot(issueops.Workspace{Root: file, Branch: "b"}); err == nil {
		t.Fatal("일반 파일이 차지한 경로는 계속 거부해야 한다")
	}

	link := filepath.Join(root, "symlinked")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("이 환경은 symlink를 만들 수 없다: %v", err)
	}
	if _, err := workspaceSnapshot(issueops.Workspace{Root: link, Branch: "b"}); err == nil {
		t.Fatal("symlink는 계속 거부해야 한다")
	}
}

// TestWorkspaceSnapshotAbsenceIsBoundToTheExactPath는 서로 다른 부재가 같은
// fingerprint를 내지 않음을 고정한다. 그러지 않으면 다른 lifecycle의 부재
// 증거를 재사용할 수 있다.
func TestWorkspaceSnapshotAbsenceIsBoundToTheExactPath(t *testing.T) {
	base := t.TempDir()
	first, err := workspaceSnapshot(issueops.Workspace{Root: filepath.Join(base, "one"), Branch: "b"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := workspaceSnapshot(issueops.Workspace{Root: filepath.Join(base, "two"), Branch: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("부재 증거는 정확한 경로에 결속돼야 한다")
	}
}

// TestReplacementResidueCleanupSkipsAnAbsentWorkspace는 #435의 두 번째 층을
// 고정한다.
//
// 잔여 파일 제거가 그 파일의 부모 디렉터리를 **만들려** 한다. 존재하는
// worktree에서는 무해하지만, worktree가 통째로 없으면 지울 것도 없는데
// 디렉터리를 만들다 실패해 finalize가 멈춘다.
//
// 실측: io-2ffbd9a6739c의 finalize가
// "clean uncommitted replacement residue .../lease-3.token: mkdir ..."로
// 실패해 lease가 revoking에 갇혔다. abandon은 claimable/released만 받으므로
// 그 지점에서 다시 막다른 길이 된다.
func TestReplacementResidueCleanupSkipsAnAbsentWorkspace(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "reclaimed-worktree")
	if err := removeReplacementRuntimeFile(absent, filepath.Join(absent, ".agent-harness", "lease-3.token")); err != nil {
		t.Fatalf("없는 worktree에는 지울 잔여물도 없다: %v", err)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("정리가 worktree를 되살리면 안 된다")
	}
}

// TestReplacementResidueCleanupStillRemovesRealFiles는 완화가 실제 정리를
// 건너뛰지 않음을 고정한다.
func TestReplacementResidueCleanupStillRemovesRealFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".agent-harness", "lease-3.token")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeReplacementRuntimeFile(root, target); err != nil {
		t.Fatalf("실재하는 잔여물은 계속 지워야 한다: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("잔여물이 남았다")
	}
}

// TestFinalizeReleasesInsteadOfClaimableWhenTheWorkspaceIsGone은 #435의 세
// 번째 층을 고정한다.
//
// finalize는 claim token을 만들고 owner context를 재봉인한다. 둘 다 canonical
// worktree에 쓴다. worktree가 없으면 그 쓰기가 실패해 lease가 revoking에
// 갇히고, abandon은 claimable/released만 받으므로 다시 막다른 길이 된다.
//
// 넘겨줄 workspace가 없으면 claimable은 사실이 아니다 — 아무도 claim할 수
// 없다. terminal 상태인 released가 정확하고, 그것이 abandon 경로를 연다.
func TestFinalizeReleasesInsteadOfClaimableWhenTheWorkspaceIsGone(t *testing.T) {
	stateRoot, record := rolloverExecutionFixture(t)
	requester := executionActor("codex", "replacement-owner")
	inspector := &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-sealed"}}
	deps := ExecutionReplaceDependencies{OrcaOwner: inspector, inspectWorkspace: quiescentWorkspaceInspector()}
	source := record.Execution.Workspace.SourceRoot

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: source,
	}, deps)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "worktree was reclaimed elsewhere",
		Actor: requester, CWD: source, Confirm: true,
	}, deps); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// 이 시점에 worktree가 사라진다 — 밖에서 정리됐거나 회수된 상황이다.
	if err := os.RemoveAll(record.Execution.Workspace.Root); err != nil {
		t.Fatal(err)
	}

	finalizePreview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: source,
	}, deps)
	if err != nil {
		t.Fatalf("부재한 worktree에서도 finalize preview는 진행돼야 한다: %v", err)
	}

	finalized, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: finalizePreview.QuiescenceFingerprint,
		Actor:                 requester, CWD: source, Confirm: true,
	}, deps)
	if err != nil {
		t.Fatalf("부재한 worktree에서도 finalize는 진행돼야 한다: %v", err)
	}
	lease := finalized.Execution.Lease
	if lease.Status != issueops.LeaseStatusReleased || lease.Holder != nil {
		t.Fatalf("넘겨줄 workspace가 없으면 released여야 한다: %#v", lease)
	}
	if lease.ClaimTokenSHA256 != "" {
		t.Fatal("claim할 수 없는 lease에 token을 발급하면 안 된다")
	}
}
