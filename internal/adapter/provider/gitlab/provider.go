package gitlab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	"issueops/internal/domain/remoteparse"
	"issueops/internal/port"
)

// Provider adapts GitLab via the `glab` CLI.
type Provider struct{}

var glabCapabilityVersionPattern = regexp.MustCompile(`(?:^|\s)v?(\d+)\.(\d+)\.(\d+)(?:\s|$)`)

func NewProvider() Provider { return Provider{} }

func (Provider) Name() string { return "gitlab" }

func (Provider) CreateIssue(req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return Provider{}.CreateIssueContext(context.Background(), req)
}

func (Provider) CreateIssueContext(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("issue title is required")
	}
	projectURL, err := gitLabProjectURL(req.ProjectKey)
	if err != nil {
		return port.IssueProviderCreateIssueResult{OK: false}, err
	}
	if req.Confirm && projectURL == "" {
		return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("GitLab project authority is required")}
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
	if projectURL != "" {
		args = append(args, "--repo", projectURL)
	}
	if !req.Confirm {
		return port.IssueProviderCreateIssueResult{
			OK:      true,
			Preview: providerutil.DryRunPreview("glab", args...),
		}, nil
	}
	return runGlabJSONContext(ctx, args, req.Repo, "issue")
}

func (Provider) FindIssueCreateCandidates(ctx context.Context, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	const searchLimit = 100
	authority := strings.TrimSpace(req.ProjectAuthority)
	parts := strings.SplitN(authority, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(req.Marker) == "" {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, fmt.Errorf("invalid GitLab issue create reconciliation request")
	}
	endpoint := "projects/" + url.PathEscape(parts[1]) + "/issues?scope=all&per_page=" + strconv.Itoa(searchLimit) + "&in=description&search=" + url.QueryEscape(req.Marker)
	args := []string{"api", endpoint}
	if parts[0] != "gitlab.com" {
		args = append(args, "--hostname", parts[0])
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", args...)
	if err != nil {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, fmt.Errorf("glab issue reconciliation failed: %w", err)
	}
	var rows []struct {
		URL         string `json:"web_url"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, fmt.Errorf("parse glab issue reconciliation response: %w", err)
	}
	candidates := make([]port.IssueProviderIssueCreateCandidate, 0, len(rows))
	for _, row := range rows {
		if strings.Contains(row.Description, req.Marker) {
			candidates = append(candidates, port.IssueProviderIssueCreateCandidate{URL: row.URL, Title: row.Title, Body: row.Description})
		}
	}
	return port.IssueProviderFindIssueCreateCandidatesResult{
		Candidates: candidates,
		Truncated:  len(rows) >= searchLimit,
	}, nil
}

func (Provider) CreatePullRequest(req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return Provider{}.CreatePullRequestContext(context.Background(), req)
}

func (Provider) CreatePullRequestContext(ctx context.Context, req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("MR title is required")
	}
	head := strings.TrimSpace(req.HeadBranch)
	base := strings.TrimSpace(req.BaseBranch)
	if head == "" || base == "" {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("source and target branches are required")
	}
	projectURL, err := gitLabProjectURL(req.ProjectKey)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false}, err
	}
	args := []string{"mr", "create", "--title", title, "--source-branch", head, "--target-branch", base, "--yes"}
	if projectURL != "" {
		args = append(args, "--repo", projectURL)
	}
	if req.Draft {
		args = append(args, "--draft")
	}
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
	if !req.Confirm {
		return port.IssueProviderCreatePullRequestResult{
			OK:      true,
			Preview: providerutil.DryRunPreview("glab", args...),
		}, nil
	}
	if gitLabProjectRequiresGlab182(projectURL) {
		version, versionErr := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", "--version")
		if versionErr != nil || !glabCapabilityAtLeast182(string(version)) {
			return port.IssueProviderCreatePullRequestResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("GitLab custom web authority requires proven glab >= 1.82.0 capability")}
		}
	}
	result, err := runGlabMRJSON(ctx, args, req.Repo)
	if err != nil {
		return result, err
	}
	if !validCanonicalGitLabMergeRequestURL(result.URL) {
		return port.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("created artifact URL unavailable; needs reconciliation; not retried")
	}
	if err := verifyCreatedGitLabMergeRequest(ctx, req, result.URL); err != nil {
		return port.IssueProviderCreatePullRequestResult{OK: false, URL: result.URL}, gitlabCreatedMergeRequestError(result.URL, err)
	}
	return result, nil
}

func validCanonicalGitLabMergeRequestURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != raw {
		return false
	}
	parts := remoteparse.SplitGitLabMRPath(parsed.EscapedPath())
	return parts.Project != "" && parts.IID != ""
}

func gitLabProjectURL(projectKey string) (string, error) {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return "", nil
	}
	if projectKey != strings.ToLower(projectKey) || strings.ContainsAny(projectKey, "\\?#@\x00\r\n\t ") {
		return "", fmt.Errorf("GitLab project authority is malformed")
	}
	projectURL := "https://" + projectKey
	parsed, err := url.Parse(projectURL)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("GitLab project authority is malformed")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || len(parts) < 2 || parsed.Host+"/"+strings.Join(parts, "/") != projectKey {
		return "", fmt.Errorf("GitLab project authority is malformed")
	}
	return projectURL, nil
}

type gitlabMergeRequestProjection struct {
	WebURL          string           `json:"web_url"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	SourceBranch    string           `json:"source_branch"`
	TargetBranch    string           `json:"target_branch"`
	Draft           bool             `json:"draft"`
	State           string           `json:"state"`
	SHA             string           `json:"sha"`
	Labels          []string         `json:"labels"`
	Assignees       []gitlabAssignee `json:"assignees"`
	SourceProjectID int64            `json:"source_project_id"`
	TargetProjectID int64            `json:"target_project_id"`
}

func verifyCreatedGitLabMergeRequest(ctx context.Context, req port.IssueProviderCreatePullRequestRequest, createdURL string) error {
	createdURL = strings.TrimSpace(createdURL)
	parsedURL, err := url.Parse(createdURL)
	if err != nil {
		return fmt.Errorf("created merge request URL is invalid")
	}
	parts := remoteparse.SplitGitLabMRPath(parsedURL.EscapedPath())
	if parts.Project == "" || parts.IID == "" {
		return fmt.Errorf("created merge request URL does not contain an exact project and IID")
	}
	projectURL, err := gitLabProjectURL(req.ProjectKey)
	if err != nil {
		return err
	}
	if projectURL == "" {
		projectURL = parsedURL.Scheme + "://" + parsedURL.Host + "/" + parts.Project
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", "mr", "view", parts.IID, "--repo", projectURL, "--output", "json")
	if err != nil {
		return fmt.Errorf("read back created merge request: %w", err)
	}
	var got gitlabMergeRequestProjection
	if err := json.Unmarshal(out, &got); err != nil {
		return fmt.Errorf("parse created merge request readback: %w", err)
	}
	assignees := gitlabAssigneeUsernames(got.Assignees)
	if strings.TrimSpace(got.WebURL) != createdURL || strings.TrimSpace(got.SourceBranch) != strings.TrimSpace(req.HeadBranch) || strings.TrimSpace(got.TargetBranch) != strings.TrimSpace(req.BaseBranch) || got.Draft != req.Draft {
		return fmt.Errorf("created merge request identity or draft status does not match the request")
	}
	if strings.TrimSpace(got.Title) != strings.TrimSpace(req.Title) || got.Description != req.Body {
		return fmt.Errorf("created merge request title or rendered body does not match the request")
	}
	if strings.TrimSpace(req.ProjectKey) != "" && gitLabSourceProjectKey(req.ProjectKey, got) != strings.TrimSpace(req.ProjectKey) {
		return fmt.Errorf("created merge request source project does not match published project authority")
	}
	if strings.TrimSpace(req.ExpectedHeadSHA) != "" && strings.TrimSpace(got.SHA) != strings.TrimSpace(req.ExpectedHeadSHA) {
		return fmt.Errorf("created merge request head SHA does not match accepted final head")
	}
	if !providerutil.SameStrings(req.Labels, got.Labels) {
		return fmt.Errorf("created merge request labels do not exactly match the request")
	}
	if !providerutil.SameStrings(req.Assignees, assignees) {
		return fmt.Errorf("created merge request assignees do not exactly match the request")
	}
	return nil
}

func (Provider) ReconcilePullRequest(req port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
	return Provider{}.ReconcilePullRequestContext(context.Background(), req)
}

func (Provider) ReconcilePullRequestContext(ctx context.Context, req port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
	projectURL, err := gitLabProjectURL(req.ProjectKey)
	if err != nil || projectURL == "" {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitLab reconcile requires exact project authority")
	}
	if gitLabProjectRequiresGlab182(projectURL) {
		version, versionErr := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", "--version")
		if versionErr != nil || !glabCapabilityAtLeast182(string(version)) {
			return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitLab custom web authority requires proven glab >= 1.82.0 capability")
		}
	}
	args := []string{"mr", "list", "--repo", projectURL, "--source-branch", strings.TrimSpace(req.HeadBranch), "--all", "--per-page", "2", "--output", "json"}
	out, err := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", args...)
	if err != nil {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("list GitLab reconcile candidates: %w", err)
	}
	var rows []gitlabMergeRequestProjection
	if err := json.Unmarshal(out, &rows); err != nil {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("parse GitLab reconcile candidates: %w", err)
	}
	if strings.TrimSpace(string(out)) == "null" {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitLab reconcile candidate response is not an authoritative array")
	}
	if len(rows) > 2 {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("GitLab reconcile candidate response exceeded bound")
	}
	result := port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: len(rows) == 0}
	for _, row := range rows {
		parsed, _ := url.Parse(strings.TrimSpace(row.WebURL))
		projectKey := ""
		if parsed != nil {
			parts := remoteparse.SplitGitLabMRPath(parsed.EscapedPath())
			if parts.Project != "" {
				projectKey = strings.ToLower(parsed.Host + "/" + parts.Project)
			}
		}
		result.Candidates = append(result.Candidates, port.IssueProviderReconcilePullRequestCandidate{
			URL: row.WebURL, ProjectKey: projectKey, SourceProjectKey: gitLabSourceProjectKey(projectKey, row), HeadBranch: row.SourceBranch, BaseBranch: row.TargetBranch,
			HeadSHA: row.SHA, Title: strings.TrimSpace(row.Title), BodySHA256: gitLabBodySHA256(row.Description), Labels: append([]string(nil), row.Labels...), Assignees: gitlabAssigneeUsernames(row.Assignees), Draft: row.Draft, State: strings.TrimSpace(row.State),
		})
	}
	return result, nil
}

func gitLabBodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func gitLabSourceProjectKey(projectKey string, row gitlabMergeRequestProjection) string {
	if row.SourceProjectID <= 0 || row.TargetProjectID <= 0 || row.SourceProjectID != row.TargetProjectID {
		return ""
	}
	return projectKey
}

func gitLabProjectRequiresGlab182(projectURL string) bool {
	parsed, err := url.Parse(projectURL)
	return err == nil && (strings.Contains(parsed.Hostname(), ":") || parsed.Port() != "" && parsed.Port() != "443")
}

func glabCapabilityAtLeast182(value string) bool {
	match := glabCapabilityVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 4 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return major > 1 || major == 1 && minor >= 82
}

func gitlabCreatedMergeRequestError(_ string, err error) error {
	return fmt.Errorf("created merge request has unknown state with a canonical URL returned separately and needs reconciliation; creation was not retried: %s", providerutil.BoundedDiagnostic(err.Error(), 384))
}
