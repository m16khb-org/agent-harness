package issueops

import (
	"fmt"
	"time"

	"agent-harness/internal/contract/issueops"
)

// IssueOpsPruneResult reports one prune preview or confirmed prune pass over
// done cycles.
type IssueOpsPruneResult struct {
	OK        bool     `json:"ok"`
	StateRoot string   `json:"state_root"`
	MaxAge    string   `json:"max_age"`
	Cutoff    string   `json:"cutoff"`
	DryRun    bool     `json:"dry_run"`
	Pruned    []string `json:"pruned"`
	Kept      []string `json:"kept"`
}

// PruneIssueOps deletes done cycles whose lease is released (or absent) and
// whose last update is older than maxAge. Every mutating lifecycle hook scans
// all stored cycles, so unpruned done receipts grow that scan without bound;
// pruning is the structural cap. Anything not provably finished — active or
// non-done phases, unreleased leases, unparseable timestamps — is kept.
// Without confirm the selection is only previewed.
func PruneIssueOps(stateRoot string, maxAge time.Duration, confirm bool) (IssueOpsPruneResult, error) {
	result := IssueOpsPruneResult{
		StateRoot: stateRoot,
		MaxAge:    maxAge.String(),
		DryRun:    !confirm,
		Pruned:    []string{},
		Kept:      []string{},
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		record, err := readIssueOpsUnchecked(stateRoot, id)
		if err != nil {
			return result, err
		}
		if !issueOpsRecordPrunable(record, cutoff) {
			result.Kept = append(result.Kept, id)
			continue
		}
		if confirm {
			if err := deleteIssueOps(stateRoot, id); err != nil {
				return result, err
			}
		}
		result.Pruned = append(result.Pruned, id)
	}
	result.OK = true
	return result, nil
}

func issueOpsRecordPrunable(record issueops.IssueOpsRecord, cutoff time.Time) bool {
	if record.Phase != IssueOpsPhaseDone {
		return false
	}
	if record.Execution != nil && record.Execution.Lease.Status != issueops.LeaseStatusReleased {
		return false
	}
	// 보존 불변식(설계 v5 WS3): 머지 증적(RemoteArtifact)이 있는 레코드는
	// reflect-completion 전까지 나이와 무관하게 보존된다. 이 부류의 가시화는
	// `issueops list`(WS6)가 cleanup 후보로 집계해 담당한다 — 강제 prune
	// 탈출구는 의도적으로 두지 않는다(C2-F6, 방치 사이클은 reflect가 정답).
	// completion 섹션 반영(write-after-verify 캐시)이 확인되기 전에는 prune으로
	// 삭제하지 않는다 — "레코드 삭제 전 보존 완료"를 전 경로에서 보장한다.
	if record.RemoteArtifact != nil {
		if record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt == "" {
			return false
		}
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	if err != nil || updatedAt.IsZero() {
		return false
	}
	return updatedAt.Before(cutoff)
}
