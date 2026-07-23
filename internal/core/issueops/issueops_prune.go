package issueops

import (
	"fmt"
	"time"

	"agent-harness/internal/core/issueops/model"
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

func issueOpsRecordPrunable(record IssueOpsRecord, cutoff time.Time) bool {
	if record.Phase != IssueOpsPhaseDone {
		return false
	}
	if record.Execution != nil && record.Execution.Lease.Status != model.LeaseStatusReleased {
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	if err != nil || updatedAt.IsZero() {
		return false
	}
	return updatedAt.Before(cutoff)
}
