package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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

func issueOpsPhaseRank(phase IssueOpsPhase) int {
	for i, known := range IssueOpsPhases {
		if known == phase {
			return i + 1
		}
	}
	return 0
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
	IssueUpdatedAt string `json:"issue_updated_at,omitempty"`
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

type IssueOpsRemoteArtifactVerification struct {
	Provider   string   `json:"provider"`
	Kind       string   `json:"kind"`
	URL        string   `json:"url"`
	Labels     []string `json:"labels"`
	Assignees  []string `json:"assignees"`
	VerifiedAt string   `json:"verified_at"`
}

type IssueOpsRemoteArtifactVerificationRequest struct {
	Provider  string
	Kind      string
	URL       string
	Labels    []string
	Assignees []string
}

type IssueOpsRecord struct {
	OK                     bool                                `json:"ok"`
	ID                     string                              `json:"id"`
	Repo                   string                              `json:"repo"`
	Branch                 string                              `json:"branch,omitempty"`
	Phase                  IssueOpsPhase                       `json:"phase"`
	IssueURL               string                              `json:"issue_url,omitempty"`
	PlanPath               string                              `json:"plan_path,omitempty"`
	WorktreePath           string                              `json:"worktree_path,omitempty"`
	IssueLinks             []IssueOpsIssueLink                 `json:"issue_links,omitempty"`
	BranchPrepare          *IssueOpsBranchPrepare              `json:"branch_prepare,omitempty"`
	RemoteArtifact         *IssueOpsRemoteArtifactVerification `json:"remote_artifact,omitempty"`
	Feedback               []IssueOpsFeedbackItem              `json:"feedback,omitempty"`
	AISlopCleanAt          string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead        string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	CreatedAt              string                              `json:"created_at"`
	UpdatedAt              string                              `json:"updated_at"`
}

type IssueOpsReadiness struct {
	OK                     bool     `json:"ok"`
	Ready                  bool     `json:"ready"`
	Strict                 bool     `json:"strict,omitempty"`
	Missing                []string `json:"missing"`
	IssueURL               string   `json:"issue_url,omitempty"`
	PlanPath               string   `json:"plan_path,omitempty"`
	WorktreePath           string   `json:"worktree_path,omitempty"`
	Branch                 string   `json:"branch,omitempty"`
	AISlopCleanHead        string   `json:"ai_slop_clean_head,omitempty"`
	CurrentHead            string   `json:"current_head,omitempty"`
	AISlopCleanFingerprint string   `json:"ai_slop_clean_fingerprint,omitempty"`
	CurrentFingerprint     string   `json:"current_fingerprint,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type IssueOpsCleanupStatusRequest struct {
	Merged bool `json:"merged"`
}

type IssueOpsCleanupStatus struct {
	OK                bool     `json:"ok"`
	Ready             bool     `json:"ready"`
	ID                string   `json:"id"`
	Merged            bool     `json:"merged"`
	Missing           []string `json:"missing"`
	Warnings          []string `json:"warnings,omitempty"`
	Choices           []string `json:"choices"`
	WorktreePath      string   `json:"worktree_path,omitempty"`
	Branch            string   `json:"branch,omitempty"`
	RemoteArtifactURL string   `json:"remote_artifact_url,omitempty"`
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
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePlan) {
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
	if parentProvider := issueOpsProviderFromURL(record.IssueURL); parentProvider != "" && issueOpsProviderFromURL(u) != parentProvider {
		return IssueOpsRecord{OK: false}, fmt.Errorf("child issue provider must match linked parent issue provider")
	}
	if err := validateIssueOpsChildMatchesParent(record.IssueURL, u); err != nil {
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
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot add feedback after %s phase", record.Phase)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback = append(record.Feedback, IssueOpsFeedbackItem{Source: source, Body: body, Classification: classification, CreatedAt: now})
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		record.Phase = IssueOpsPhaseFeedback
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	marked := false
	for i := range record.Feedback {
		if issueOpsFeedbackRequiresIssueUpdate(record.Feedback[i]) {
			record.Feedback[i].IssueUpdatedAt = now
			marked = true
		}
	}
	if !marked {
		return IssueOpsRecord{OK: false}, fmt.Errorf("no unresolved contract_change feedback requires a remote issue update")
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase != IssueOpsPhasePR {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot verify remote artifact before pr phase")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "github" && provider != "gitlab" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if issueProvider := issueOpsProviderFromURL(record.IssueURL); issueProvider != "" && provider != issueProvider {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact provider must match linked issue provider")
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	if kind != "pr" && kind != "mr" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact kind must be pr or mr")
	}
	if provider == "github" && kind != "pr" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("github remote artifact kind must be pr")
	}
	if provider == "gitlab" && kind != "mr" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("gitlab remote artifact kind must be mr")
	}
	artifactURL := strings.TrimSpace(req.URL)
	if err := validateRemoteArtifactURL(artifactURL, provider, kind); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	if err := validateRemoteArtifactMatchesIssue(record.IssueURL, artifactURL, provider, kind); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	labels := cleanIssueOpsRemoteValues(req.Labels)
	if len(labels) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact labels are required")
	}
	assignees := cleanIssueOpsRemoteValues(req.Assignees)
	if len(assignees) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact assignees are required")
	}
	if invalid := invalidIssueOpsRemoteAssignee(assignees); invalid != "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact assignee must be a verified provider user, not placeholder %q", invalid)
	}
	record.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
		Provider:   provider,
		Kind:       kind,
		URL:        artifactURL,
		Labels:     labels,
		Assignees:  assignees,
		VerifiedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	return touchAndWriteIssueOps(stateRoot, record)
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
	if record.Phase == phase {
		if phase == IssueOpsPhaseAISlopClean {
			if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
				return IssueOpsRecord{OK: false}, fmt.Errorf("cannot refresh ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
			}
			record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
			record.AISlopCleanHead = issueOpsCurrentHead(record)
			record.AISlopCleanFingerprint = issueOpsChangeFingerprint(record)
			return touchAndWriteIssueOps(stateRoot, record)
		}
		return record, nil
	}
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot leave done phase")
	}
	if issueOpsPhaseRank(phase) < issueOpsPhaseRank(record.Phase) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot move issueops phase backward from %s to %s", record.Phase, phase)
	}
	if phase == IssueOpsPhaseImplement {
		if ready := IssueOpsImplementationReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter implement phase: missing %s", strings.Join(ready.Missing, ", "))
		}
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
	if phase == IssueOpsPhaseDone {
		if missing := issueOpsRemoteArtifactMissing(record); len(missing) > 0 {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
		}
	}
	record.Phase = phase
	if phase == IssueOpsPhaseAISlopClean && strings.TrimSpace(record.AISlopCleanAt) == "" {
		record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if phase == IssueOpsPhaseAISlopClean {
		record.AISlopCleanHead = issueOpsCurrentHead(record)
		record.AISlopCleanFingerprint = issueOpsChangeFingerprint(record)
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func issueOpsRemoteArtifactMissing(record IssueOpsRecord) []string {
	if record.RemoteArtifact == nil {
		return []string{"remote_artifact"}
	}
	missing := []string{}
	if strings.TrimSpace(record.RemoteArtifact.Provider) == "" {
		missing = append(missing, "remote_artifact_provider")
	}
	if strings.TrimSpace(record.RemoteArtifact.Kind) == "" {
		missing = append(missing, "remote_artifact_kind")
	}
	if strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact_url")
	}
	if len(cleanIssueOpsRemoteValues(record.RemoteArtifact.Labels)) == 0 {
		missing = append(missing, "remote_artifact_labels")
	}
	if len(cleanIssueOpsRemoteValues(record.RemoteArtifact.Assignees)) == 0 {
		missing = append(missing, "remote_artifact_assignees")
	}
	return uniqSorted(missing)
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req IssueOpsCleanupStatusRequest) (IssueOpsCleanupStatus, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return IssueOpsCleanupStatus{OK: false, ID: id}, err
	}
	return IssueOpsCleanupStatusForRecord(record, req), nil
}

func IssueOpsCleanupStatusForRecord(record IssueOpsRecord, req IssueOpsCleanupStatusRequest) IssueOpsCleanupStatus {
	status := IssueOpsCleanupStatus{
		OK:           true,
		ID:           record.ID,
		Merged:       req.Merged,
		WorktreePath: strings.TrimSpace(record.WorktreePath),
		Branch:       strings.TrimSpace(record.Branch),
	}
	if record.RemoteArtifact != nil {
		status.RemoteArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePR) {
		status.Missing = append(status.Missing, "pr_phase")
	}
	status.Missing = append(status.Missing, issueOpsRemoteArtifactMissing(record)...)
	if !req.Merged {
		status.Missing = append(status.Missing, "remote_artifact_merged")
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		status.Missing = append(status.Missing, "worktree_path")
		return finishIssueOpsCleanupStatus(status)
	}
	if !issueOpsWorktreePathValid(worktree) {
		status.Missing = append(status.Missing, "worktree_exists")
		return finishIssueOpsCleanupStatus(status)
	}
	if code, out, stderr := GitCmd(worktree, "status", "--porcelain=v1"); code != 0 {
		status.Missing = append(status.Missing, "worktree_git_status")
		if strings.TrimSpace(stderr) != "" {
			status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
		}
	} else if strings.TrimSpace(out) != "" {
		status.Missing = append(status.Missing, "worktree_dirty")
	}
	actualBranch := strings.TrimSpace(GitOut(worktree, "branch", "--show-current"))
	if actualBranch == "" {
		status.Missing = append(status.Missing, "branch")
	} else if strings.TrimSpace(record.Branch) != "" && actualBranch != strings.TrimSpace(record.Branch) {
		status.Missing = append(status.Missing, "branch_match")
	}
	remote := firstIssueOpsGitRemote(worktree)
	if remote == "" {
		status.Missing = append(status.Missing, "remote_branch_check_unavailable")
	} else if actualBranch != "" {
		if code, out, stderr := GitCmd(worktree, "ls-remote", "--heads", remote, actualBranch); code != 0 {
			status.Missing = append(status.Missing, "remote_branch_check_failed")
			if strings.TrimSpace(stderr) != "" {
				status.Warnings = append(status.Warnings, strings.TrimSpace(stderr))
			}
		} else if strings.TrimSpace(out) != "" {
			status.Missing = append(status.Missing, "remote_branch_present")
		}
	}
	return finishIssueOpsCleanupStatus(status)
}

func finishIssueOpsCleanupStatus(status IssueOpsCleanupStatus) IssueOpsCleanupStatus {
	status.Missing = uniqSorted(status.Missing)
	status.Warnings = uniqSorted(status.Warnings)
	status.Ready = len(status.Missing) == 0
	if status.Ready {
		status.Choices = []string{
			"1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 확인합니다.",
			"3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	} else {
		status.Choices = []string{
			"1. 차단 해소: missing 항목의 merge/worktree/remote branch 증거를 먼저 보강합니다. (추천)",
			"2. 보류: worktree는 유지하고 나중에 다시 확인합니다.",
			"3. 확장 점검: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.",
		}
	}
	return status
}

func firstIssueOpsGitRemote(worktree string) string {
	for _, remote := range strings.Fields(GitOut(worktree, "remote")) {
		remote = strings.TrimSpace(remote)
		if remote != "" {
			return remote
		}
	}
	return ""
}

func IssueOpsAISlopCleanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if !issueOpsHasImplementationEvidence(record) {
		missing = append(missing, "implementation_changes")
	}
	missing = uniqSorted(missing)
	ready.Missing = missing
	ready.Ready = len(missing) == 0
	return ready
}

func IssueOpsImplementationReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
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

func issueOpsHasImplementationEvidence(record IssueOpsRecord) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !issueOpsWorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		if issueOpsGitStatusHasImplementationChange(record, worktree) {
			return true
		}
		return issueOpsGitHeadDiffersFromBase(record, worktree)
	}
	return issueOpsFileTreeHasImplementationChange(record, worktree)
}

func issueOpsCurrentHead(record IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := GitCmd(gitRoot, "rev-parse", "HEAD"); code == 0 {
		return strings.TrimSpace(out)
	}
	return ""
}

func issueOpsChangeFingerprint(record IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		return ""
	}
	paths := map[string]bool{}
	if base := issueOpsDiffBaseRef(record, gitRoot); base != "" {
		_, names, _ := GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			if path := cleanIssueOpsRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	status := GitOut(gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(status, "\n") {
		if path := cleanIssueOpsRelativePath(issueOpsPorcelainPath(line)); path != "" {
			paths[path] = true
		}
	}
	if len(paths) == 0 {
		return ""
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	var b strings.Builder
	b.WriteString("issueops-ai-slop-clean:v1\n")
	for _, rel := range ordered {
		abs := filepath.Join(gitRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			b.WriteString(rel + "\x00deleted\n")
			continue
		}
		if info.IsDir() {
			continue
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return ""
		}
		sum := sha256.Sum256(content)
		b.WriteString(rel + "\x00" + hex.EncodeToString(sum[:]) + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func issueOpsDiffBaseRef(record IssueOpsRecord, gitRoot string) string {
	if record.BranchPrepare == nil {
		return ""
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return ""
	}
	for _, ref := range []string{"origin/" + base, base} {
		if code, _, _ := GitCmd(gitRoot, "rev-parse", "--verify", ref+"^{commit}"); code == 0 {
			return ref
		}
	}
	return ""
}

func cleanIssueOpsRelativePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return ""
	}
	return filepath.ToSlash(path)
}

func issueOpsGitStatusHasImplementationChange(record IssueOpsRecord, worktree string) bool {
	out := GitOut(worktree, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(out, "\n") {
		path := issueOpsPorcelainPath(line)
		if path == "" {
			continue
		}
		if !issueOpsPathMatchesPlan(record, worktree, path) {
			return true
		}
	}
	return false
}

func issueOpsGitHeadDiffersFromBase(record IssueOpsRecord, worktree string) bool {
	base := ""
	if record.BranchPrepare != nil {
		base = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	if base == "" {
		return false
	}
	for _, ref := range []string{"origin/" + base, base} {
		if code, _, _ := GitCmd(worktree, "rev-parse", "--verify", ref+"^{commit}"); code != 0 {
			continue
		}
		_, names, _ := GitCmd(worktree, "diff", "--name-only", ref+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			name = strings.TrimSpace(name)
			if name != "" && !issueOpsPathMatchesPlan(record, worktree, name) {
				return true
			}
		}
		return false
	}
	return false
}

func issueOpsFileTreeHasImplementationChange(record IssueOpsRecord, worktree string) bool {
	found := false
	_ = filepath.WalkDir(worktree, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !issueOpsPathMatchesPlan(record, worktree, path) {
			found = true
		}
		return nil
	})
	return found
}

func issueOpsPorcelainPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if len(line) < 4 {
		return ""
	}
	path := strings.TrimSpace(line[3:])
	if renamed := strings.LastIndex(path, " -> "); renamed >= 0 {
		path = strings.TrimSpace(path[renamed+4:])
	}
	return strings.Trim(path, `"`)
}

func issueOpsPathMatchesPlan(record IssueOpsRecord, worktree, path string) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	if planPath == "" || path == "" {
		return false
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(worktree, filepath.FromSlash(planPath))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, filepath.FromSlash(path))
	}
	planPath = cleanAbsPath(planPath)
	path = cleanAbsPath(path)
	return path == planPath
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean")
	}
	if issueOpsHasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
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

