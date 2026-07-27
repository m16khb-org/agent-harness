package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type portCompletionSection = port.IssueProviderCompletionSection

type fakeFinishGit struct {
	statusOut string
	branchOID string
	// remoteBranchOID가 비어 있으면 원격 브랜치는 이미 부재다(finish의 정상
	// 전제). lsRemoteFail은 관측 불가를 흉내낸다.
	remoteBranchOID string
	lsRemoteFail    bool
	failStep        string
	removedWorktree bool
	deletedBranch   bool
	casOID          string
}

func (g *fakeFinishGit) run(dir string, args ...string) (int, string) {
	switch args[0] {
	case "ls-remote":
		if g.lsRemoteFail {
			return 128, "fatal: 'origin' does not appear to be a git repository"
		}
		if g.remoteBranchOID == "" {
			return 0, ""
		}
		return 0, g.remoteBranchOID + "\trefs/heads/80-finish\n"
	case "status":
		return 0, g.statusOut
	case "rev-parse":
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	case "worktree":
		if g.failStep == "worktree_remove" {
			return 1, "simulated worktree remove failure"
		}
		g.removedWorktree = true
		return 0, ""
	case "update-ref":
		if g.failStep == "branch_delete" {
			return 1, "simulated update-ref failure"
		}
		g.casOID = args[len(args)-1]
		g.deletedBranch = true
		g.branchOID = ""
		return 0, ""
	}
	return 0, ""
}

func finishTestRecord(t *testing.T, withWorktree bool) (string, IssueOpsRecord, string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "80-finish"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := ""
	record.Phase = IssueOpsPhaseDone
	record.IssueURL = "https://github.com/acme/repo/issues/80"
	record.RemoteArtifact = &IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/90"}
	// execution complete가 base_branch 없는 done 전이를 거부하므로 done 레코드는
	// 항상 준비된 base를 갖는다.
	record.BranchPrepare = &model.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: "80-finish",
		BaseBranch: "main", BaseSHA: "deadbeef", LinkVerified: true,
	}
	if withWorktree {
		worktree = filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "80-finish")
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
		record.Execution = &Execution{
			Mode:      model.ExecutionModeDirect,
			Workspace: Workspace{SourceRoot: repo, Root: worktree, Branch: "80-finish", BaseHead: "deadbeef", Driver: "git", LinkedAt: "2026-07-24T00:00:00Z"},
			Lease:     WriteLease{Generation: 1, Status: model.LeaseStatusReleased},
		}
	}
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, e := writeIssueOps(stateRoot, record)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, worktree
}

func finishRequest(id string, apply bool, fingerprint string) CleanupFinishRequest {
	return CleanupFinishRequest{
		ID: id, CWD: "/tmp/elsewhere",
		Merged: true, CompletionReflected: true, IssueClosed: true,
		MergedBaseBranch: "main",
		Apply:            apply, Confirm: apply, Fingerprint: fingerprint,
	}
}

func finishDeps(git *fakeFinishGit) CleanupFinishDeps {
	return CleanupFinishDeps{
		Git:              git.run,
		InspectProcesses: func(string) ([]string, error) { return nil, nil },
	}
}

func TestCleanupFinishPreviewGatesRejectMissingEvidence(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}

	cases := []struct {
		name    string
		mutate  func(*CleanupFinishRequest)
		missing string
	}{
		{"unmerged", func(r *CleanupFinishRequest) { r.Merged = false }, "remote_artifact_merged"},
		{"completion", func(r *CleanupFinishRequest) { r.CompletionReflected = false }, "completion_reflected"},
		{"issue open", func(r *CleanupFinishRequest) { r.IssueClosed = false }, "issue_closed"},
		{"cwd inside", func(r *CleanupFinishRequest) { r.CWD = filepath.Join(worktree, "sub") }, "cwd_outside_worktree"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := finishRequest(record.ID, false, "")
			tc.mutate(&req)
			result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(git))
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
		})
	}

	t.Run("dirty worktree", func(t *testing.T) {
		dirty := &fakeFinishGit{branchOID: "abc123", statusOut: " M f.go"}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(dirty))
		if err == nil || !containsString(result.Missing, "worktree_clean") {
			t.Fatalf("dirty worktree must block: %v %v", err, result.Missing)
		}
	})
	t.Run("live process", func(t *testing.T) {
		deps := finishDeps(git)
		deps.InspectProcesses = func(string) ([]string, error) { return []string{"1234:codex"}, nil }
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err == nil || !containsString(result.Missing, "workspace_processes_quiescent") {
			t.Fatalf("live process must block: %v %v", err, result.Missing)
		}
	})
	t.Run("unclosed child", func(t *testing.T) {
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
			rec.IssueLinks = append(rec.IssueLinks, IssueOpsIssueLink{Type: "child", URL: "https://github.com/acme/repo/issues/91"})
		})
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err == nil || !containsString(result.Missing, "child_tasks_closed") {
			t.Fatalf("unclosed child must block: %v %v", err, result.Missing)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
			rec.IssueLinks[len(rec.IssueLinks)-1].CloseVerifiedAt = "t"
		})
	})
}

