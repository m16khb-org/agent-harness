package issueops

import (
	"strings"
	"time"

	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/issueops/readinesspaths"
	"agent-harness/internal/core/issueops/session"
	"agent-harness/internal/core/issueops/stalescan"
	"agent-harness/internal/core/operationalhealth"
	"agent-harness/internal/core/preflight"
	"context"
)

type IssueOpsStaleScanRequest struct {
	Repo         string
	MaxAge       time.Duration
	Apply        bool
	PruneDoneAge time.Duration // if > 0 and Apply is set, prune done cycles older than this
}

type IssueOpsStaleScanResult struct {
	OK             bool                `json:"ok"`
	Repo           string              `json:"repo"`
	Applied        bool                `json:"applied"`
	Findings       []stalescan.Finding `json:"findings"`
	Released       []string            `json:"released,omitempty"`
	Errors         []string            `json:"errors,omitempty"`
	PrunedDone     int                 `json:"pruned_done,omitempty"`
	StaleBindings  []string            `json:"stale_bindings,omitempty"`
	PrunedBindings int                 `json:"pruned_bindings,omitempty"`
}

// ScanStaleIssueOpsCycles classifies every non-done cycle for the repo with
// multi-signal liveness probes. With Apply set, it force-releases findings that
// are Releasable (confirmed-stale and likely-done); needs-review findings are
// always reported only. This is a maintenance operation meant to run off the
// tool-call hot path, so it is allowed to consult git/remote state.
func ScanStaleIssueOpsCycles(req IssueOpsStaleScanRequest) IssueOpsStaleScanResult {
	repo := pathutil.CleanAbsPath(req.Repo)
	result := IssueOpsStaleScanResult{OK: true, Repo: repo, Applied: req.Apply, Findings: []stalescan.Finding{}}
	if repo == "" {
		result.OK = false
		result.Errors = append(result.Errors, "repo is required")
		return result
	}
	now := time.Now()
	probe := stalescan.Probe{
		WorktreeDirExists:  readinesspaths.WorktreePathValid,
		WorktreeHeadBranch: pathutil.GitBranchFromHead,
		RemoteBranchExists: issueOpsRemoteBranchExists,
		Now:                func() time.Time { return now },
	}
	for _, record := range active.NonDoneCyclesForRepo(issueOpsActiveStore(), repo) {
		finding, ok := stalescan.Classify(record, probe, req.MaxAge)
		if operationalhealth.EvaluateCycleAuthority(issueOpsOperationalCycle(record), operationalhealth.Options{Now: now}) == operationalhealth.AuthorityDead {
			if ok {
				finding.Reasons = appendUniqueStaleReason(finding.Reasons, operationalhealth.FindingDeadOwner)
			} else {
				finding = stalescan.Finding{
					ID:           record.ID,
					Branch:       strings.TrimSpace(record.Branch),
					Phase:        string(record.Phase),
					Category:     stalescan.CategoryNeedsReview,
					Reasons:      []string{operationalhealth.FindingDeadOwner},
					WorktreePath: strings.TrimSpace(record.WorktreePath),
					Releasable:   false,
				}
				ok = true
			}
		}
		if !ok {
			continue
		}
		result.Findings = append(result.Findings, finding)
		if req.Apply && finding.Releasable {
			// Hold the per-id lock across re-read + re-classify + force-release to
			// fully close the TOCTOU window identified in CAUTIONS 21. A parallel
			// session cannot advance or mutate the cycle while we re-probe and
			// release it.
			err := withIssueOpsLock(context.Background(), IssueOpsStateRoot(), finding.ID, func(context.Context) error {
				fresh, err := ReadIssueOps(IssueOpsStateRoot(), finding.ID)
				if err != nil {
					return err
				}
				confirm, stillStale := stalescan.Classify(fresh, probe, req.MaxAge)
				if !stillStale || !confirm.Releasable {
					return nil
				}
				reason := "stale-cleanup: " + strings.Join(confirm.Reasons, ",")
				_, err = forceReleaseLocked(IssueOpsStateRoot(), finding.ID, reason)
				return err
			})
			if err != nil {
				result.Errors = append(result.Errors, finding.ID+": "+err.Error())
				continue
			}
			result.Released = append(result.Released, finding.ID)
		}
	}
	// #2581 (Task F3): NonDoneCyclesForRepo excludes done cycles, so a done-phase
	// record whose supervised handoff is still non-terminal (recovery_required,
	// dispatched, etc.) is invisible to the loop above yet keeps fencing the
	// source checkout. SupervisedHandoffCyclesForRepo retains such records while
	// their handoff is non-closed; report the done ones as the report-only
	// handoff_nonterminal_on_terminal_phase signal. Never Releasable — --apply
	// must not release or prune it (pruneDoneCycles also skips it).
	for _, record := range active.SupervisedHandoffCyclesForRepo(issueOpsActiveStore(), repo) {
		if record.Phase != IssueOpsPhaseDone {
			continue
		}
		if finding, ok := stalescan.Classify(record, probe, req.MaxAge); ok {
			result.Findings = append(result.Findings, finding)
		}
	}
	// Whenever --apply is set, run git worktree prune on the repo to clean up
	// stale .git/worktrees/<name> registrations left behind by deleted/reset
	// worktrees, and remove any still-present orphan worktree directories that
	// were stamped by a previous force-release or stale-reset (these are
	// off-hot-path git calls, per CAUTIONS 21).
	if req.Apply {
		issueOpsGitWorktreeCleanup(repo, &result)
	}
	// Scan session bindings for stale entries: a binding whose cycle record is
	// absent or done is stale and should be reported (and, with --apply, pruned).
	// Session bindings are never scanned by StatePrune or the cycle classifier,
	// so without this pass orphan bindings accumulate indefinitely.
	stateRoot := IssueOpsStateRoot()
	isCycleLive := func(cycleID string) bool {
		rec, err := ReadIssueOps(stateRoot, cycleID)
		if err != nil || !rec.OK || rec.Phase == IssueOpsPhaseDone {
			return false
		}
		return true
	}
	store := issueOpsSessionStore()
	staleEntries, scanErr := session.FindStaleBindings(store, repo, isCycleLive)
	if scanErr != nil {
		result.Errors = append(result.Errors, "scan session bindings: "+scanErr.Error())
	} else {
		for _, e := range staleEntries {
			result.StaleBindings = append(result.StaleBindings, e.CycleID)
		}
		if req.Apply && len(staleEntries) > 0 {
			pruned, pruneErr := session.PruneStaleBindings(store, repo, staleEntries, isCycleLive)
			if pruneErr != nil {
				result.Errors = append(result.Errors, "prune session bindings: "+pruneErr.Error())
			} else {
				result.PrunedBindings = pruned
			}
		}
	}
	// Prune done cycles older than PruneDoneAge when --apply is set. This
	// deletes the record for cycles that have already reached the done phase
	// and are past the retention threshold.
	if req.Apply && req.PruneDoneAge > 0 {
		pruneDoneCycles(repo, req.PruneDoneAge, &result)
	}
	return result
}