func issueOpsHasUnresolvedContractFeedback(record IssueOpsRecord) bool {
	for _, item := range record.Feedback {
		if issueOpsFeedbackRequiresIssueUpdate(item) {
			return true
		}
	}
	return false
}

func issueOpsFeedbackRequiresIssueUpdate(item IssueOpsFeedbackItem) bool {
	return strings.EqualFold(strings.TrimSpace(item.Classification), "contract_change") &&
		strings.TrimSpace(item.IssueUpdatedAt) == ""
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
	currentHead := ""
	currentFingerprint := ""

	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		missing = append(missing, "repo")
	} else if code, out, _ := GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		missing = append(missing, "repo_git")
	} else {
		currentHead = issueOpsCurrentHead(record)
		currentFingerprint = issueOpsChangeFingerprint(record)
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
			if code, _, stderr := GitCmd(gitRoot, "fetch", "--quiet"); code != 0 {
				missing = append(missing, "upstream_fetch")
				if strings.TrimSpace(stderr) != "" {
					warnings = append(warnings, "failed to fetch upstream: "+strings.TrimSpace(stderr))
				}
			}
			counts := strings.Fields(GitOut(gitRoot, "rev-list", "--left-right", "--count", "HEAD...@{u}"))
			if len(counts) != 2 || counts[0] != "0" || counts[1] != "0" {
				missing = append(missing, "upstream_synced")
				if len(counts) == 2 {
					warnings = append(warnings, "branch divergence against upstream: ahead="+counts[0]+" behind="+counts[1])
				}
			}
		}
	}
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		storedFingerprint := strings.TrimSpace(record.AISlopCleanFingerprint)
		if storedFingerprint == "" && currentFingerprint != "" {
			missing = append(missing, "ai_slop_clean_fingerprint")
		} else if storedFingerprint != "" && currentFingerprint == "" {
			missing = append(missing, "current_fingerprint")
		} else if storedFingerprint != "" && storedFingerprint != currentFingerprint {
			missing = append(missing, "ai_slop_clean_stale")
		}
	}

	if path := strings.TrimSpace(record.PlanPath); path != "" && !issueOpsPlanPathExists(gitRoot, path) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}

	ready.Missing = uniqSorted(missing)
	ready.Warnings = warnings
	ready.AISlopCleanHead = record.AISlopCleanHead
	ready.CurrentHead = currentHead
	ready.AISlopCleanFingerprint = record.AISlopCleanFingerprint
	ready.CurrentFingerprint = currentFingerprint
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
	info, err := os.Lstat(worktree)
	if err != nil {
		return fmt.Errorf("worktree_path does not exist: %s", worktree)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("worktree_path must not be a symlink: %s", worktree)
	}
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return fmt.Errorf("source checkout path cannot be resolved: %s", repo)
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return fmt.Errorf("worktree_path cannot be resolved: %s", worktree)
	}
	resolvedParent := filepath.Join(filepath.Dir(resolvedRepo), filepath.Base(resolvedRepo)+".worktrees")
	if resolvedWorktree == resolvedRepo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	if !pathWithin(resolvedWorktree, resolvedParent) {
		return fmt.Errorf("worktree_path must resolve under sibling worktree directory: %s", resolvedParent)
	}
	return nil
}

