package issueops

import (
	"fmt"
	"strings"
	"time"
)

func ForceReleaseIssueOps(stateRoot, id, reason string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = forceReleaseLocked(stateRoot, id, reason)
		return e
	})
	if err == nil {
		unbindIssueOpsSessionForCycle(rec.Repo, id)
	}
	return rec, err
}

// forceReleaseLocked performs the force-release mutation without acquiring the
// per-id lock. Callers must hold withIssueOpsLock for stateRoot+id before
// calling this function (e.g. ScanStaleIssueOpsCycles which holds the lock
// across re-read+classify+release to close the TOCTOU window).
func forceReleaseLocked(stateRoot, id, reason string) (IssueOpsRecord, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("force-release reason must be at least 10 characters")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return record, nil
	}
	record.Phase = IssueOpsPhaseDone
	record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.ForceReleaseReason = reason
	// Preserve the existing worktree path as an orphan audit marker so the
	// off-hot-path stale-scan reaper can later run git worktree prune/remove.
	// Do NOT synchronously delete the directory — it may contain uncommitted work.
	if strings.TrimSpace(record.WorktreePath) != "" && strings.TrimSpace(record.OrphanWorktreePath) == "" {
		record.OrphanWorktreePath = record.WorktreePath
	}
	return touchAndWriteIssueOps(stateRoot, record)
}