func issueOpsOperationalCycle(record model.IssueOpsRecord) operationalhealth.Cycle {
	cycle := operationalhealth.Cycle{
		ID:           strings.TrimSpace(record.ID),
		Repo:         pathutil.CleanAbsPath(record.Repo),
		Branch:       strings.TrimSpace(record.Branch),
		Phase:        string(record.Phase),
		WorktreePath: pathutil.CleanAbsPath(record.WorktreePath),
	}
	if record.ExecutionHandoff == nil {
		cycle.LastHeartbeatAt = parseIssueOpsTime(record.LastHeartbeatAt)
		return cycle
	}

	handoffRecord := record.ExecutionHandoff
	cycle.HandoffState = strings.TrimSpace(handoffRecord.State)
	cycle.Attempt = handoffRecord.Attempt
	cycle.OwnershipEpoch = strings.TrimSpace(handoffRecord.OwnershipEpoch)
	cycle.ContextSHA256 = strings.TrimSpace(handoffRecord.ContextSHA256)
	if handoffRecord.WorkerSession != nil {
		cycle.WorkerSessionID = strings.TrimSpace(handoffRecord.WorkerSession.SessionID)
		cycle.WorkerAgentID = strings.TrimSpace(handoffRecord.WorkerSession.AgentID)
	}
	if handoffRecord.Orca != nil {
		cycle.OrcaRuntimeID = strings.TrimSpace(handoffRecord.Orca.RuntimeID)
		cycle.OrcaRepoID = strings.TrimSpace(handoffRecord.Orca.RepoID)
		cycle.OrcaWorktreeID = strings.TrimSpace(handoffRecord.Orca.WorktreeID)
		cycle.OrcaWorktreeInstanceID = strings.TrimSpace(handoffRecord.Orca.WorktreeInstanceID)
		cycle.TerminalHandle = strings.TrimSpace(handoffRecord.Orca.WorkerTerminalHandle)
		cycle.PTYID = strings.TrimSpace(handoffRecord.Orca.WorkerPTYID)
		cycle.TerminalTabID = strings.TrimSpace(handoffRecord.Orca.WorkerTabID)
		cycle.TerminalLeafID = strings.TrimSpace(handoffRecord.Orca.WorkerLeafID)
		cycle.TaskID = strings.TrimSpace(handoffRecord.Orca.TaskID)
		cycle.DispatchID = strings.TrimSpace(handoffRecord.Orca.DispatchID)
		if cycle.WorktreePath == "" {
			cycle.WorktreePath = pathutil.CleanAbsPath(handoffRecord.Orca.WorktreePath)
		}
	}
	if cycle.WorktreePath == "" {
		cycle.WorktreePath = pathutil.CleanAbsPath(handoffRecord.WorkerRoot)
	}
	heartbeat := strings.TrimSpace(handoffRecord.LastHeartbeatAt)
	if heartbeat == "" {
		heartbeat = strings.TrimSpace(record.LastHeartbeatAt)
	}
	cycle.LastHeartbeatAt = parseIssueOpsTime(heartbeat)
	return cycle
}

func appendUniqueStaleReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

// issueOpsGitWorktreeCleanup runs git worktree prune on the repo and, for
// done/force-released cycles whose orphan worktree directory still exists on
// disk, removes the git worktree registration and the directory. This is only
// called from the off-hot-path stale scan with --apply.
func issueOpsGitWorktreeCleanup(repo string, result *IssueOpsStaleScanResult) {
	code, _, stderr := preflight.GitCmd(repo, "worktree", "prune")
	if code != 0 {
		result.Errors = append(result.Errors, "git worktree prune: "+stderr)
	}
	// Find done/force-released cycles with orphan worktree paths to clean.
	stateRoot := IssueOpsStateRoot()
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		result.Errors = append(result.Errors, "list state records: "+err.Error())
		return
	}
	for _, id := range ids {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil || record.Repo != repo || record.Phase != IssueOpsPhaseDone {
			continue
		}
		orphan := strings.TrimSpace(record.OrphanWorktreePath)
		if orphan == "" || !readinesspaths.WorktreePathValid(orphan) {
			continue
		}
		code, _, stderr = preflight.GitCmd(repo, "worktree", "remove", "--force", orphan)
		if code != 0 {
			result.Errors = append(result.Errors, "git worktree remove "+orphan+": "+stderr)
		}
	}
}

// pruneDoneCycles deletes done-cycle records older than maxAge for the given
// repo. This is only called from the off-hot-path stale scan with --apply and
// --prune-done set.
func pruneDoneCycles(repo string, maxAge time.Duration, result *IssueOpsStaleScanResult) {
	stateRoot := IssueOpsStateRoot()
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		result.Errors = append(result.Errors, "prune-done list state records: "+err.Error())
		return
	}
	now := time.Now()
	for _, id := range ids {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil || record.Repo != repo || record.Phase != IssueOpsPhaseDone {
			continue
		}
		if record.ExecutionHandoff != nil && record.ExecutionHandoff.State != handoff.StateClosed {
			// #2581 (Task F3): a done cycle whose supervised handoff is
			// non-terminal may still own un-reconciled Orca artifacts (a
			// cleanup_only worktree/task). Age-based prune here would be a TTL
			// auto-release that abandons them ("timeout != absence"). Skip it;
			// the stalescan classifier reports it and the operator recovers it.
			continue
		}
		ts := parseIssueOpsTime(record.UpdatedAt)
		if ts.IsZero() || now.Sub(ts) < maxAge {
			continue
		}
		if err := deleteIssueOps(stateRoot, id); err != nil {
			result.Errors = append(result.Errors, "prune-done delete "+id+": "+err.Error())
			continue
		}
		result.PrunedDone++
	}
}

// parseIssueOpsTime parses an RFC 3339 timestamp string, trying nano then
// second precision. Returns the zero time on failure.
func parseIssueOpsTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// issueOpsRemoteBranchExists reports whether the cycle's remote branch still
// exists. It is best-effort: when the remote/branch cannot be determined it
// returns determined=false so the classifier does not infer a merge.
func issueOpsRemoteBranchExists(record model.IssueOpsRecord) (bool, bool) {
	dir := strings.TrimSpace(record.WorktreePath)
	if dir == "" || !readinesspaths.WorktreePathValid(dir) {
		dir = strings.TrimSpace(record.Repo)
	}
	if dir == "" {
		return false, false
	}
	branch := strings.TrimSpace(record.Branch)
	if record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.Branch) != "" {
		branch = strings.TrimSpace(record.BranchPrepare.Branch)
	}
	if branch == "" {
		return false, false
	}
	remote := firstIssueOpsRemote(dir)
	if remote == "" {
		return false, false
	}
	code, out, _ := preflight.GitCmd(dir, "ls-remote", "--heads", remote, branch)
	if code != 0 {
		return false, false
	}
	return strings.TrimSpace(out) != "", true
}

func firstIssueOpsRemote(dir string) string {
	for remote := range strings.FieldsSeq(preflight.GitOut(dir, "remote")) {
		if remote = strings.TrimSpace(remote); remote != "" {
			return remote
		}
	}
	return ""
}
