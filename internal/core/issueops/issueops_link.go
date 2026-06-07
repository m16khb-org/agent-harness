package issueops

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/remote"
)

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	u := strings.TrimSpace(issueURL)
	if err := validateIssueURL(u); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.IssueURL = u
	if ready := IssueOpsPlanReadiness(record); ready.Ready && issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePlan) {
		record.Phase = IssueOpsPhasePlan
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	path := strings.TrimSpace(planPath)
	if path == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must not contain path traversal")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := issueOpsBranchEvidenceMissing(record); len(missing) > 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before linked worktree")
	}
	if missing := issueOpsDesignReviewMissing(record); len(missing) > 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before approved design review: missing %s", strings.Join(uniqSorted(missing), ", "))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, path)
	}
	if !issueOpsPlanPathExists(record.Repo, path) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path does not exist: %s", path)
	}
	if !issueOpsPlanPathInsideWorktree(worktree, path) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", worktree)
	}
	record.PlanPath = path
	record.Phase = IssueOpsPhaseImplement
	return touchAndWriteIssueOps(stateRoot, record)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	path := strings.TrimSpace(worktreePath)
	if path == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path must not contain path traversal")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := issueOpsBranchEvidenceMissing(record); len(missing) > 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot link worktree before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	if !issueOpsWorktreePathValid(path) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path does not exist or is not a directory: %s", path)
	}
	if err := validateIssueOpsIsolatedWorktreePath(record, path); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	if err := validateIssueOpsWorktreeBranch(record, path); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	if planPath := strings.TrimSpace(record.PlanPath); planPath != "" && !issueOpsPlanPathInsideWorktree(path, planPath) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", path)
	}
	record.WorktreePath = path
	return touchAndWriteIssueOps(stateRoot, record)
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	u := strings.TrimSpace(childURL)
	if err := validateIssueURL(u); err != nil {
		return IssueOpsRecord{OK: false}, fmt.Errorf("child_url %s", strings.TrimPrefix(err.Error(), "issue_url "))
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot link child before linked parent issue")
	}
	if parentProvider := remote.ProviderFromURL(record.IssueURL); parentProvider != "" && remote.ProviderFromURL(u) != parentProvider {
		return IssueOpsRecord{OK: false}, fmt.Errorf("child issue provider must match linked parent issue provider")
	}
	if err := remote.ValidateChildMatchesParent(record.IssueURL, u); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && link.URL == u {
			return IssueOpsRecord{OK: false}, fmt.Errorf("child issue already linked: %s", u)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.IssueLinks = append(record.IssueLinks, IssueOpsIssueLink{
		Type:      "child",
		URL:       u,
		Title:     strings.TrimSpace(title),
		Provider:  remote.ProviderFromURL(u),
		CreatedAt: now,
	})
	return touchAndWriteIssueOps(stateRoot, record)
}

func validateIssueURL(issueURL string) error {
	if issueURL == "" {
		return fmt.Errorf("issue_url is required")
	}
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("issue_url must be an http(s) URL")
	}
	return nil
}
