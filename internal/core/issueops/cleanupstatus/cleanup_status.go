package cleanupstatus

import (
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/issueops/stringlist"
	"agent-harness/internal/core/preflight"
	"os"
	"strings"
)

type Store struct {
	Read func(stateRoot, id string) (model.IssueOpsRecord, error)
}

func RemoteArtifactMissing(record model.IssueOpsRecord) []string {
	if record.RemoteArtifact == nil {
		return []string{"remote_artifact"}
	}
	missing := []string{}
	if strings.TrimSpace(record.RemoteArtifact.Provider) == "" {
		missing = append(missing, "remote_artifact_provider")
	}
	if strings.TrimSpace(record.RemoteArtifact.Kind) == "" {
		missing = append(missing, "remote_artifact_kind")
	}
	if strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact_url")
	}
	if len(remote.CleanValues(record.RemoteArtifact.Labels)) == 0 {
		missing = append(missing, "remote_artifact_labels")
	}
	if len(remote.CleanValues(record.RemoteArtifact.Assignees)) == 0 {
		missing = append(missing, "remote_artifact_assignees")
	}
	return stringlist.UniqueSorted(missing)
}

func ByID(store Store, stateRoot, id string, req model.IssueOpsCleanupStatusRequest) (model.IssueOpsCleanupStatus, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return model.IssueOpsCleanupStatus{OK: false, ID: id}, err
	}
	return ForRecord(record, req), nil
}

func ForRecord(record model.IssueOpsRecord, req model.IssueOpsCleanupStatusRequest) model.IssueOpsCleanupStatus {
	status := model.IssueOpsCleanupStatus{
		OK:           true,
		ID:           record.ID,
		Merged:       req.Merged,
		WorktreePath: strings.TrimSpace(record.WorktreePath),
		Branch:       strings.TrimSpace(record.Branch),
	}
	if record.RemoteArtifact != nil {
		status.RemoteArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	if model.IssueOpsPhaseRank(record.Phase) < model.IssueOpsPhaseRank(model.IssueOpsPhasePR) {
		status.Missing = append(status.Missing, "pr_phase")
	}
	status.Missing = append(status.Missing, RemoteArtifactMissing(record)...)
	if !req.Merged {
		status.Missing = append(status.Missing, "remote_artifact_merged")
	}
	if hasUnverifiedChildClose(record) {
		status.Missing = append(status.Missing, "child_tasks_closed")
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		status.Missing = append(status.Missing, "worktree_path")
		return finishIssueOpsCleanupStatus(status)
	}
	if !worktreePathValid(worktree) {
		status.Missing = append(status.Missing, "worktree_exists")
		return finishIssueOpsCleanupStatus(status)
	}
	if code, out, stderr := preflight.GitCmd(worktree, "status", "--porcelain=v1"); code != 0 {
		status.Missing = append(status.Missing, "worktree_git_status")
		if strings.TrimSpace(stderr) != "" {
			status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
		}
	} else if strings.TrimSpace(out) != "" {
		// worktree_clean: `missing`은 충족되지 않은 요구의 목록이므로 상태 차단도
		// 요구형으로 적는다. `worktree_dirty`처럼 차단 사실을 적으면 "dirty라는
		// 요구가 미충족"으로 읽힌다(#185). cleanup finish와 switch-mode 게이트가
		// 같은 극성을 쓴다.
		status.Missing = append(status.Missing, "worktree_clean")
	}
	actualBranch := strings.TrimSpace(preflight.GitOut(worktree, "branch", "--show-current"))
	if actualBranch == "" {
		status.Missing = append(status.Missing, "branch")
	} else if strings.TrimSpace(record.Branch) != "" && actualBranch != strings.TrimSpace(record.Branch) {
		status.Missing = append(status.Missing, "branch_match")
	}
	remote := firstIssueOpsGitRemote(worktree)
	if remote == "" {
		status.Missing = append(status.Missing, "remote_branch_check_unavailable")
	} else if actualBranch != "" {
		if code, out, stderr := preflight.GitCmd(worktree, "ls-remote", "--heads", remote, actualBranch); code != 0 {
			status.Missing = append(status.Missing, "remote_branch_check_failed")
			if strings.TrimSpace(stderr) != "" {
				status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
			}
		} else if strings.TrimSpace(out) != "" {
			// remote_branch_absent: cleanup finish가 같은 상태에 쓰는 슬러그다.
			// 여기서 `remote_branch_present`를 쓰면 두 명령이 같은 상태를 반대로
			// 읽히게 보고하고, 운영자는 브랜치가 이미 없다고 판단해
			// `cleanup remote-branch`를 건너뛴다(#185, #181 정리에서 실측).
			//
			// execution sync-base의 동명 슬러그는 브랜치가 **있어야** 한다는 진짜
			// 요구이므로 별개다.
			status.Missing = append(status.Missing, "remote_branch_absent")
		}
	}
	return finishIssueOpsCleanupStatus(status)
}

func hasUnverifiedChildClose(record model.IssueOpsRecord) bool {
	for _, link := range record.IssueLinks {
		if link.Type != "child" {
			continue
		}
		if strings.TrimSpace(link.CloseVerifiedAt) == "" {
			return true
		}
	}
	return false
}

func finishIssueOpsCleanupStatus(status model.IssueOpsCleanupStatus) model.IssueOpsCleanupStatus {
	status.Missing = stringlist.UniqueSorted(status.Missing)
	status.Warnings = stringlist.UniqueSorted(status.Warnings)
	status.Ready = len(status.Missing) == 0
	if status.Ready {
		status.Choices = []string{
			"1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 확인합니다.",
			"3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	} else {
		status.Choices = []string{
			"1. 차단 해소: missing 항목의 merge/worktree/remote branch 증거를 먼저 보강합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 다시 확인합니다.",
			"3. 확장 점검: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	}
	return status
}

func worktreePathValid(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func firstIssueOpsGitRemote(worktree string) string {
	for _, remote := range strings.Fields(preflight.GitOut(worktree, "remote")) {
		remote = strings.TrimSpace(remote)
		if remote != "" {
			return remote
		}
	}
	return ""
}
