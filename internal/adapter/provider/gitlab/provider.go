package gitlab

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"agent-harness/internal/port"
)

// Provider adapts GitLab via the `glab` CLI.
type Provider struct{}

func NewProvider() Provider { return Provider{} }

func (Provider) Name() string { return "gitlab" }

func (Provider) CreateIssue(req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("issue title is required")
	}
	args := []string{"issue", "create", "--title", title}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		args = append(args, "--description", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "glab " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreateIssueResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	return runGlabJSON(args, req.Repo, "issue")
}

func (Provider) CreatePullRequest(req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("MR title is required")
	}
	head := strings.TrimSpace(req.HeadBranch)
	base := strings.TrimSpace(req.BaseBranch)
	if head == "" || base == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("source and target branches are required")
	}
	args := []string{"mr", "create", "--title", title, "--source-branch", head, "--target-branch", base}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		args = append(args, "--description", body)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}
	for _, assignee := range req.Assignees {
		args = append(args, "--assignee", assignee)
	}
	cmdStr := "glab " + strings.Join(args, " ")
	if !req.Confirm {
		return port.IssueProviderCreatePullRequestResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: %s", cmdStr),
		}, nil
	}
	return runGlabMRJSON(args, req.Repo)
}

type glabResult struct {
	WebURL string `json:"web_url"`
	IID    int    `json:"iid"`
}

func runGlabJSON(args []string, repo string, kind string) (port.IssueProviderCreateIssueResult, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return port.IssueProviderCreateIssueResult{OK: false},
			fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
	}
	cmd := exec.Command("glab", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return port.IssueProviderCreateIssueResult{OK: false},
			fmt.Errorf("glab %s create failed: %s", kind, stderr)
	}
	url := findGlabURL(string(out), kind)
	return port.IssueProviderCreateIssueResult{OK: true, URL: url}, nil
}

func runGlabMRJSON(args []string, repo string) (port.IssueProviderCreatePullRequestResult, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false},
			fmt.Errorf("glab CLI is not installed")
	}
	cmd := exec.Command("glab", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return port.IssueProviderCreatePullRequestResult{OK: false},
			fmt.Errorf("glab mr create failed: %s", stderr)
	}
	url := findGlabURL(string(out), "mr")
	return port.IssueProviderCreatePullRequestResult{OK: true, URL: url}, nil
}

func findGlabURL(out string, kind string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	// Try JSON first.
	var result glabResult
	if err := json.Unmarshal([]byte(out), &result); err == nil && result.WebURL != "" {
		return result.WebURL
	}
	// Fall back: scan lines for an https URL.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}
