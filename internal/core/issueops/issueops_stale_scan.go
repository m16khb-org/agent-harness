package issueops

import (
	"strings"
	"time"

	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/issueops/readinesspaths"
	"agent-harness/internal/core/issueops/session"
	"agent-harness/internal/core/issueops/stalescan"
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
	probe := stalescan.Probe{
		WorktreeDirExists:  readinesspaths.WorktreePathValid,
		WorktreeHeadBranch: pathutil.GitBranchFromHead,
		RemoteBranchExists: issueOpsRemoteBranchExists,
		Now:                time.Now,
	}
	for _, record := range active.NonDoneCyclesForRepo(issueOpsActiveStore(), repo) {
		finding, ok := stalescan.Classify(record, probe, req.MaxAge)
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