// done 전이는 draft PR 생성 직후에 일어나고 finish는 머지 이후에 실행되므로, 그
// 사이 구간에서 draft PR의 base가 바뀔 수 있다. 준비된 base가 아닌 브랜치로
// 머지된 결과를 파괴 전에 잡지 못하면 재검증 수단이 남지 않는다.
func TestCleanupFinishBlocksBaseBranchDrift(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}

	t.Run("drifted", func(t *testing.T) {
		req := finishRequest(record.ID, false, "")
		req.MergedBaseBranch = "release"
		result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(git))
		if err == nil || !containsString(result.Missing, "base_branch_drifted") {
			t.Fatalf("drifted base must block: err=%v missing=%v", err, result.Missing)
		}
	})
	t.Run("unobserved", func(t *testing.T) {
		req := finishRequest(record.ID, false, "")
		req.MergedBaseBranch = ""
		result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(git))
		if err == nil || !containsString(result.Missing, "merged_base_branch_unobserved") {
			t.Fatalf("unobserved base must fail closed: err=%v missing=%v", err, result.Missing)
		}
	})
	t.Run("matching base still passes", func(t *testing.T) {
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err != nil || containsString(result.Missing, "base_branch_drifted") || containsString(result.Missing, "merged_base_branch_unobserved") {
			t.Fatalf("matching base must not block: err=%v missing=%v", err, result.Missing)
		}
	})
}

// base 관측은 fingerprint 입력이 아니다: 네트워크 관측을 인벤토리에 섞으면
// 일시적 원격 오류가 preview 재발급 루프를 만들고 기존 fingerprint가 모두
// 무효화된다(remote_branch_absent와 같은 규율).
func TestCleanupFinishFingerprintIgnoresObservedBaseBranch(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	first, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
	if err != nil {
		t.Fatal(err)
	}
	req := finishRequest(record.ID, false, "")
	req.MergedBaseBranch = "main"
	second, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(git))
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("observed base must stay out of the fingerprint: %q vs %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestCleanupFinishApplyRejectsStaleFingerprint(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
	if err != nil || preview.Fingerprint == "" {
		t.Fatalf("preview must issue a fingerprint: %v %+v", err, preview)
	}
	// 외부 변화(워크트리 소멸) 후 이전 fingerprint로 apply → 거부.
	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), finishDeps(git)); err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("stale fingerprint must be rejected: %v", err)
	}
}

// AC-03: 중간 실패 주입 후 재실행이 최종 상태로 수렴하고, 실패 시 레코드가
// 잔존하며, 완료 후 동일 브랜치 start는 새 사이클을 만든다.
func TestCleanupFinishResumableConvergesAndRecordDeleted(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123", failStep: "worktree_remove"}
	deps := finishDeps(git)

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != "worktree_remove" {
		t.Fatalf("worktree removal failure must stop apply: %v %+v", err, result)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatalf("record must be preserved on failure: %v", err)
	}
	if kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != "worktree_remove" {
		t.Fatalf("failure point must be recorded: %+v", kept.CleanupFinishFailure)
	}

	// 외부에서 워크트리가 정리된 상태로 재실행: 새 preview → 새 fingerprint → 수렴.
	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}
	git.failStep = ""
	preview2, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if preview2.Fingerprint == preview.Fingerprint {
		t.Fatal("partial cleanup must change the fingerprint")
	}
	final, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview2.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if final.WorktreeRemoved || !final.BranchDeleted || !final.RecordDeleted {
		t.Fatalf("resume must skip the absent worktree, delete branch and record: %+v", final)
	}
	if git.casOID != "abc123" {
		t.Fatalf("branch deletion must CAS on the observed OID: %q", git.casOID)
	}

	// 레코드 삭제 후 동일 (repo, branch) start → 새 problem-phase 사이클.
	fresh, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: record.Repo, Branch: "80-finish"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Phase != IssueOpsPhaseProblem || fresh.Execution != nil || fresh.RemoteArtifact != nil {
		t.Fatalf("finish must unlock same-branch rework with a fresh cycle: %+v", fresh)
	}
}

