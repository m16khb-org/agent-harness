package issueops

import (
	"time"

	"agent-harness/internal/adapter/issueops/pathutil"
	"agent-harness/internal/contract/issueops"
)

// IssueOpsListEntry는 한 사이클의 조망 행이다. 메인(planner) 세션이 여러
// 사이클을 한 번에 파악하는 read-only 표면으로, lease를 잡거나 repair를
// 수행하지 않는다(설계 v5 WS6).
type IssueOpsListEntry struct {
	ID                    string                 `json:"id"`
	Repo                  string                 `json:"repo"`
	Branch                string                 `json:"branch,omitempty"`
	Phase                 issueops.IssueOpsPhase `json:"phase"`
	Mode                  string                 `json:"mode,omitempty"`
	LeaseStatus           string                 `json:"lease_status,omitempty"`
	HolderHost            string                 `json:"holder_host,omitempty"`
	HolderSession         string                 `json:"holder_session,omitempty"`
	OwnerModel            string                 `json:"owner_model,omitempty"`
	WorkspaceRoot         string                 `json:"workspace_root,omitempty"`
	RemoteArtifactURL     string                 `json:"remote_artifact_url,omitempty"`
	UpdatedAt             string                 `json:"updated_at,omitempty"`
	Claimable             bool                   `json:"claimable,omitempty"`
	CleanupCandidate      bool                   `json:"cleanup_candidate,omitempty"`
	CompletionUnreflected bool                   `json:"completion_unreflected,omitempty"`
	Invalid               bool                   `json:"invalid,omitempty"`
}

// IssueOpsListResult는 집계와 그 비용을 함께 노출한다 — scanned_records가
// O(N) 전량 읽기 비용을 관측 가능하게 한다(브룩스 2차 F9).
type IssueOpsListResult struct {
	OK             bool                `json:"ok"`
	GeneratedAt    string              `json:"generated_at"`
	ScannedRecords int                 `json:"scanned_records"`
	Entries        []IssueOpsListEntry `json:"entries"`
}

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
