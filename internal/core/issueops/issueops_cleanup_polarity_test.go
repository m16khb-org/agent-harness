package issueops

import (
	"context"
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

// cleanup 진단의 `missing`은 **충족되지 않은 요구**의 목록이다. 그 안에 상태 차단을
// 그대로 적으면 극성이 뒤집혀 읽힌다(#185).
//
// `#181` 정리에서 실측했다. 원격 브랜치가 **존재하는** 상태에서 두 명령이 이렇게
// 보고했다:
//
//	cleanup status --merged  ->  missing: ["remote_branch_present"]
//	cleanup finish --preview ->  missing: ["remote_branch_absent"]
//
// 둘 다 "원격 브랜치가 있어서 막혔다"를 말하고 있었지만, `missing` 안의
// `remote_branch_present`는 "원격 브랜치 존재라는 요구가 미충족" = 브랜치가 없다로
// 읽힌다. 실제 상태를 `git ls-remote`로 따로 확인해야 했다.
//
// 요구 극성이 다수이고 선례다 — `#167`의 switch-mode 게이트(`worktree_clean`,
// `lease_holds_no_writer`, `orca_branch_name_free`)와 `cleanup finish`
// (`worktree_clean`, `remote_branch_absent`)가 그렇다.
//
// 이 테스트가 고정하는 것은 이름의 의미가 아니라 **두 표면이 같은 물리 상태를 같은
// 슬러그로 부른다**는 것이다. 극성의 의미는 문자열로 판정할 수 없지만 두 표면의
// 불일치는 기계적으로 잡을 수 있다.
func TestRemoteBranchSurvivalBlocksBothCleanupSurfacesWithTheSameSlug(t *testing.T) {
	statusMissing := cleanupStatusMissingWithSurvivingRemoteBranch(t)

	stateRoot, record, _ := finishTestRecord(t, true)
	present := &fakeFinishGit{branchOID: "abc123", remoteBranchOID: "f00dcafe"}
	finish, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(present))
	if err == nil {
		t.Fatal("surviving remote branch must block finish")
	}

	shared := sharedRemoteBranchSlug(statusMissing, finish.Missing)
	if shared == "" {
		t.Fatalf("두 표면이 원격 브랜치 잔존을 다른 슬러그로 부른다:\nstatus=%v\nfinish=%v\n"+
			"missing은 충족되지 않은 요구의 목록이므로 한쪽만 상태-차단 극성을 쓰면 반대로 읽힌다",
			statusMissing, finish.Missing)
	}
	if shared != "remote_branch_absent" {
		t.Fatalf("원격 브랜치 잔존의 요구형 슬러그는 remote_branch_absent다: %q", shared)
	}
}

// sharedRemoteBranchSlug는 두 missing 목록이 공유하는 원격 브랜치 슬러그를 돌려준다.
// 두 표면이 같은 상태에 다른 이름을 쓰면 공유가 없다.
func sharedRemoteBranchSlug(left, right []string) string {
	inRight := map[string]bool{}
	for _, slug := range right {
		inRight[slug] = true
	}
	for _, slug := range left {
		if !inRight[slug] {
			continue
		}
		if slug == "remote_branch_absent" || slug == "remote_branch_present" {
			return slug
		}
	}
	return ""
}

// cleanupStatusMissingWithSurvivingRemoteBranch는 원격 소스 브랜치가 남아 있는
// 머지된 사이클을 실제 Git 저장소로 만들고 그 상태의 missing을 돌려준다.
// `cleanup status`는 주입된 러너가 아니라 실제 저장소를 관측하므로 fake로 대체할 수
// 없다.
func cleanupStatusMissingWithSurvivingRemoteBranch(t *testing.T) []string {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "2-polarity"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	if record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/2"); err != nil {
		t.Fatal(err)
	}
	if record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	if record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md")); err != nil {
		t.Fatal(err)
	}
	record = recordIssueOpsPreparedExecutionForTest(t, stateRoot, record.ID, worktree)
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\n")
	if record, err = AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean), issueOpsActorForTest(worktree)); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "add", "internal/demo.go", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "feat: implement polarity fixture"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	if record, err = AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(IssueOpsPhasePR), issueOpsActorForTest(worktree)); err != nil {
		t.Fatal(err)
	}
	if record, err = VerifyIssueOpsRemoteArtifactWithActor(stateRoot, record.ID, issueops.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/2",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}, issueOpsActorForTest(worktree)); err != nil {
		t.Fatal(err)
	}

	// 원격 브랜치를 지우지 않은 상태다. 그것이 유일한 차단 사유여야 한다.
	status := IssueOpsCleanupStatusForRecord(record, issueops.IssueOpsCleanupStatusRequest{Merged: true})
	if status.Ready {
		t.Fatal("surviving remote branch must block cleanup status")
	}
	return status.Missing
}
