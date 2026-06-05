package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IssueOpsPhase string

const (
	IssueOpsPhaseProblem     IssueOpsPhase = "problem"
	IssueOpsPhaseGrill       IssueOpsPhase = "grill"
	IssueOpsPhasePlan        IssueOpsPhase = "plan"
	IssueOpsPhaseImplement   IssueOpsPhase = "implement"
	IssueOpsPhaseAISlopClean IssueOpsPhase = "ai-slop-clean"
	IssueOpsPhaseFeedback    IssueOpsPhase = "feedback"
	IssueOpsPhasePR          IssueOpsPhase = "pr"
	IssueOpsPhaseDone        IssueOpsPhase = "done"
)

// IssueOpsPhases lists every known IssueOps phase in lifecycle order, mirroring
// the SKILL.md required phases (problem intake, domain grill, issue/plan,
// implementation, AI slop cleanup, feedback, PR/MR, done).
var IssueOpsPhases = []IssueOpsPhase{
	IssueOpsPhaseProblem,
	IssueOpsPhaseGrill,
	IssueOpsPhasePlan,
	IssueOpsPhaseImplement,
	IssueOpsPhaseAISlopClean,
	IssueOpsPhaseFeedback,
	IssueOpsPhasePR,
	IssueOpsPhaseDone,
}

func knownIssueOpsPhase(phase IssueOpsPhase) bool {
	for _, known := range IssueOpsPhases {
		if known == phase {
			return true
		}
	}
	return false
}

type IssueOpsStartRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
}

