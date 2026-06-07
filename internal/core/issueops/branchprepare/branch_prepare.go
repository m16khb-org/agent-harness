package branchprepare

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
)

type Store struct {
	Read             func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite       func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	ValidateIssueURL func(issueURL string) error
}

func Prepare(store Store, stateRoot, id string, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsRecord, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	issueURL := strings.TrimSpace(req.IssueURL)
	if issueURL == "" {
		record, err := store.Read(stateRoot, id)
		if err != nil {
			return record, err
		}
		issueURL = strings.TrimSpace(record.IssueURL)
	}
	if err := store.ValidateIssueURL(issueURL); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if provider == "" {
		provider = remote.ProviderFromURL(issueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("provider must be github or gitlab")
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("branch is required")
	}
	if err := ValidateBranch(branch); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	baseBranch := strings.TrimSpace(req.BaseBranch)
	if baseBranch == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_branch is required")
	}
	if issueNumber := remote.IssueNumber(issueURL); issueNumber != "" {
		if !strings.HasPrefix(branch, issueNumber+"-") {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("issueops branch for issue %s must start with %s-; for example %s-fix-login", issueNumber, issueNumber, issueNumber)
		}
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issue must be linked before branch prepare")
	}
	if strings.TrimSpace(record.Branch) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issueops record must be started with branch before branch prepare")
	}
	if record.Branch != branch {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("branch does not match IssueOps record branch")
	}
	if record.IssueURL != issueURL {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("issue_url does not match linked IssueOps issue")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.BranchPrepare = &model.IssueOpsBranchPrepare{
		Provider:        provider,
		IssueURL:        issueURL,
		Branch:          branch,
		BaseBranch:      baseBranch,
		BaseSHA:         strings.TrimSpace(req.BaseSHA),
		RemoteBranchURL: strings.TrimSpace(req.RemoteBranchURL),
		LinkVerified:    req.LinkVerified,
		Steps:           Steps(provider, issueURL, branch, baseBranch),
		CreatedAt:       now,
	}
	return store.TouchWrite(stateRoot, record)
}

func ValidateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	if strings.ContainsAny(branch, " \t\r\n") || strings.HasPrefix(branch, "/") || strings.Contains(branch, "..") {
		return fmt.Errorf("issueops branch contains invalid characters: %s", branch)
	}
	issueNumber, slug, ok := strings.Cut(branch, "-")
	if !ok || strings.TrimSpace(slug) == "" || !isDecimalString(issueNumber) {
		return fmt.Errorf("issueops branch must start with the issue number followed by a hyphen; use names like 2387-fix-grpc-ai-dmm-tag-replication-lag or 2388-fanza-delete-404-stale-registered")
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

func Steps(provider, issueURL, branch, baseBranch string) []model.IssueOpsBranchPrepareStep {
	switch provider {
	case "gitlab":
		return []model.IssueOpsBranchPrepareStep{
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
		return []model.IssueOpsBranchPrepareStep{
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