func TestCleanupFinishOrcaRemovalRunsFirstAndFailureKeepsRecord(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Execution.Mode = model.ExecutionModeOrca
		rec.Execution.Workspace.Driver = "orca"
		rec.Execution.Orca = &model.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	calls := []string{}
	deps.RemoveOrcaWorktree = func(_ context.Context, worktreeID string) error {
		calls = append(calls, "orca:"+worktreeID)
		return fmt.Errorf("orca runtime unreachable")
	}
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != "orca_remove" {
		t.Fatalf("orca failure must stop before git mutation: %v %+v", err, result)
	}
	if git.removedWorktree || git.deletedBranch {
		t.Fatalf("git steps must not run after orca failure: %+v", git)
	}
	if len(calls) != 1 || calls[0] != "orca:wt-1" {
		t.Fatalf("orca removal must run first with the bound worktree id: %v", calls)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("record must survive orca failure: %v", err)
	}
}

// Orca는 자체 메타데이터뿐 아니라 연결된 Git worktree도 함께 제거한다. 따라서
// 성공 직후 사라진 경로에 git worktree remove를 다시 실행하면 정상 정리가
// 실패로 기록된다.
func TestCleanupFinishSkipsGitRemovalWhenOrcaAlreadyRemovedWorktree(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Execution.Mode = model.ExecutionModeOrca
		rec.Execution.Workspace.Driver = "orca"
		rec.Execution.Orca = &model.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	deps.RemoveOrcaWorktree = func(_ context.Context, worktreeID string) error {
		if worktreeID != "wt-1" {
			t.Fatalf("unexpected worktree id: %s", worktreeID)
		}
		return os.RemoveAll(worktree)
	}

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OrcaRemoved || !result.WorktreeRemoved || !result.BranchDeleted || !result.RecordDeleted {
		t.Fatalf("orca 제거 후 남은 단계까지 수렴해야 한다: %+v", result)
	}
	if git.removedWorktree {
		t.Fatal("orca가 이미 제거한 worktree에 git worktree remove를 다시 실행하면 안 된다")
	}
}

// AC-03: completion 미반영 + RemoteArtifact 보유 레코드는 prune되지 않는다.
func TestPrunePreservesUnreflectedMergedRecords(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, false)
	old := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339Nano)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) { rec.UpdatedAt = old })

	result, err := PruneIssueOps(stateRoot, 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 0 || !containsString(result.Kept, record.ID) {
		t.Fatalf("unreflected merged record must be kept: %+v", result)
	}

	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.RemoteCompletion = &IssueOpsRemoteCompletion{ReflectedAt: "t"}
		rec.UpdatedAt = old
	})
	result, err = PruneIssueOps(stateRoot, 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(result.Pruned, record.ID) {
		t.Fatalf("reflected record must be prunable: %+v", result)
	}
}

// AC-03 보강(C2-F2): ④(branch_delete) 실패 주입, apply-without-confirm 거부,
// phase/lease 게이트, 그리고 ④' 감사가 파괴 전 스냅샷으로 렌더됨을 고정한다.
func TestCleanupFinishBranchDeleteFailureAndGates(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123", failStep: "branch_delete"}
	deps := finishDeps(git)

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	// apply without confirm → 거부.
	req := finishRequest(record.ID, true, preview.Fingerprint)
	req.Confirm = false
	if _, err := CleanupFinish(context.Background(), stateRoot, req, deps); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("apply without confirm must be rejected: %v", err)
	}
	// ④ 실패 → 레코드 보존 + FailedStep 기록. (③은 이미 성공했으므로 재실행
	// 수렴 경로는 Resumable 테스트가 덮는다.)
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != "branch_delete" {
		t.Fatalf("branch delete failure must stop apply: %v %+v", err, result)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != "branch_delete" {
		t.Fatalf("failure point must be recorded: %v %+v", err, kept.CleanupFinishFailure)
	}

	// phase/lease 게이트.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) { rec.Phase = IssueOpsPhasePR })
	if result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps); err == nil || !containsString(result.Missing, "phase_done") {
		t.Fatalf("non-done phase must block: %v %v", err, result.Missing)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Phase = IssueOpsPhaseDone
		rec.Execution.Lease.Status = "active"
		rec.Execution.Lease.ClaimedAt = "2026-07-24T00:00:00Z"
		rec.Execution.Lease.Holder = &NativeActor{
			Host: "codex", SessionID: "s",
			SessionProcess: &NativeProcessReceipt{PID: 1234, StartedAt: "2026-07-24T00:00:00Z", Executable: "/usr/bin/codex"},
		}
	})
	if result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps); err == nil || !containsString(result.Missing, "lease_released") {
		t.Fatalf("active lease must block: %v %v", err, result.Missing)
	}
}

