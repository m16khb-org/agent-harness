package issueops

import (
	"time"

	"agent-harness/internal/adapter/issueops/pathutil"
	"agent-harness/internal/contract/issueops"
)

// ListIssueOpsCycles는 상태 저장소의 사이클을 집계한다. repo가 비어 있지
// 않으면 그 저장소의 사이클만 남긴다. 훅과 같은 unchecked 읽기를 쓰되
// 손상 레코드는 숨기지 않고 invalid로 표시한다.
func ListIssueOpsCycles(stateRoot, repo string) (IssueOpsListResult, error) {
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		return IssueOpsListResult{OK: false}, err
	}
	repo = pathutil.CleanAbsPath(repo)
	result := IssueOpsListResult{
		OK:          true,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Entries:     []IssueOpsListEntry{},
	}
	for _, id := range ids {
		record, err := readIssueOpsUnchecked(stateRoot, id)
		if err != nil {
			continue
		}
		result.ScannedRecords++
		if repo != "" && pathutil.CleanAbsPath(record.Repo) != repo {
			continue
		}
		entry := IssueOpsListEntry{
			ID: record.ID, Repo: record.Repo, Branch: record.Branch, Phase: record.Phase,
			UpdatedAt: record.UpdatedAt, Invalid: record.Invalid,
		}
		if record.RemoteArtifact != nil {
			entry.RemoteArtifactURL = record.RemoteArtifact.URL
			// prune 보존 불변식으로 나이와 무관하게 잔존하는 부류를 가시화한다
			// — reflect-completion이 다음 행동이다(C2-F6 위임).
			entry.CompletionUnreflected = record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt == ""
		}
		if record.Execution != nil {
			entry.Mode = string(record.Execution.Mode)
			entry.LeaseStatus = string(record.Execution.Lease.Status)
			entry.WorkspaceRoot = record.Execution.Workspace.Root
			if record.Execution.Lease.Holder != nil {
				entry.HolderHost = record.Execution.Lease.Holder.Host
				entry.HolderSession = record.Execution.Lease.Holder.SessionID
			}
			if record.Execution.Orca != nil {
				entry.OwnerModel = record.Execution.Orca.OwnerModel
			}
			entry.Claimable = record.Execution.Lease.Status == issueops.LeaseStatusClaimable
		}
		// 완전 정리(cleanup finish)는 레코드를 삭제하므로, 잔존하는 done
		// 레코드는 전부 정리 후보다.
		entry.CleanupCandidate = record.Phase == issueops.IssueOpsPhaseDone
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}
