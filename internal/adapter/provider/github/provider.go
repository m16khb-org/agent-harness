package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/adapter/provider/issuebody"
	"agent-harness/internal/adapter/provider/providerutil"
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
		OK:  true,
		URL: result.URL,
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
		OK:  true,
		URL: result.URL,
	}, nil
}

func (Provider) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, fmt.Errorf("child title is required")
	}
	owner, repoName, parentNumber, err := parseGitHubIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, err
	}
	createArgs := []string{"issue", "create", "--title", title}
	body := strings.TrimSpace(req.Body)
	if body != "" {
		createArgs = append(createArgs, "--body", body)
	}
	for _, label := range req.Labels {
		createArgs = append(createArgs, "--label", label)
	}
	for _, assignee := range req.Assignees {
		createArgs = append(createArgs, "--assignee", assignee)
	}
	createArgs = append(createArgs, "--repo", owner+"/"+repoName)
	if !req.Confirm {
		preferred := append(append([]string{}, createArgs...), "--parent", parentNumber)
		preview := fmt.Sprintf("[dry-run] would execute: gh %s; verify with gh api repos/%s/%s/issues/%s/sub_issues; fallback if --parent is unsupported: gh %s; gh api repos/%s/%s/issues/{child_number}; gh api -X POST repos/%s/%s/issues/%s/sub_issues -f sub_issue_id={child_database_id}; gh api repos/%s/%s/issues/%s/sub_issues",
			strings.Join(preferred, " "), owner, repoName, parentNumber, strings.Join(createArgs, " "), owner, repoName, owner, repoName, parentNumber, owner, repoName, parentNumber)
		return port.IssueProviderCreateChildResult{OK: true, Provider: "github", Preview: preview}, nil
	}
	preferredCreateArgs := append(append([]string{}, createArgs...), "--parent", parentNumber)
	child, err := runGhJSON(preferredCreateArgs, req.Repo, "issue")
	usedParentFlag := err == nil
	if err != nil {
		child, err = runGhJSON(createArgs, req.Repo, "issue")
	}
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, err
	}
	childURL := strings.TrimSpace(child.URL)
	_, _, childNumber, err := parseGitHubIssueURL(childURL)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, fmt.Errorf("parse created child issue URL: %w", err))
	}
	childIssue, err := runGhAPIJSON[githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + childNumber}, "issue lookup")
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, err)
	}
	if childIssue.ID == 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, fmt.Errorf("created child issue is missing database id"))
	}
	if !usedParentFlag {
		_, err = runGhAPIJSON[map[string]any](req.Repo, []string{"-X", "POST", "repos/" + owner + "/" + repoName + "/issues/" + parentNumber + "/sub_issues", "-f", "sub_issue_id=" + strconv.FormatInt(childIssue.ID, 10)}, "sub-issue attach")
		if err != nil {
			return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, err)
		}
	}
	children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + parentNumber + "/sub_issues"}, "sub-issue verification")
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, err)
	}
	if !githubIssueListContains(children, childIssue.ID, childNumber) {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, fmt.Errorf("github sub-issue hierarchy verification failed"))
	}
	labels := githubLabelNames(childIssue.Labels)
	assignees := githubAssigneeLogins(childIssue.Assignees)
	if missing := providerutil.MissingStrings(req.Labels, labels); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, fmt.Errorf("github child issue missing labels: %s", strings.Join(missing, ", ")))
	}
	if missing := providerutil.MissingStrings(req.Assignees, assignees); len(missing) > 0 {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, githubCreatedChildError(childURL, fmt.Errorf("github child issue missing assignees: %s", strings.Join(missing, ", ")))
	}
	return port.IssueProviderCreateChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          providerutil.FirstNonEmpty(childIssue.HTMLURL, childURL),
		ChildNumber:       childNumber,
		HierarchyVerified: true,
		Labels:            labels,
		Assignees:         assignees,
	}, nil
}

func (Provider) CloseChild(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	owner, repoName, parentNumber, err := parseGitHubIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	childOwner, childRepo, childNumber, err := parseGitHubIssueURL(req.ChildURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	if childOwner != owner || childRepo != repoName {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("child issue url must match linked parent issue project")
	}
	if !req.Confirm {
		preview := fmt.Sprintf("[dry-run] would execute: gh api repos/%s/%s/issues/%s/sub_issues; gh api -X PATCH repos/%s/%s/issues/%s -f state=closed -f state_reason=completed; gh api repos/%s/%s/issues/%s",
			owner, repoName, parentNumber, owner, repoName, childNumber, owner, repoName, childNumber)
		return port.IssueProviderCloseChildResult{OK: true, Provider: "github", ChildURL: strings.TrimSpace(req.ChildURL), Preview: preview}, nil
	}
	children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + parentNumber + "/sub_issues"}, "sub-issue verification")
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	childIssue := githubIssueByNumber(children, childNumber)
	if childIssue.Number == 0 {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("github sub-issue hierarchy verification failed")
	}
	alreadyClosed := strings.EqualFold(childIssue.State, "closed")
	if !alreadyClosed {
		if _, err := runGhAPIJSON[githubIssue](req.Repo, []string{"-X", "PATCH", "repos/" + owner + "/" + repoName + "/issues/" + childNumber, "-f", "state=closed", "-f", "state_reason=completed"}, "issue close"); err != nil {
			return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
		}
	}
	verified, err := runGhAPIJSON[githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + childNumber}, "issue close verification")
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	if !strings.EqualFold(verified.State, "closed") {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("github child issue close verification failed: state=%s", verified.State)
	}
	return port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          providerutil.FirstNonEmpty(verified.HTMLURL, childIssue.HTMLURL, req.ChildURL),
		HierarchyVerified: true,
		Closed:            true,
		AlreadyClosed:     alreadyClosed,
		State:             verified.State,
	}, nil
}

