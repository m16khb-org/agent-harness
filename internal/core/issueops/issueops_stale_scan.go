package issueops

import (
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
	Repo   string
	MaxAge time.Duration
	Apply  bool
}

type IssueOpsStaleScanResult struct {
	OK       bool                `json:"ok"`
	Repo     string              `json:"repo"`
	Applied  bool                `json:"applied"`
	Findings []stalescan.Finding `json:"findings"`
	Released []string            `json:"released,omitempty"`
	Errors   []string            `json:"errors,omitempty"`
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
			reason := "stale-cleanup: " + strings.Join(finding.Reasons, ",")
			if _, err := ForceReleaseIssueOps(IssueOpsStateRoot(), finding.ID, reason); err != nil {
				result.Errors = append(result.Errors, finding.ID+": "+err.Error())
				continue
			}
			result.Released = append(result.Released, finding.ID)
		}
	}
	return result
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
