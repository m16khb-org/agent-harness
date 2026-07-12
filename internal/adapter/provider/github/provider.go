package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
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
	if !req.Confirm {
		return port.IssueProviderCreateIssueResult{
			OK:      true,
			Preview: providerutil.DryRunPreview("gh", args...),
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
	projectSelector, err := githubProjectSelector(req.ProjectKey)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false}, err
	}
	args := []string{"pr", "create", "--title", title, "--head", head, "--base", base}
	if projectSelector != "" {
		args = append(args, "--repo", projectSelector)
	}
	if req.Draft {
		args = append(args, "--draft")
	}
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
	if !req.Confirm {
		return port.IssueProviderCreatePullRequestResult{
			OK:      true,
			Preview: providerutil.DryRunPreview("gh", args...),
		}, nil
	}
	result, err := runGhPRCreate(args, req.Repo)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false, URL: result.URL}, err
	}
	resultProject := githubPullRequestProjectKey(result.URL)
	if !validCanonicalGitHubPullRequestURL(result.URL) || projectSelector != "" && resultProject != projectSelector || projectSelector == "" && !strings.HasPrefix(resultProject, "github.com/") {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("created artifact URL unavailable; needs reconciliation; not retried")
	}
	if err := verifyCreatedGitHubPullRequest(req, result.URL); err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false, URL: result.URL}, githubCreatedPullRequestError(result.URL, err)
	}
	return port.IssueProviderCreatePullRequestResult{
		OK:  true,
		URL: result.URL,
	}, nil
}

func runGhPRCreate(args []string, repo string) (ghResult, error) {
	out, invoked, err := providerutil.RunBoundedMutation(repo, "gh", args...)
	result := parseGhOutput(string(out))
	if err == nil {
		return result, nil
	}
	if !invoked {
		return ghResult{}, &port.IssueProviderCreateError{Invoked: false, Err: err}
	}
	if validCanonicalGitHubPullRequestURL(result.URL) {
		return result, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("gh PR creation outcome unknown with a canonical URL returned separately; do not retry: %s", providerutil.BoundedDiagnostic(err.Error(), 384))}
	}
	return ghResult{}, &port.IssueProviderCreateError{Invoked: true, Err: fmt.Errorf("gh PR creation outcome unknown; do not retry: %s", providerutil.BoundedDiagnostic(err.Error(), 384))}
}

func validCanonicalGitHubPullRequestURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != raw || strings.Contains(parsed.EscapedPath(), "%") {
		return false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] != "pull" {
		return false
	}
	number, err := strconv.Atoi(parts[3])
	return err == nil && number > 0
}

func githubProjectSelector(projectKey string) (string, error) {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return "", nil
	}
	if projectKey != strings.ToLower(projectKey) || strings.ContainsAny(projectKey, "\\?#@\x00\r\n\t ") {
		return "", fmt.Errorf("GitHub project authority is malformed")
	}
	parsed, err := url.Parse("https://" + projectKey)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("GitHub project authority is malformed")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" || parsed.Host+"/"+strings.Join(parts, "/") != projectKey {
		return "", fmt.Errorf("GitHub project authority is malformed")
	}
	return projectKey, nil
}

func githubPullRequestProjectKey(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 {
		return ""
	}
	return strings.ToLower(parsed.Host + "/" + strings.Join(parts[:2], "/"))
}

