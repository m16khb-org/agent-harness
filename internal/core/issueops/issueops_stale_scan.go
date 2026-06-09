package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/issueops/readinesspaths"
	"agent-harness/internal/core/issueops/stalescan"
	"agent-harness/internal/core/preflight"
)

type IssueOpsStaleScanRequest struct {
	Repo         string
	MaxAge       time.Duration
	Apply        bool
	PruneDoneAge time.Duration // if > 0 and Apply is set, prune done cycles older than this
}

type IssueOpsStaleScanResult struct {
	OK         bool                `json:"ok"`
	Repo       string              `json:"repo"`
	Applied    bool                `json:"applied"`
	Findings   []stalescan.Finding `json:"findings"`
	Released   []string            `json:"released,omitempty"`
	Errors     []string            `json:"errors,omitempty"`
	PrunedDone int                 `json:"pruned_done,omitempty"`
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
			err := withIssueOpsLock(IssueOpsStateRoot(), finding.ID, func() error {
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
	// After releasing stale cycles, run git worktree prune on the repo to clean
	// up stale .git/worktrees/<name> registrations left behind by deleted/reset
	// worktrees. Also remove any still-present orphan worktree directories that
	// were stamped by a previous force-release or stale-reset (these are
	// off-hot-path git calls, per CAUTIONS 21).
	if req.Apply && len(result.Released) > 0 {
		issueOpsGitWorktreeCleanup(repo, &result)
	}
	// Prune done cycles older than PruneDoneAge when --apply is set. This
	// removes old JSON + lock files for cycles that have already reached the
	// done phase and are past the retention threshold.
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
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		result.Errors = append(result.Errors, "read state dir: "+err.Error())
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
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
	// Clean orphaned .lock files that have no corresponding .json cycle file.
	// These accumulate when a lock file was created (O_CREATE) but the cycle was
	// deleted or never written, leaving a stale lock file with no guarding cycle.
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".lock")
		jsonPath := filepath.Join(stateRoot, id+".json")
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			os.Remove(filepath.Join(stateRoot, entry.Name()))
		}
	}
}

// pruneDoneCycles removes done-cycle JSON and lock files older than maxAge for
// the given repo. This is only called from the off-hot-path stale scan with
// --apply and --prune-done set.
func pruneDoneCycles(repo string, maxAge time.Duration, result *IssueOpsStaleScanResult) {
	stateRoot := IssueOpsStateRoot()
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		result.Errors = append(result.Errors, "prune-done read state dir: "+err.Error())
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil || record.Repo != repo || record.Phase != IssueOpsPhaseDone {
			continue
		}
		ts := parseIssueOpsTime(record.UpdatedAt)
		if ts.IsZero() || now.Sub(ts) < maxAge {
			continue
		}
		jsonPath := filepath.Join(stateRoot, entry.Name())
		lockPath := filepath.Join(stateRoot, id+".lock")
		os.Remove(jsonPath)
		os.Remove(lockPath)
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
