package issueops

import (
	"agent-harness/internal/core/preflight"
	"strings"
)

func issueOpsRemoteArtifactMissing(record IssueOpsRecord) []string {
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
	if len(cleanIssueOpsRemoteValues(record.RemoteArtifact.Labels)) == 0 {
		missing = append(missing, "remote_artifact_labels")
	}
	if len(cleanIssueOpsRemoteValues(record.RemoteArtifact.Assignees)) == 0 {
		missing = append(missing, "remote_artifact_assignees")
	}
	return uniqSorted(missing)
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req IssueOpsCleanupStatusRequest) (IssueOpsCleanupStatus, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsCleanupStatus{OK: false, ID: id}, err
	}
	return IssueOpsCleanupStatusForRecord(record, req), nil
}

func IssueOpsCleanupStatusForRecord(record IssueOpsRecord, req IssueOpsCleanupStatusRequest) IssueOpsCleanupStatus {
	status := IssueOpsCleanupStatus{
		OK:           true,
		ID:           record.ID,
		Merged:       req.Merged,
		WorktreePath: strings.TrimSpace(record.WorktreePath),
		Branch:       strings.TrimSpace(record.Branch),
	}
	if record.RemoteArtifact != nil {
		status.RemoteArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePR) {
		status.Missing = append(status.Missing, "pr_phase")
	}
	status.Missing = append(status.Missing, issueOpsRemoteArtifactMissing(record)...)
	if !req.Merged {
		status.Missing = append(status.Missing, "remote_artifact_merged")
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		status.Missing = append(status.Missing, "worktree_path")
		return finishIssueOpsCleanupStatus(status)
	}
	if !issueOpsWorktreePathValid(worktree) {
		status.Missing = append(status.Missing, "worktree_exists")
		return finishIssueOpsCleanupStatus(status)
	}
	if code, out, stderr := preflight.GitCmd(worktree, "status", "--porcelain=v1"); code != 0 {
		status.Missing = append(status.Missing, "worktree_git_status")
		if strings.TrimSpace(stderr) != "" {
			status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
		}
	} else if strings.TrimSpace(out) != "" {
		status.Missing = append(status.Missing, "worktree_dirty")
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
			status.Missing = append(status.Missing, "remote_branch_present")
		}
	}
	return finishIssueOpsCleanupStatus(status)
}

func finishIssueOpsCleanupStatus(status IssueOpsCleanupStatus) IssueOpsCleanupStatus {
	status.Missing = uniqSorted(status.Missing)
	status.Warnings = uniqSorted(status.Warnings)
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

func firstIssueOpsGitRemote(worktree string) string {
	for _, remote := range strings.Fields(preflight.GitOut(worktree, "remote")) {
		remote = strings.TrimSpace(remote)
		if remote != "" {
			return remote
		}
	}
	return ""
}