type githubPullRequestProjection struct {
	URL            string           `json:"url"`
	Title          string           `json:"title"`
	Body           string           `json:"body"`
	HeadRefName    string           `json:"headRefName"`
	BaseRefName    string           `json:"baseRefName"`
	IsDraft        bool             `json:"isDraft"`
	HeadRefOID     string           `json:"headRefOid"`
	Labels         []githubLabel    `json:"labels"`
	Assignees      []githubAssignee `json:"assignees"`
	HeadRepository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"headRepository"`
}

func githubSourceProjectKey(projectKey string, row githubPullRequestProjection) string {
	selector, err := githubProjectSelector(projectKey)
	if err != nil || selector == "" || strings.TrimSpace(row.HeadRepository.NameWithOwner) == "" {
		return ""
	}
	host := strings.SplitN(selector, "/", 2)[0]
	return strings.ToLower(host + "/" + strings.Trim(strings.TrimSpace(row.HeadRepository.NameWithOwner), "/"))
}

func verifyCreatedGitHubPullRequest(req port.IssueProviderCreatePullRequestRequest, createdURL string) error {
	createdURL = strings.TrimSpace(createdURL)
	args := []string{"pr", "view", createdURL}
	if strings.TrimSpace(req.ProjectKey) != "" {
		args = append(args, "--repo", strings.TrimSpace(req.ProjectKey))
	}
	args = append(args, "--json", "url,title,body,headRefName,baseRefName,isDraft,headRefOid,labels,assignees,headRepository")
	out, err := providerutil.RunBoundedReadback(req.Repo, "gh", args...)
	if err != nil {
		return fmt.Errorf("read back created pull request: %w", err)
	}
	var got githubPullRequestProjection
	if err := json.Unmarshal(out, &got); err != nil {
		return fmt.Errorf("parse created pull request readback: %w", err)
	}
	labels := make([]string, 0, len(got.Labels))
	for _, label := range got.Labels {
		labels = append(labels, label.Name)
	}
	assignees := githubAssigneeLogins(got.Assignees)
	if strings.TrimSpace(got.URL) != createdURL || strings.TrimSpace(got.HeadRefName) != strings.TrimSpace(req.HeadBranch) || strings.TrimSpace(got.BaseRefName) != strings.TrimSpace(req.BaseBranch) || got.IsDraft != req.Draft {
		return fmt.Errorf("created pull request identity or draft status does not match the request")
	}
	if strings.TrimSpace(got.Title) != strings.TrimSpace(req.Title) || got.Body != req.Body {
		return fmt.Errorf("created pull request title or rendered body does not match the request")
	}
	if strings.TrimSpace(req.ProjectKey) != "" && githubSourceProjectKey(req.ProjectKey, got) != strings.TrimSpace(req.ProjectKey) {
		return fmt.Errorf("created pull request source project does not match published project authority")
	}
	if strings.TrimSpace(req.ExpectedHeadSHA) != "" && strings.TrimSpace(got.HeadRefOID) != strings.TrimSpace(req.ExpectedHeadSHA) {
		return fmt.Errorf("created pull request head SHA does not match accepted final head")
	}
	if !providerutil.SameStrings(req.Labels, labels) {
		return fmt.Errorf("created pull request labels do not exactly match the request")
	}
	wantedAssignees, err := githubReadbackAssignees(req.Repo, req.Assignees)
	if err != nil {
		return err
	}
	if !providerutil.SameStrings(wantedAssignees, assignees) {
		return fmt.Errorf("created pull request assignees do not exactly match the request")
	}
	return nil
}

func githubReadbackAssignees(repo string, requested []string) ([]string, error) {
	resolved := make([]string, 0, len(requested))
	for _, value := range requested {
		if strings.TrimSpace(value) != "@me" {
			resolved = append(resolved, value)
			continue
		}
		out, err := providerutil.RunBoundedReadback(repo, "gh", "api", "user", "--jq", ".login")
		if err != nil || strings.TrimSpace(string(out)) == "" {
			return nil, fmt.Errorf("resolve exact GitHub @me assignee for readback")
		}
		resolved = append(resolved, strings.TrimSpace(string(out)))
	}
	return resolved, nil
}

func (Provider) ReconcilePullRequest(req port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
	selector, err := githubProjectSelector(req.ProjectKey)
	if err != nil || selector == "" {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitHub reconcile requires exact project authority")
	}
	args := []string{"pr", "list", "--repo", selector, "--head", strings.TrimSpace(req.HeadBranch), "--state", "all", "--limit", "2", "--json", "url,title,body,headRefName,baseRefName,isDraft,headRefOid,labels,assignees,headRepository"}
	out, err := providerutil.RunBoundedReadback(req.Repo, "gh", args...)
	if err != nil {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("list GitHub reconcile candidates: %w", err)
	}
	var rows []githubPullRequestProjection
	if err := json.Unmarshal(out, &rows); err != nil {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("parse GitHub reconcile candidates: %w", err)
	}
	if strings.TrimSpace(string(out)) == "null" {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitHub reconcile candidate response is not an authoritative array")
	}
	if len(rows) > 2 {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitHub reconcile candidate response exceeded bound")
	}
	result := port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: len(rows) == 0}
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			labels = append(labels, label.Name)
		}
		result.Candidates = append(result.Candidates, port.IssueProviderReconcilePullRequestCandidate{
			URL: row.URL, ProjectKey: githubPullRequestProjectKey(row.URL), SourceProjectKey: githubSourceProjectKey(selector, row), HeadBranch: row.HeadRefName, BaseBranch: row.BaseRefName,
			HeadSHA: row.HeadRefOID, Title: strings.TrimSpace(row.Title), BodySHA256: providerBodySHA256(row.Body), Labels: labels, Assignees: githubAssigneeLogins(row.Assignees), Draft: row.IsDraft,
		})
	}
	return result, nil
}

func providerBodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func githubCreatedPullRequestError(_ string, err error) error {
	return fmt.Errorf("created pull request has unknown state with a canonical URL returned separately and needs reconciliation; creation was not retried: %s", providerutil.BoundedDiagnostic(err.Error(), 384))
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
		preview := strings.Join([]string{
			providerutil.DryRunPreview("gh", preferred...),
			providerutil.DryRunPreview("gh", "api", "repos/"+owner+"/"+repoName+"/issues/"+parentNumber+"/sub_issues"),
			providerutil.DryRunPreview("gh", createArgs...),
			providerutil.DryRunPreview("gh", "api", "-X", "POST", "repos/"+owner+"/"+repoName+"/issues/"+parentNumber+"/sub_issues", "-f", "sub_issue_id={child_database_id}"),
		}, "; ") + "; fallback if --parent is unsupported"
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
