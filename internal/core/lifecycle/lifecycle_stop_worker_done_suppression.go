package lifecycle

import (
	"strings"

	"agent-harness/internal/core/issueops/handoff"
)

// SuppressStopNextActionForCompletedWorker reports whether the Stop hook's
// numbered-next-action relay and missing-choice re-entry must be suppressed
// for this exact worker. It resolves only repo-scoped IssueOps session
// bindings for the canonical worker root; it never scans the global record
// set, inspects transcripts, shell commands, CLI output, or assistant prose,
// and never compares ORCA_TERMINAL_HANDLE or mailbox handles.
//
// Suppression requires a durable handoff that is authoritatively complete
// (submitted, or closed+accepted) with a completed result and a persisted
// terminal worker_done_projection state (sent, failed, or intent), plus exact
// canonical worktree, branch, host, native-session, and agent identity. Any
// mismatch or ambiguity fails closed and leaves legacy Stop behavior untouched.
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
	if cwd == "" || cwd != recordWorktree || recordWorktree != workerRoot || !currentWorkerBranchMatches(record) || !nativeSessionMatches(req, h.WorkerSession) {
		return false
	}
	completed := h.State == handoff.StateSubmitted || (h.State == handoff.StateClosed && h.ClosedDisposition == handoff.DispositionAccepted)
	if !completed {
		return false
	}
	if h.Result == nil || h.Result.Outcome != handoff.OutcomeCompleted {
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

	repos := make([]string, 0, 4)
	seenRepos := map[string]bool{}
	for _, candidate := range []string{
		req.SourceCheckout,
		req.Repo,
		sourceCheckoutFromWorktree(req.CWD),
		sourceCheckoutFromWorktree(req.Repo),
	} {
		repo := cleanAbsPath(candidate)
		if repo == "" || seenRepos[repo] {
			continue
		}
		seenRepos[repo] = true
		repos = append(repos, repo)
	}

	records := map[string]IssueOpsRecord{}
	for _, repo := range repos {
		bindings, err := listIssueOpsSessionBindings(repo)
		if err != nil {
			return IssueOpsRecord{}, false
		}
		for _, binding := range bindings {
			if cleanAbsPath(binding.ExpectedWorktree) != cwd {
				continue
			}
			record, active := ActiveIssueOpsCycleForBranch(repo, binding.Branch)
			if !active || record.ID != strings.TrimSpace(binding.CycleID) || cleanAbsPath(record.Repo) != repo {
				return IssueOpsRecord{}, false
			}
			records[record.ID] = record
		}
	}
	if len(records) != 1 {
		return IssueOpsRecord{}, false
	}
	for _, record := range records {
		return record, true
	}
	return IssueOpsRecord{}, false
}
