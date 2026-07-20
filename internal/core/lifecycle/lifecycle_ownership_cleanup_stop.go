package lifecycle

import (
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
)

// OwnershipCleanupHumanGate returns the one pending ownership-transfer cycle
// that a source-root session must present to a human. It is read-only: Stop
// hooks never call preview, approve, record, Orca, or Git cleanup operations.
func OwnershipCleanupHumanGate(req HookToolUseLifecycleRequest) (string, bool) {
	repo := cleanAbsPath(req.CWD)
	if repo == "" || repo != cleanAbsPath(req.Repo) || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.SessionID) == "" {
		return "", false
	}
	ids, err := issueops.ListIssueOpsIDs(issueops.IssueOpsStateRoot())
	if err != nil {
		return "", false
	}
	for _, id := range ids {
		record, err := issueops.ReadIssueOpsExisting(issueops.IssueOpsStateRoot(), id)
		if err != nil || cleanAbsPath(record.Repo) != repo || record.ExecutionHandoff == nil {
			continue
		}
		h := record.ExecutionHandoff
		if h.ProtocolVersion != handoff.OwnershipTransferProtocolVersion || h.State != handoff.StateCleanupPendingHumanDecision || h.Completion == nil {
			continue
		}
		if h.OwnerSession != nil && nativeSessionMatches(req, h.OwnerSession) {
			continue
		}
		return record.ID, true
	}
	return "", false
}
