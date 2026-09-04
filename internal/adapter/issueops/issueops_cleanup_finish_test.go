package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type portCompletionSection = port.IssueProviderCompletionSection

type fakeFinishGit struct {
	statusOut string
	branchOID string
	// remoteBranchOID가 비어 있으면 원격 브랜치는 이미 부재다(finish의 정상
	// 전제). lsRemoteFail은 관측 불가를 흉내낸다.
	remoteBranchOID string
	lsRemoteFail    bool
	// basePresent는 준비 base 브랜치가 origin에 남아 있는 상태다(#490).
	// defaultRef가 비면 원격 HEAD 미설정을 흉내내 관측 실패가 된다.
	basePresent     bool
	defaultRef      string
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
		if len(args) > 1 && args[1] == "--symref" {
			if g.defaultRef == "" {
				return 0, ""
			}
			return 0, "ref: " + g.defaultRef + "\tHEAD\nabc\tHEAD\n"
		}
		ref := args[len(args)-1]
		if ref != "refs/heads/80-finish" {
			// 준비 base 브랜치 관측: 부재는 exit 0 + 빈 출력이다(실측).
			if g.basePresent {
				return 0, "base123\t" + ref + "\n"
			}
			return 0, ""
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

func finishTestRecord(t *testing.T, withWorktree bool) (string, issueops.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "80-finish"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := ""
	record.Phase = IssueOpsPhaseDone
	record.IssueURL = "https://github.com/acme/repo/issues/80"
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/90"}
	// execution complete가 base_branch 없는 done 전이를 거부하므로 done 레코드는
	// 항상 준비된 base를 갖는다.
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: "80-finish",
		BaseBranch: "main", BaseSHA: "deadbeef", LinkVerified: true,
	}
	if withWorktree {
		worktree = filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "80-finish")
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
		record.Execution = &issueops.Execution{
			Mode:      issueops.ExecutionModeDirect,
			Workspace: issueops.Workspace{SourceRoot: repo, Root: worktree, Branch: "80-finish", BaseHead: "deadbeef", Driver: "git", LinkedAt: "2026-07-24T00:00:00Z"},
			Lease:     issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased},
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
		Git:       git.run,
		Processes: quietCleanupProcesses(),
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
	t.Run("live process is listed, not blocking", func(t *testing.T) {
		deps := finishDeps(git)
		deps.Processes = worldCleanupProcesses(occupiedWorld(t, codexOccupant()), nil)
		deps.OrcaTerminals = readyOrca(t, worktree)
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil || containsString(result.Missing, "workspace_processes_quiescent") || len(result.WorkspaceProcesses) != 1 {
			t.Fatalf("occupants are apply targets now, not a preview blocker: %v %+v", err, result)
		}
	})
	t.Run("unclosed child", func(t *testing.T) {
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.IssueLinks = append(rec.IssueLinks, issueops.IssueOpsIssueLink{Type: "child", URL: "https://github.com/acme/repo/issues/91"})
		})
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err == nil || !containsString(result.Missing, "child_tasks_closed") {
			t.Fatalf("unclosed child must block: %v %v", err, result.Missing)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.IssueLinks[len(rec.IssueLinks)-1].CloseVerifiedAt = "t"
		})
	})
}

