package issueops

import (
	"strings"
	"time"
)

// RecordIssueOpsHeartbeat updates the LastHeartbeatAt timestamp for a cycle.
// This is a lightweight operation that signals the cycle is still actively
// being worked on, distinct from state-mutating phase/link/intent changes
// (which touch UpdatedAt). The stale scan uses LastHeartbeatAt (falling back
// to UpdatedAt) as the primary liveness signal.
func RecordIssueOpsHeartbeat(stateRoot, id string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = recordHeartbeatLocked(stateRoot, id)
		return e
	})
	return rec, err
}

func recordHeartbeatLocked(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return record, nil
	}
	record.LastHeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	return touchAndWriteIssueOps(stateRoot, record)
}

// IssueOpsLastActiveAt returns the best available liveness timestamp for a
// cycle: LastHeartbeatAt if set, otherwise UpdatedAt.
func IssueOpsLastActiveAt(record IssueOpsRecord) string {
	if hb := strings.TrimSpace(record.LastHeartbeatAt); hb != "" {
		return hb
	}
	return strings.TrimSpace(record.UpdatedAt)
}