type IssueOpsFeedbackItem struct {
	Source         string `json:"source"`
	Body           string `json:"body"`
	Classification string `json:"classification,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type IssueOpsIssueLink struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Provider  string `json:"provider,omitempty"`
	CreatedAt string `json:"created_at"`
}

type IssueOpsBranchPrepareStep struct {
	Order         int            `json:"order"`
	Strategy      string         `json:"strategy"`
	Tool          string         `json:"tool,omitempty"`
	ToolArguments map[string]any `json:"tool_arguments,omitempty"`
	Command       []string       `json:"command,omitempty"`
	Description   string         `json:"description"`
}

type IssueOpsBranchPrepare struct {
	Provider        string                      `json:"provider"`
	IssueURL        string                      `json:"issue_url"`
	Branch          string                      `json:"branch"`
	BaseBranch      string                      `json:"base_branch"`
	BaseSHA         string                      `json:"base_sha,omitempty"`
	RemoteBranchURL string                      `json:"remote_branch_url,omitempty"`
	LinkVerified    bool                        `json:"link_verified"`
	Steps           []IssueOpsBranchPrepareStep `json:"steps"`
	CreatedAt       string                      `json:"created_at"`
}

type IssueOpsBranchPrepareRequest struct {
	Provider        string `json:"provider"`
	IssueURL        string `json:"issue_url"`
	Branch          string `json:"branch"`
	BaseBranch      string `json:"base_branch"`
	BaseSHA         string `json:"base_sha,omitempty"`
	RemoteBranchURL string `json:"remote_branch_url,omitempty"`
	LinkVerified    bool   `json:"link_verified,omitempty"`
}

type IssueOpsRecord struct {
	OK            bool                   `json:"ok"`
	ID            string                 `json:"id"`
	Repo          string                 `json:"repo"`
	Branch        string                 `json:"branch,omitempty"`
	Phase         IssueOpsPhase          `json:"phase"`
	IssueURL      string                 `json:"issue_url,omitempty"`
	PlanPath      string                 `json:"plan_path,omitempty"`
	WorktreePath  string                 `json:"worktree_path,omitempty"`
	IssueLinks    []IssueOpsIssueLink    `json:"issue_links,omitempty"`
	BranchPrepare *IssueOpsBranchPrepare `json:"branch_prepare,omitempty"`
	Feedback      []IssueOpsFeedbackItem `json:"feedback,omitempty"`
	AISlopCleanAt string                 `json:"ai_slop_clean_at,omitempty"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

type IssueOpsReadiness struct {
	OK           bool     `json:"ok"`
	Ready        bool     `json:"ready"`
	Strict       bool     `json:"strict,omitempty"`
	Missing      []string `json:"missing"`
	IssueURL     string   `json:"issue_url,omitempty"`
	PlanPath     string   `json:"plan_path,omitempty"`
	WorktreePath string   `json:"worktree_path,omitempty"`
	Branch       string   `json:"branch,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("repo is required")
	}
	branch := strings.TrimSpace(req.Branch)
	if err := validateIssueOpsIssueBranch(branch); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	id := newIssueOpsID(repo, branch)
	// Identity is deterministic per (repo, branch): resume an existing record
	// instead of minting a new one so cycles cannot accumulate as stale duplicates.
	if existing, err := ReadIssueOps(stateRoot, id); err == nil {
		return existing, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    branch,
		Phase:     IssueOpsPhaseProblem,
		Feedback:  []IssueOpsFeedbackItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return writeIssueOps(stateRoot, record)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	path := issueopsPath(stateRoot, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: file has %q", record.ID)
	}
	record.OK = true
	return record, nil
}

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
	record.Phase = IssueOpsPhasePlan
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
	if !issueOpsPlanPathExists(record.Repo, path) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path does not exist: %s", path)
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
	if parentProvider := issueOpsProviderFromURL(record.IssueURL); parentProvider != "" && issueOpsProviderFromURL(u) != parentProvider {
		return IssueOpsRecord{OK: false}, fmt.Errorf("child issue provider must match linked parent issue provider")
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
		Provider:  issueOpsProviderFromURL(u),
		CreatedAt: now,
	})
	return touchAndWriteIssueOps(stateRoot, record)
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	issueURL := strings.TrimSpace(req.IssueURL)
	if issueURL == "" {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return record, err
		}
		issueURL = strings.TrimSpace(record.IssueURL)
	}
	if err := validateIssueURL(issueURL); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	if provider == "" {
		provider = issueOpsProviderFromURL(issueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("provider must be github or gitlab")
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("branch is required")
	}
	if err := validateIssueOpsIssueBranch(branch); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	baseBranch := strings.TrimSpace(req.BaseBranch)
	if baseBranch == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("base_branch is required")
	}
	if provider == "gitlab" {
		if issueNumber := issueOpsIssueNumber(issueURL); issueNumber != "" {
			if !strings.HasPrefix(branch, issueNumber+"-") {
				return IssueOpsRecord{OK: false}, fmt.Errorf("gitlab branch for issue %s must start with %s- so GitLab links it in the issue Development section; for example %s-fix-login", issueNumber, issueNumber, issueNumber)
			}
		}
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("issue must be linked before branch prepare")
	}
	if strings.TrimSpace(record.Branch) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("issueops record must be started with branch before branch prepare")
	}
	if record.Branch != branch {
		return IssueOpsRecord{OK: false}, fmt.Errorf("branch does not match IssueOps record branch")
	}
	if record.IssueURL != issueURL {
		return IssueOpsRecord{OK: false}, fmt.Errorf("issue_url does not match linked IssueOps issue")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.BranchPrepare = &IssueOpsBranchPrepare{
		Provider:        provider,
		IssueURL:        issueURL,
		Branch:          branch,
		BaseBranch:      baseBranch,
		BaseSHA:         strings.TrimSpace(req.BaseSHA),
		RemoteBranchURL: strings.TrimSpace(req.RemoteBranchURL),
		LinkVerified:    req.LinkVerified,
		Steps:           issueOpsBranchPrepareSteps(provider, issueURL, branch, baseBranch),
		CreatedAt:       now,
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func validateIssueOpsIssueBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	if strings.ContainsAny(branch, " \t\r\n") || strings.HasPrefix(branch, "/") || strings.Contains(branch, "..") {
		return fmt.Errorf("issueops branch contains invalid characters: %s", branch)
	}
	issueNumber, slug, ok := strings.Cut(branch, "-")
	if !ok || strings.TrimSpace(slug) == "" || !isDecimalString(issueNumber) {
		return fmt.Errorf("issueops branch must start with the issue number followed by a hyphen so GitLab links it; use names like 2387-fix-grpc-ai-dmm-tag-replication-lag or 2388-fanza-delete-404-stale-registered")
	}
	return nil
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (IssueOpsRecord, error) {
	source = strings.TrimSpace(source)
	body = strings.TrimSpace(body)
	classification = strings.TrimSpace(classification)
	if source == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback source is required")
	}
	if body == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback body is required")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback = append(record.Feedback, IssueOpsFeedbackItem{Source: source, Body: body, Classification: classification, CreatedAt: now})
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		record.Phase = IssueOpsPhaseFeedback
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func issueOpsBranchPrepareSteps(provider, issueURL, branch, baseBranch string) []IssueOpsBranchPrepareStep {
	switch provider {
	case "gitlab":
		return []IssueOpsBranchPrepareStep{
			{
				Order:    1,
				Strategy: "mcp",
				Tool:     "mcp__glab.glab_api",
				ToolArguments: map[string]any{
					"endpoint": "projects/:fullpath/repository/branches",
					"method":   "POST",
					"field":    []string{"branch=" + branch, "ref=" + baseBranch},
				},
				Description: "Create the issue-prefixed branch through the GitLab MCP authenticated API tool.",
			},
			{
				Order:       2,
				Strategy:    "fallback_api",
				Command:     []string{"glab", "api", "projects/:fullpath/repository/branches", "-X", "POST", "-f", "branch=" + branch, "-f", "ref=" + baseBranch},
				Description: "Fallback to the GitLab API through glab when the MCP tool is unavailable or fails.",
			},
			{
				Order:       3,
				Strategy:    "fail",
				Description: "Stop the IssueOps branch preparation if neither provider-linked creation path succeeds.",
			},
		}
	case "github":
		return []IssueOpsBranchPrepareStep{
			{
				Order:       1,
				Strategy:    "mcp_unavailable",
				Description: "No GitHub MCP branch-creation tool is currently exposed in this harness session; do not silently create a local branch.",
			},
			{
				Order:       2,
				Strategy:    "fallback_api",
				Command:     []string{"gh", "issue", "develop", issueURL, "--base", baseBranch, "--name", branch},
				Description: "Create a GitHub linked development branch through gh issue develop.",
			},
			{
				Order:       3,
				Strategy:    "fail",
				Description: "Stop the IssueOps branch preparation if the linked development branch cannot be created.",
			},
		}
	default:
		return nil
	}
}

// AdvanceIssueOpsPhase moves an IssueOps loop to an explicitly named phase. The
// workflow is advisory until code-review/remote-artifact boundaries. The hard
// gates are that ai-slop-clean requires a concrete worktree to inspect, and PR
// phase requires strict PR readiness.
func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	phase := IssueOpsPhase(strings.TrimSpace(to))
	if !knownIssueOpsPhase(phase) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if phase == IssueOpsPhaseAISlopClean {
		if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseFeedback && strings.TrimSpace(record.AISlopCleanAt) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
	}
	if phase == IssueOpsPhasePR {
		if ready := IssueOpsStrictPRReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseDone && record.Phase != IssueOpsPhasePR {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter done phase before pr phase")
	}
	record.Phase = phase
	if phase == IssueOpsPhaseAISlopClean && strings.TrimSpace(record.AISlopCleanAt) == "" {
		record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func IssueOpsAISlopCleanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      uniqSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean")
	}
	missing = uniqSorted(missing)
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      missing,
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func issueOpsBaseImplementationMissing(record IssueOpsRecord) []string {
	missing := issueOpsBranchEvidenceMissing(record)
	if strings.TrimSpace(record.PlanPath) == "" {
		missing = append(missing, "plan_path")
	}
	return missing
}

func issueOpsPlanExistenceRoot(record IssueOpsRecord) string {
	if worktree := strings.TrimSpace(record.WorktreePath); worktree != "" {
		return worktree
	}
	return strings.TrimSpace(record.Repo)
}

func issueOpsBranchEvidenceMissing(record IssueOpsRecord) []string {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if record.BranchPrepare == nil {
		missing = append(missing, "branch_prepare")
	} else if !record.BranchPrepare.LinkVerified {
		missing = append(missing, "branch_link_verified")
	}
	return missing
}

func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsPRReadiness(record)
	ready.Strict = true
	missing := append([]string{}, ready.Missing...)
	warnings := []string{}

	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		missing = append(missing, "repo")
	} else if code, out, _ := GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		missing = append(missing, "repo_git")
	} else {
		branch := strings.TrimSpace(GitOut(gitRoot, "branch", "--show-current"))
		if strings.TrimSpace(record.Branch) != "" && branch != strings.TrimSpace(record.Branch) {
			missing = append(missing, "branch_match")
			warnings = append(warnings, "current branch "+branch+" does not match IssueOps branch "+strings.TrimSpace(record.Branch))
		}
		if strings.TrimSpace(GitOut(gitRoot, "status", "--porcelain=v1")) != "" {
			missing = append(missing, "worktree_clean")
		}
		upstream := strings.TrimSpace(GitOut(gitRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
		if upstream == "" {
			missing = append(missing, "upstream")
		} else {
			counts := strings.Fields(GitOut(gitRoot, "rev-list", "--left-right", "--count", "HEAD...@{u}"))
			if len(counts) != 2 || counts[0] != "0" || counts[1] != "0" {
				missing = append(missing, "upstream_synced")
				if len(counts) == 2 {
					warnings = append(warnings, "branch divergence against upstream: ahead="+counts[0]+" behind="+counts[1])
				}
			}
		}
	}

	if path := strings.TrimSpace(record.PlanPath); path != "" && !issueOpsPlanPathExists(gitRoot, path) {
		missing = append(missing, "plan_exists")
	}
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}

	ready.Missing = uniqSorted(missing)
	ready.Warnings = warnings
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func issueOpsStrictGitRoot(record IssueOpsRecord) string {
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(record.Repo)
}

func issueOpsWorktreePathValid(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func validateIssueOpsIsolatedWorktreePath(record IssueOpsRecord, path string) error {
	repo := cleanAbsPath(record.Repo)
	worktree := cleanAbsPath(path)
	if repo == "" || worktree == "" {
		return fmt.Errorf("worktree_path and repo must be absolute or resolvable paths")
	}
	if worktree == repo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	parent := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees")
	if !pathWithin(worktree, parent) {
		return fmt.Errorf("worktree_path must be under sibling worktree directory: %s", parent)
	}
	return nil
}

func issueOpsPlanPathExists(repo, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(strings.TrimSpace(repo), path)
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func IssueOpsStateRoot() string {
	return filepath.Join(StateDir(), "issueops")
}

// ActiveIssueOpsCycleForBranch loads the single deterministic cycle for the given
// (repo, branch) and reports it when it is not done. Because the id is derived
// only from (repo, branch), the guard reads exactly one record — the current
// work's cycle — and legacy timestamped records are never consulted, so stale
// state cannot cause a false lock.
func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return IssueOpsRecord{}, false
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), newIssueOpsID(repo, branch))
	if err != nil {
		return IssueOpsRecord{}, false
	}
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{}, false
	}
	if issueOpsPlanBranchMismatchesRecord(record) {
		return IssueOpsRecord{}, false
	}
	return record, true
}

func issueOpsPlanBranchMismatchesRecord(record IssueOpsRecord) bool {
	planPath := cleanAbsPath(record.PlanPath)
	repo := cleanAbsPath(record.Repo)
	if planPath == "" || repo == "" || pathWithin(planPath, repo) || !isInsideWorktreesPath(planPath) {
		return false
	}
	branch := gitBranchFromAncestor(planPath)
	return branch != "" && branch != strings.TrimSpace(record.Branch)
}

func gitBranchFromAncestor(path string) string {
	current := cleanAbsPath(path)
	if current == "" {
		return ""
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return gitBranchFromHead(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// IssueOpsPhaseExpectsWorktree reports whether a phase is a code-editing phase
// for which isolated-worktree work is expected.
func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	switch phase {
	case IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback, IssueOpsPhasePR:
		return true
	default:
		return false
	}
}

func newIssueOpsID(repo, branch string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(branch)))
	return "io-" + hex.EncodeToString(sum[:])[:12]
}

func touchAndWriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeIssueOps(stateRoot, record)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, err
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		record.OK = false
		return record, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, err
	}
	path := issueopsPath(stateRoot, record.ID)
	tmp, err := os.CreateTemp(stateRoot, "."+record.ID+"-*.tmp")
	if err != nil {
		record.OK = false
		return record, err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(b); err != nil {
			return err
		}
		if _, err := tmp.Write([]byte{'\n'}); err != nil {
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		record.OK = false
		return record, writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		record.OK = false
		return record, err
	}
	return record, nil
}

func issueopsPath(stateRoot, id string) string {
	return filepath.Join(stateRoot, id+".json")
}

func normalizeIssueOpsID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if !strings.HasPrefix(id, "io-") {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
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

func issueOpsProviderFromURL(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	if host == "github.com" && strings.Contains(path, "/issues/") {
		return "github"
	}
	if strings.Contains(host, "gitlab") || strings.Contains(path, "/-/issues/") {
		return "gitlab"
	}
	return ""
}

func issueOpsIssueNumber(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if part == "issues" && i+1 < len(parts) {
			number := parts[i+1]
			for _, r := range number {
				if r < '0' || r > '9' {
					return ""
				}
			}
			return number
		}
	}
	return ""
}