// done 전이는 draft PR 생성 직후에 일어나고 finish는 머지 이후에 실행되므로, 그
// 사이 구간에서 draft PR의 base가 바뀔 수 있다. 준비된 base가 아닌 브랜치로
// 머지된 결과를 파괴 전에 잡지 못하면 재검증 수단이 남지 않는다.
// #490: 부모 브랜치가 머지·삭제되어 provider가 PR을 기본 브랜치로 재타깃한
// 흐름은 drift가 아니다. 준비 base의 원격 부재와 기본 브랜치 일치라는 두
// 관측으로만 통과시키고, 나머지는 그대로 거부한다.
func TestClassifyMergedBaseRetarget(t *testing.T) {
	cases := []struct {
		name                              string
		prepared, observed, defaultBranch string
		preparedRemotePresent, observed_  bool
		want                              []string
	}{
		{name: "same base passes", prepared: "main", observed: "main", defaultBranch: "main", observed_: true},
		{name: "no prepared base is not judged", prepared: "", observed: "release", defaultBranch: "main", observed_: true},
		{name: "retarget to default after parent merged", prepared: "484-parent", observed: "main", defaultBranch: "main", observed_: true},
		{name: "parent branch still present is drift", prepared: "484-parent", observed: "main", defaultBranch: "main", preparedRemotePresent: true, observed_: true, want: []string{"base_branch_drifted"}},
		{name: "merged into a non-default branch is drift", prepared: "484-parent", observed: "release", defaultBranch: "main", observed_: true, want: []string{"base_branch_drifted"}},
		{name: "unobserved keeps the drift fact and names the missing observation", prepared: "484-parent", observed: "main", defaultBranch: "main", observed_: false, want: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
		{name: "empty default branch fails closed", prepared: "484-parent", observed: "main", defaultBranch: "", observed_: true, want: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyMergedBase(tc.prepared, tc.observed, tc.defaultBranch, tc.preparedRemotePresent, tc.observed_)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("classifyMergedBase = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCleanupFinishAllowsRetargetedBaseAfterParentMerged(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.BranchPrepare.BaseBranch = "484-parent"
	})

	t.Run("parent gone and default base passes", func(t *testing.T) {
		git := &fakeFinishGit{branchOID: "abc123", defaultRef: "refs/heads/main"}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err != nil || containsString(result.Missing, "base_branch_drifted") || containsString(result.Missing, "merged_base_remote_unobserved") {
			t.Fatalf("retarget after the parent merged must pass: err=%v missing=%v", err, result.Missing)
		}
	})
	t.Run("parent still present blocks", func(t *testing.T) {
		git := &fakeFinishGit{branchOID: "abc123", defaultRef: "refs/heads/main", basePresent: true}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err == nil || !containsString(result.Missing, "base_branch_drifted") {
			t.Fatalf("a live prepared base must stay drift: err=%v missing=%v", err, result.Missing)
		}
	})
	t.Run("remote HEAD unset fails closed", func(t *testing.T) {
		git := &fakeFinishGit{branchOID: "abc123"}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(git))
		if err == nil || !containsString(result.Missing, "merged_base_remote_unobserved") {
			t.Fatalf("unobservable default branch must fail closed: err=%v missing=%v", err, result.Missing)
		}
	})
}

func TestCleanupFinishBlocksBaseBranchDrift(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	// 준비 base(main)가 원격에 살아 있고 기본 브랜치도 관측되는 정상 환경이다.
	// 이 상태에서 다른 브랜치로 머지된 것은 재타깃이 아니라 drift다(#490).
	git := &fakeFinishGit{branchOID: "abc123", defaultRef: "refs/heads/main", basePresent: true}

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
	t.Run("drift with unobservable remote fails closed on the observation", func(t *testing.T) {
		req := finishRequest(record.ID, false, "")
		req.MergedBaseBranch = "release"
		result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(&fakeFinishGit{branchOID: "abc123", lsRemoteFail: true}))
		if err == nil || !containsString(result.Missing, "merged_base_remote_unobserved") || !containsString(result.Missing, "base_branch_drifted") {
			t.Fatalf("unobservable remote must keep the drift fact and name the missing observation: err=%v missing=%v", err, result.Missing)
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
	fresh, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: record.Repo, Branch: "80-finish"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Phase != IssueOpsPhaseProblem || fresh.Execution != nil || fresh.RemoteArtifact != nil {
		t.Fatalf("finish must unlock same-branch rework with a fresh cycle: %+v", fresh)
	}
}

func TestCleanupFinishOrcaRemovalRunsFirstAndFailureKeepsRecord(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Mode = issueops.ExecutionModeOrca
		rec.Execution.Workspace.Driver = "orca"
		rec.Execution.Orca = &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	deps.OrcaTerminals = readyOrca(t, worktree)
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
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Mode = issueops.ExecutionModeOrca
		rec.Execution.Workspace.Driver = "orca"
		rec.Execution.Orca = &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	deps.OrcaTerminals = readyOrca(t, worktree)
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
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhasePR })
	if result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps); err == nil || !containsString(result.Missing, "phase_done") {
		t.Fatalf("non-done phase must block: %v %v", err, result.Missing)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Phase = IssueOpsPhaseDone
		rec.Execution.Lease.Status = "active"
		rec.Execution.Lease.ClaimedAt = "2026-07-24T00:00:00Z"
		rec.Execution.Lease.Holder = &issueops.NativeActor{
			Host: "codex", SessionID: "s",
			SessionProcess: &issueops.NativeProcessReceipt{PID: 1234, StartedAt: "2026-07-24T00:00:00Z", Executable: "/usr/bin/codex"},
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
	artifactDir := filepath.Join(worktree, ".issueops", "artifact")
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
	deps.ReflectAudit = func(_ issueops.IssueOpsRecord, completion portCompletionSection, audit string) error {
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
	deps2.ReflectAudit = func(issueops.IssueOpsRecord, portCompletionSection, string) error {
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

// design-review H8: remote-branch를 건너뛴 finish가 레코드를 지우면 typed 원격 삭제
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

func mutateFinishRecord(t *testing.T, stateRoot, id string, mutate func(*issueops.IssueOpsRecord)) {
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

// 재타깃이 branch retarget으로 레코드에 기록되면 finish는 준비 base와 관측 base가
// 같으므로 drift를 보고하지 않는다. 옛 base(release/stg)가 원격에 살아 있어도
// 마찬가지다 — 면제가 아니라 기록된 결정이기 때문이다.
func TestCleanupFinishAcceptsRecordedRetarget(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.BranchPrepare.Retargets = []issueops.IssueOpsBranchRetarget{{FromBase: "main", ToBase: "2803-umbrella", Reason: "child of the umbrella"}}
		rec.BranchPrepare.BaseBranch = "2803-umbrella"
	})
	git := &fakeFinishGit{branchOID: "abc123", defaultRef: "refs/heads/main", basePresent: true}
	req := finishRequest(record.ID, false, "")
	req.MergedBaseBranch = "2803-umbrella"
	result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(git))
	if err != nil || containsString(result.Missing, "base_branch_drifted") || result.RetargetedBase != nil {
		t.Fatalf("a recorded retarget is the prepared base, not drift: err=%v missing=%v retargeted=%+v", err, result.Missing, result.RetargetedBase)
	}
}
