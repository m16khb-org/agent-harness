package lifecycle

import (
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
)

// SuppressStopNextActionForCompletedWorker reports whether the Stop hook's
// numbered-next-action relay and missing-choice re-entry must be suppressed
// for this exact worker. It derives one source checkout and branch from the
// native cwd, then reads only their deterministic IssueOps record; it never
// scans session bindings or the global record set, inspects transcripts, shell
// commands, CLI output, or assistant prose, and never compares
// ORCA_TERMINAL_HANDLE or mailbox handles.
//
// Suppression requires a durable handoff that is authoritatively complete
// with completion evidence and a persisted
// terminal worker_done_projection state (sent, failed, or intent), plus exact
// canonical worktree, branch, host, native-session, and agent identity. Any
// mismatch or ambiguity fails closed and leaves normal Stop behavior untouched.
func SuppressStopNextActionForCompletedWorker(req HookToolUseLifecycleRequest) bool {
	record, ok := stopSuppressionRecord(req)
	if !ok {
		return false
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		return false
	}
	h := record.ExecutionHandoff
	if h == nil {
		return false
	}
	cwd := cleanAbsPath(req.CWD)
	recordWorktree := cleanAbsPath(record.WorktreePath)
	workerRoot := cleanAbsPath(h.WorkerRoot)
	if cwd == "" || cwd != recordWorktree || recordWorktree != workerRoot || !nativeSessionMatches(req, h.OwnerSession) {
		return false
	}
	if h.State != handoff.StateCleanupPendingHumanDecision || h.Completion == nil {
		return false
	}
	projection := h.WorkerDoneProjection
	if projection == nil {
		return false
	}
	switch projection.State {
	case "sent", "failed", "intent":
	default:
		return false
	}
	return true
}

func stopSuppressionRecord(req HookToolUseLifecycleRequest) (IssueOpsRecord, bool) {
	cwd := cleanAbsPath(req.CWD)
	if cwd == "" {
		return IssueOpsRecord{}, false
	}
	repo := cleanAbsPath(sourceCheckoutFromWorktree(cwd))
	branch := strings.TrimSpace(gitBranchFromHead(cwd))
	if repo == "" || branch == "" {
		return IssueOpsRecord{}, false
	}
	record, err := issueops.ReadIssueOpsForStopSuppression(issueops.IssueOpsStateRoot(), issueops.NewIssueOpsID(repo, branch))
	if err != nil || !record.OK || cleanAbsPath(record.Repo) != repo || strings.TrimSpace(record.Branch) != branch {
		return IssueOpsRecord{}, false
	}
	return record, true
}
