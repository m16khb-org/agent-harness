package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"agent-harness/internal/port"
)

// Provider adapts GitHub via the `gh` CLI.
type Provider struct{}

func NewProvider() Provider { return Provider{} }

func (Provider) Name() string { return "github" }

func (Provider) CreateIssue(req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("issue title is required")
	}
	body := strings.TrimSpace(req.Body)
	args := []string{"issue", "create", "--title", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "gh " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreateIssueResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	result, err := runGhJSON(args, req.Repo, "issue")
	if err != nil {
		return port.IssueProviderCreateIssueResult{OK: false}, err
	}
	return port.IssueProviderCreateIssueResult{
		OK:     true,
		URL:    result.URL,
		Number: result.Number,
	}, nil
}

func (Provider) CreatePullRequest(req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("PR title is required")
	}
	head := strings.TrimSpace(req.HeadBranch)
	base := strings.TrimSpace(req.BaseBranch)
	if head == "" || base == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("head and base branches are required")
	}
	args := []string{"pr", "create", "--title", title, "--head", head, "--base", base}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		args = append(args, "--body", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "gh " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreatePullRequestResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	result, err := runGhJSON(args, req.Repo, "pr")
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false}, err
	}
	return port.IssueProviderCreatePullRequestResult{
		OK:     true,
		URL:    result.URL,
		Number: result.Number,
	}, nil
}

type ghResult struct {
	URL    string `json:"url"`
	Number string `json:"number"`
}

func runGhJSON(args []string, repo string, kind string) (ghResult, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return ghResult{}, fmt.Errorf("gh CLI is not installed; install it from https://cli.github.com")
	}
	cmd := exec.Command("gh", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return ghResult{}, fmt.Errorf("gh %s create failed: %s", kind, stderr)
	}
	return parseGhOutput(string(out), kind)
}

func parseGhOutput(out string, kind string) (ghResult, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return ghResult{}, nil
	}
	// gh can output either a URL string or JSON. Try JSON first.
	var result ghResult
	if err := json.Unmarshal([]byte(out), &result); err == nil && (result.URL != "" || result.Number != "") {
		return result, nil
	}
	// Fall back to treating output as a URL.
	return ghResult{URL: out}, nil
}
