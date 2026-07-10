package issueops

import (
	"strings"
)

// RecordIssueOpsHeartbeat updates the LastHeartbeatAt timestamp for a cycle.
// This is a lightweight operation that signals the cycle is still actively
// being worked on, distinct from state-mutating phase/link/intent changes
// (which touch UpdatedAt). The stale scan uses LastHeartbeatAt (falling back
// to UpdatedAt) as the primary liveness signal.
func RecordIssueOpsHeartbeat(stateRoot, id string) (IssueOpsRecord, error) {
	return RecordIssueOpsHeartbeatWithRequest(stateRoot, IssueOpsHeartbeatRequest{ID: id})
}

// IssueOpsLastActiveAt returns the best available liveness timestamp for a
// cycle: LastHeartbeatAt if set, otherwise UpdatedAt.
func IssueOpsLastActiveAt(record IssueOpsRecord) string {
	if hb := strings.TrimSpace(record.LastHeartbeatAt); hb != "" {
		return hb
	}
	return strings.TrimSpace(record.UpdatedAt)
}
