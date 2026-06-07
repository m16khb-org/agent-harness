package linking

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
)

type Store struct {
	Read                   func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite             func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanReadiness          func(record model.IssueOpsRecord) model.IssueOpsReadiness
	PhaseRank              func(phase model.IssueOpsPhase) int
	BranchEvidenceMissing  func(record model.IssueOpsRecord) []string
	DesignReviewMissing    func(record model.IssueOpsRecord) []string
	PlanPathExists         func(repo, path string) bool
	PlanPathInsideWorktree func(worktree, planPath string) bool
	WorktreePathValid      func(path string) bool
	UniqueSorted           func(values []string) []string
}

func LinkIssue(store Store, stateRoot, id, issueURL string) (model.IssueOpsRecord, error) {
	u := strings.TrimSpace(issueURL)
	if err := ValidateIssueURL(u); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.IssueURL = u
	if ready := store.PlanReadiness(record); ready.Ready && store.PhaseRank(record.Phase) < store.PhaseRank(model.IssueOpsPhasePlan) {
		record.Phase = model.IssueOpsPhasePlan
	}
	return store.TouchWrite(stateRoot, record)
}

func LinkPlan(store Store, stateRoot, id, planPath string) (model.IssueOpsRecord, error) {
	path := strings.TrimSpace(planPath)
	if path == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must not contain path traversal")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := store.BranchEvidenceMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before linked worktree")
	}
	if missing := store.DesignReviewMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link plan before approved design review: missing %s", strings.Join(store.UniqueSorted(missing), ", "))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, path)
	}
	if !store.PlanPathExists(record.Repo, path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path does not exist: %s", path)
	}
	if !store.PlanPathInsideWorktree(worktree, path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", worktree)
	}
	record.PlanPath = path
	record.Phase = model.IssueOpsPhaseImplement
	return store.TouchWrite(stateRoot, record)
}

func LinkWorktree(store Store, stateRoot, id, worktreePath string) (model.IssueOpsRecord, error) {
	path := strings.TrimSpace(worktreePath)
	if path == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path must not contain path traversal")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if missing := store.BranchEvidenceMissing(record); len(missing) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link worktree before branch evidence: missing %s", strings.Join(missing, ", "))
	}
	if !store.WorktreePathValid(path) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("worktree_path does not exist or is not a directory: %s", path)
	}
	if err := ValidateIsolatedWorktreePath(record, path); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if err := ValidateWorktreeBranch(record, path); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if planPath := strings.TrimSpace(record.PlanPath); planPath != "" && !store.PlanPathInsideWorktree(path, planPath) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must be inside linked worktree: %s", path)
	}
	record.WorktreePath = path
	return store.TouchWrite(stateRoot, record)
}

func LinkChild(store Store, stateRoot, id, childURL, title string) (model.IssueOpsRecord, error) {
	u := strings.TrimSpace(childURL)
	if err := ValidateIssueURL(u); err != nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("child_url %s", strings.TrimPrefix(err.Error(), "issue_url "))
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot link child before linked parent issue")
	}
	if parentProvider := remote.ProviderFromURL(record.IssueURL); parentProvider != "" && remote.ProviderFromURL(u) != parentProvider {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("child issue provider must match linked parent issue provider")
	}
	if err := remote.ValidateChildMatchesParent(record.IssueURL, u); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && link.URL == u {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("child issue already linked: %s", u)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.IssueLinks = append(record.IssueLinks, model.IssueOpsIssueLink{
		Type:      "child",
		URL:       u,
		Title:     strings.TrimSpace(title),
		Provider:  remote.ProviderFromURL(u),
		CreatedAt: now,
	})
	return store.TouchWrite(stateRoot, record)
}

func ValidateIssueURL(issueURL string) error {
	if issueURL == "" {
		return fmt.Errorf("issue_url is required")
	}
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("issue_url must be an http(s) URL")
	}
	return nil
}
