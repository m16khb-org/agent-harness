package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

// AC-11a: 이원 구조 배관 E2E(시뮬레이션) — 메인(planner) 세션이 두 사이클을
// 병렬로 스폰 준비(각각 claimable, 자신은 lease 미보유)하고, 그 상태에서
// 세 번째 사이클을 자유롭게 계획하며, 머지 후 finish가 레코드를 삭제해
// 동일 브랜치 재작업이 새 사이클로 열리는 전 과정을 하나의 상태 저장소에서
// 검증한다. 가드 정상성(released deny·삭제 후 비차단)은
// TestCleanupFinishResumableConvergesAndRecordDeleted와 hookcli 회귀가 고정한다.
func TestDualStructurePipelineEndToEnd(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")

	prepareOrcaCycle := func(branch string) IssueOpsRecord {
		t.Helper()
		record := executionPrepareRecordForBranch(t, stateRoot, branch)
		fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
		fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
			if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
				return port.ExecutionOrcaWorkspaceReceipt{}, err
			}
			return executionOrcaWorkspaceReceipt(workspace), nil
		}
		got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
			ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
			Actor: executionActor("claude", "planner-session"), OwnerHost: "claude",
		}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
		if err != nil {
			t.Fatal(err)
		}
		if got.Execution.Lease.Status != model.LeaseStatusClaimable || got.Execution.Lease.Holder != nil {
			t.Fatalf("handoff must leave a claimable lease with no planner holder: %+v", got.Execution.Lease)
		}
		if got.Execution.Orca.OwnerModel != port.IssueOpsImplementerModelClaude {
			t.Fatalf("implementer defaults must ride the handoff: %+v", got.Execution.Orca)
		}
		updated, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		return updated
	}

	// 1) 두 사이클 병렬 스폰 준비 — 워크스페이스가 서로 격리되고 둘 다 claimable.
	cycleA := prepareOrcaCycle("101-parallel-a")
	cycleB := prepareOrcaCycle("102-parallel-b")
	if cycleA.Execution.Workspace.Root == cycleB.Execution.Workspace.Root {
		t.Fatalf("parallel cycles must own distinct canonical worktrees")
	}

	// 2) 두 사이클이 claimable인 동안에도 메인 세션은 세 번째 계획을 자유롭게
	// 시작한다 — planner는 lease를 보유하지 않으므로 어떤 것에도 묶이지 않는다.
	third, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: cycleA.Repo, Branch: "103-parallel-c"})
	if err != nil {
		t.Fatalf("planner must stay free to plan while handoffs are claimable: %v", err)
	}
	if _, err := RecordIssueOpsIntent(stateRoot, third.ID, IssueOpsIntentRecordRequest{
		RawRequest: "세 번째 계획", InterpretedIntent: "병렬 계획 자유 검증", SuccessCriteria: []string{"list가 3사이클을 조망"},
	}); err != nil {
		t.Fatal(err)
	}

	// 3) 집계 표면이 세 사이클을 한 번에 조망한다.
	listed, err := ListIssueOpsCycles(stateRoot, cycleA.Repo)
	if err != nil || len(listed.Entries) != 3 {
		t.Fatalf("list must aggregate all three cycles: %v %+v", err, listed.Entries)
	}

	// 4) 사이클 A의 머지 후 정리: 하위 세션 종료(released) 상태를 만들고
	// finish가 레코드를 삭제한다. 게이트 입력은 CLI가 원격 readback으로
	// 검증하는 값이므로 여기서는 검증 완료 상태를 주입한다.
	mutateFinishRecord(t, stateRoot, cycleA.ID, func(rec *IssueOpsRecord) {
		rec.Phase = model.IssueOpsPhaseDone
		rec.RemoteArtifact = &IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/101"}
		rec.Execution.Lease = WriteLease{Generation: 1, Status: model.LeaseStatusReleased}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	orcaRemoved := false
	deps.RemoveOrcaWorktree = func(context.Context, string) error { orcaRemoved = true; return nil }
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(cycleA.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	final, err := CleanupFinish(context.Background(), stateRoot, finishRequest(cycleA.ID, true, preview.Fingerprint), deps)
	if err != nil || !final.RecordDeleted || !orcaRemoved {
		t.Fatalf("finish must reclaim orca then delete the record: %v %+v", err, final)
	}

	// 5) 사이클 B는 영향 없이 claimable로 남고, A의 브랜치는 새 사이클로 열린다.
	remaining, err := ReadIssueOps(stateRoot, cycleB.ID)
	if err != nil || remaining.Execution.Lease.Status != model.LeaseStatusClaimable {
		t.Fatalf("parallel cycle B must stay untouched: %v %+v", err, remaining.Execution)
	}
	reborn, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: cycleA.Repo, Branch: "101-parallel-a"})
	if err != nil || reborn.Phase != model.IssueOpsPhaseProblem || reborn.Execution != nil {
		t.Fatalf("finish must unlock same-branch rework: %v %+v", err, reborn)
	}
}

// executionPrepareRecordForBranch는 executionPrepareRecord와 같은 준비 상태를
// 임의 브랜치·공유 state root로 만든다(E2E에서 사이클 3개가 한 저장소를 공유).
func executionPrepareRecordForBranch(t *testing.T, stateRoot, branch string) IssueOpsRecord {
	t.Helper()
	base := executionPrepareRecord
	_ = base
	repo := sharedDualE2ERepo(t)
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/acme/repo/issues/" + strings.SplitN(branch, "-", 2)[0]
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.IssueURL = issueURL
		rec.BranchPrepare = &IssueOpsBranchPrepare{
			Provider: "github", IssueURL: issueURL, Branch: branch,
			BaseBranch: "main", BaseSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", LinkVerified: true,
			CreatedAt: "2026-07-24T00:00:00Z",
		}
	})
	record, err = ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

var sharedDualE2ERepoPath string

func sharedDualE2ERepo(t *testing.T) string {
	t.Helper()
	if sharedDualE2ERepoPath == "" {
		sharedDualE2ERepoPath = t.TempDir()
	}
	return sharedDualE2ERepoPath
}