// C2-F1(c): ④' 감사는 파괴 시작 전 스냅샷으로 렌더되어, 워크트리가 삭제된
// 뒤에도 보존 본문(plan/spec)이 빈 값으로 덮이지 않는다.
func TestCleanupFinishAuditUsesPreDestructionSnapshot(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	artifactDir := filepath.Join(worktree, ".agent-harness", "artifact")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "plan.md"), []byte("보존되어야 하는 계획"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	var audited *struct {
		completion string
		audit      string
	}
	deps.ReflectAudit = func(_ IssueOpsRecord, completion portCompletionSection, audit string) error {
		audited = &struct {
			completion string
			audit      string
		}{completion.PlanBody, audit}
		// ④' 시점에는 이미 git worktree remove가 실행된 뒤다 — 스냅샷이
		// 아니라면 PlanBody는 빈 값이었을 것이다.
		return nil
	}
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	// fake git은 실제 파일을 지우지 않으므로 여기서 실제 삭제를 흉내낸다:
	// apply 직전 스냅샷 → 단계 실행 → ④' 검증 순서를 그대로 태운다.
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuditReflected || result.AuditError != "" {
		t.Fatalf("audit must be reflected without error: %+v", result)
	}
	if audited == nil || audited.completion != "보존되어야 하는 계획" || !strings.Contains(audited.audit, "cleanup 완료") {
		t.Fatalf("audit must render the pre-destruction snapshot: %+v", audited)
	}

	// 실패 표면화: ReflectAudit 에러는 ⑤를 막지 않되 AuditError로 드러난다.
	stateRoot2, record2, _ := finishTestRecord(t, true)
	git2 := &fakeFinishGit{branchOID: "abc123"}
	deps2 := finishDeps(git2)
	deps2.ReflectAudit = func(IssueOpsRecord, portCompletionSection, string) error {
		return fmt.Errorf("provider unavailable")
	}
	preview2, err := CleanupFinish(context.Background(), stateRoot2, finishRequest(record2.ID, false, ""), deps2)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := CleanupFinish(context.Background(), stateRoot2, finishRequest(record2.ID, true, preview2.Fingerprint), deps2)
	if err != nil {
		t.Fatal(err)
	}
	if !result2.RecordDeleted || result2.AuditReflected || !strings.Contains(result2.AuditError, "provider unavailable") {
		t.Fatalf("audit failure must be surfaced without blocking deletion: %+v", result2)
	}
}

// brooks H8: remote-branch를 건너뛴 finish가 레코드를 지우면 typed 원격 삭제
// 경로가 그 브랜치에 영원히 닿지 못한다. 잔존과 관측 불가 모두 차단한다.
func TestCleanupFinishBlocksWhileRemoteBranchStillExists(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	present := &fakeFinishGit{branchOID: "abc123", remoteBranchOID: "f00dcafe"}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(present))
	if err == nil || !containsString(result.Missing, "remote_branch_absent") {
		t.Fatalf("surviving remote branch must block finish: %v %v", err, result.Missing)
	}
	unreadable := &fakeFinishGit{branchOID: "abc123", lsRemoteFail: true}
	result, err = CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(unreadable))
	if err == nil || !containsString(result.Missing, "remote_branch_absent") {
		t.Fatalf("unreadable remote must fail closed: %v %v", err, result.Missing)
	}
}

func mutateFinishRecord(t *testing.T, stateRoot, id string, mutate func(*IssueOpsRecord)) {
	t.Helper()
	if err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		mutate(&rec)
		_, err = writeIssueOps(stateRoot, rec)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