type ghResult struct {
	URL string `json:"url"`
}

type githubIssue struct {
	ID        int64            `json:"id"`
	Number    int              `json:"number"`
	HTMLURL   string           `json:"html_url"`
	State     string           `json:"state"`
	Labels    []githubLabel    `json:"labels"`
	Assignees []githubAssignee `json:"assignees"`
}

type githubLabel struct {
	Name string `json:"name"`
}

type githubAssignee struct {
	Login string `json:"login"`
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
	return parseGhOutput(string(out)), nil
}

// parseGhOutput extracts the created artifact's URL from gh output. No create
// call passes --json, so gh issue/pr create always emits the artifact URL as a
// bare line; the output is treated verbatim as that URL.
func parseGhOutput(out string) ghResult {
	return ghResult{URL: strings.TrimSpace(out)}
}

func runGhAPIJSON[T any](repo string, args []string, kind string) (T, error) {
	var zero T
	if _, err := exec.LookPath("gh"); err != nil {
		return zero, fmt.Errorf("gh CLI is not installed; install it from https://cli.github.com")
	}
	cmdArgs := append([]string{"api"}, args...)
	cmd := exec.Command("gh", cmdArgs...)
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return zero, fmt.Errorf("gh api %s failed: %s", kind, stderr)
	}
	if err := json.Unmarshal(out, &zero); err != nil {
		return zero, fmt.Errorf("parse gh api %s response: %w", kind, err)
	}
	return zero, nil
}

func githubCreatedChildError(childURL string, err error) error {
	childURL = strings.TrimSpace(childURL)
	if childURL == "" {
		return err
	}
	return fmt.Errorf("created child %s but follow-up failed: %w", childURL, err)
}

func parseGitHubIssueURL(raw string) (owner, repo, number string, err error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "/")
	for i := 0; i+4 < len(parts); i++ {
		if parts[i] == "github.com" && parts[i+3] == "issues" {
			if parts[i+1] == "" || parts[i+2] == "" || parts[i+4] == "" {
				break
			}
			return parts[i+1], parts[i+2], parts[i+4], nil
		}
	}
	return "", "", "", fmt.Errorf("parent_issue_url must be a GitHub issue URL")
}

func githubIssueListContains(issues []githubIssue, id int64, number string) bool {
	for _, issue := range issues {
		if id != 0 && issue.ID == id {
			return true
		}
		if number != "" && strconv.Itoa(issue.Number) == number {
			return true
		}
	}
	return false
}

func githubIssueByNumber(issues []githubIssue, number string) githubIssue {
	for _, issue := range issues {
		if number != "" && strconv.Itoa(issue.Number) == number {
			return issue
		}
	}
	return githubIssue{}
}

func githubLabelNames(labels []githubLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.TrimSpace(label.Name) != "" {
			out = append(out, label.Name)
		}
	}
	return out
}

func githubAssigneeLogins(assignees []githubAssignee) []string {
	out := make([]string, 0, len(assignees))
	for _, assignee := range assignees {
		if strings.TrimSpace(assignee.Login) != "" {
			out = append(out, assignee.Login)
		}
	}
	return out
}

func (Provider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	issueURL := strings.TrimSpace(req.IssueURL)
	if issueURL == "" {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, fmt.Errorf("issue url is required")
	}
	section := issuebody.RenderDevilsAdvocateSection(req.Findings, time.Now().UTC().Format(time.RFC3339))
	if !req.Confirm {
		return port.IssueProviderUpdateIssueBodySectionResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: gh issue view %s --json body; gh issue edit %s --body <merged devil's-advocate section>", issueURL, issueURL),
		}, nil
	}
	body, err := readGhIssueBody(req.Repo, issueURL)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	if err := runGhIssueEdit(req.Repo, issueURL, issuebody.MergeIssueBodySection(body, section)); err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: issueURL, Updated: true}, nil
}

func readGhIssueBody(repo, issueURL string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI is not installed; install it from https://cli.github.com")
	}
	cmd := exec.Command("gh", "issue", "view", issueURL, "--json", "body")
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh issue view failed: %s", ghExecStderr(err))
	}
	var payload struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gh issue body: %w", err)
	}
	return payload.Body, nil
}

func runGhIssueEdit(repo, issueURL, body string) error {
	cmd := exec.Command("gh", "issue", "edit", issueURL, "--body", body)
	if repo != "" {
		cmd.Dir = repo
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh issue edit failed: %s", ghExecStderr(err))
	}
	return nil
}

func ghExecStderr(err error) string {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return stderr
		}
	}
	return err.Error()
}