func validateIssueOpsWorktreeBranch(record IssueOpsRecord, path string) error {
	expected := strings.TrimSpace(record.Branch)
	if expected == "" {
		return nil
	}
	actual := strings.TrimSpace(gitBranchFromHead(path))
	if actual == "" {
		return fmt.Errorf("worktree_path must be a git worktree on IssueOps branch %s", expected)
	}
	if actual != expected {
		return fmt.Errorf("worktree branch %s does not match IssueOps branch %s", actual, expected)
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

func issueOpsPlanInLinkedWorktree(record IssueOpsRecord) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	worktree := strings.TrimSpace(record.WorktreePath)
	if planPath == "" || worktree == "" {
		return true
	}
	return issueOpsPlanPathInsideWorktree(worktree, planPath)
}

func issueOpsPlanPathInsideWorktree(worktree, planPath string) bool {
	planPath = strings.TrimSpace(planPath)
	if planPath == "" || strings.Contains(planPath, "\x00") {
		return false
	}
	if !filepath.IsAbs(planPath) {
		return true
	}
	if !pathWithin(planPath, worktree) {
		return false
	}
	resolvedPlan, err := filepath.EvalSymlinks(planPath)
	if err != nil {
		return false
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return false
	}
	return pathWithin(resolvedPlan, resolvedWorktree)
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

func ActiveIssueOpsLinkedWorktreeCycleForRepo(repo string) (IssueOpsRecord, bool) {
	repo = cleanAbsPath(repo)
	if repo == "" {
		return IssueOpsRecord{}, false
	}
	entries, err := os.ReadDir(IssueOpsStateRoot())
	if err != nil {
		return IssueOpsRecord{}, false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := ReadIssueOps(IssueOpsStateRoot(), id)
		if err != nil {
			continue
		}
		if cleanAbsPath(record.Repo) != repo {
			continue
		}
		if record.Phase == IssueOpsPhaseDone {
			continue
		}
		if issueOpsPlanBranchMismatchesRecord(record) {
			continue
		}
		if worktree := strings.TrimSpace(record.WorktreePath); worktree == "" || !issueOpsWorktreePathValid(worktree) {
			continue
		}
		return record, true
	}
	return IssueOpsRecord{}, false
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

func validateRemoteArtifactURL(artifactURL, provider, kind string) error {
	if artifactURL == "" {
		return fmt.Errorf("remote artifact url is required")
	}
	if strings.ContainsAny(artifactURL, "\x00\r\n\t ") {
		return fmt.Errorf("remote artifact url must not contain whitespace or control characters")
	}
	parsed, err := url.Parse(artifactURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("remote artifact url must be an http(s) URL")
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.EscapedPath())
	switch provider + ":" + kind {
	case "github:pr":
		parts := splitIssueOpsURLPath(path)
		if host != "github.com" || len(parts) != 4 || parts[2] != "pull" || !isDecimalString(parts[3]) {
			return fmt.Errorf("remote artifact url must be a GitHub pull request URL")
		}
	case "gitlab:mr":
		parts := splitIssueOpsURLPath(path)
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" || !isDecimalString(parts[len(parts)-1]) {
			return fmt.Errorf("remote artifact url must be a GitLab merge request URL")
		}
	default:
		return fmt.Errorf("remote artifact provider/kind combination is unsupported")
	}
	return nil
}

func validateRemoteArtifactMatchesIssue(issueURL, artifactURL, provider, kind string) error {
	issueKey := issueOpsRemoteProjectKey(issueURL, provider, "issue")
	artifactKey := issueOpsRemoteProjectKey(artifactURL, provider, kind)
	if issueKey == "" || artifactKey == "" {
		return nil
	}
	if issueKey != artifactKey {
		return fmt.Errorf("remote artifact url must match linked issue project")
	}
	return nil
}

func validateIssueOpsChildMatchesParent(parentURL, childURL string) error {
	provider := issueOpsProviderFromURL(parentURL)
	if provider == "" {
		return nil
	}
	if issueOpsIssueNumber(childURL) == "" {
		return fmt.Errorf("child issue url must be a numeric %s issue URL", provider)
	}
	parentKey := issueOpsRemoteProjectKey(parentURL, provider, "issue")
	childKey := issueOpsRemoteProjectKey(childURL, provider, "issue")
	if parentKey == "" || childKey == "" {
		return fmt.Errorf("child issue url must be a %s issue URL", provider)
	}
	if parentKey != childKey {
		return fmt.Errorf("child issue url must match linked parent issue project")
	}
	return nil
}

func issueOpsRemoteProjectKey(rawURL, provider, kind string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	parts := splitIssueOpsURLPath(strings.ToLower(parsed.EscapedPath()))
	switch provider + ":" + kind {
	case "github:issue":
		if host != "github.com" || len(parts) != 4 || parts[2] != "issues" {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "github:pr":
		if host != "github.com" || len(parts) != 4 || parts[2] != "pull" {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "gitlab:issue":
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "issues" {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	case "gitlab:mr":
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	default:
		return ""
	}
}

func splitIssueOpsURLPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func cleanIssueOpsRemoteValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func invalidIssueOpsRemoteAssignee(values []string) string {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "@me", "me", "self", "@self", "current_user", "current-user":
			return value
		}
	}
	return ""
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
