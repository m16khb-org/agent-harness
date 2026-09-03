package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"agent-harness/internal/adapter/provider/providerutil"
	bodysync "agent-harness/internal/domain/issueopsbodysync"
	"agent-harness/internal/domain/remoteparse"
	"agent-harness/internal/port"
)

// glabArtifactEndpoint resolves the REST collection that addresses a sync kind.
// The set is closed: "pr" is GitHub's name for the same artifact.
func glabArtifactEndpoint(kind, artifactURL string) (hostname, endpoint string, err error) {
	switch strings.TrimSpace(kind) {
	case "issue", "child":
		host, project, iid, perr := parseGitLabIssueURL(artifactURL)
		if perr != nil {
			return "", "", fmt.Errorf("artifact url must be a GitLab issue or work item URL")
		}
		return host, "projects/" + url.PathEscape(project) + "/issues/" + iid, nil
	case "mr":
		host, project, iid, perr := parseGitLabMergeRequestURL(artifactURL)
		if perr != nil {
			return "", "", perr
		}
		return host, "projects/" + url.PathEscape(project) + "/merge_requests/" + iid, nil
	}
	return "", "", fmt.Errorf("unsupported gitlab artifact kind %q (want issue|child|mr)", kind)
}

func parseGitLabMergeRequestURL(raw string) (hostname, projectPath, iid string, err error) {
	trimmed := strings.TrimSpace(raw)
	parsed, perr := url.Parse(trimmed)
	if perr != nil {
		return "", "", "", fmt.Errorf("artifact url must be a GitLab merge request URL")
	}
	// A scheme-less URL folds the authority into Path; re-parse so the host is
	// recovered, matching splitGitLabIssueURL.
	if parsed.Hostname() == "" && !strings.Contains(trimmed, "://") {
		if reparsed, rerr := url.Parse("https://" + trimmed); rerr == nil {
			parsed = reparsed
		}
	}
	parts := remoteparse.SplitGitLabMRPath(parsed.EscapedPath())
	if parsed.Hostname() == "" || parts.Project == "" || parts.IID == "" {
		return "", "", "", fmt.Errorf("artifact url must be a GitLab merge request URL")
	}
	return parsed.Hostname(), parts.Project, parts.IID, nil
}

// ReadArtifactBody reads one artifact's live description and lifecycle state.
func (Provider) ReadArtifactBody(ctx context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	hostname, endpoint, err := glabArtifactEndpoint(req.Kind, req.URL)
	if err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	body, state, _, err := readGlabArtifactBody(ctx, req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	return port.IssueProviderArtifactBody{
		Provider: "gitlab", Kind: req.Kind, URL: strings.TrimSpace(req.URL), Body: body, State: state,
	}, nil
}

// ReplaceArtifactBody replaces the whole description and verifies the write by
// reading it back, so a rejected edit cannot report success.
func (Provider) ReplaceArtifactBody(ctx context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	hostname, endpoint, err := glabArtifactEndpoint(req.Kind, req.URL)
	if err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab"}, err
	}
	if strings.TrimSpace(req.Body) == "" {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab"}, fmt.Errorf("replacement body is empty")
	}
	if len(req.Body) > gitLabIssueBodyLimit {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab"},
			fmt.Errorf("replacement body exceeds the GitLab description limit (%d > %d bytes)", len(req.Body), gitLabIssueBodyLimit)
	}
	if !req.Confirm {
		return port.IssueProviderReplaceArtifactBodyResult{
			OK: true, Provider: "gitlab", URL: strings.TrimSpace(req.URL),
			Preview: fmt.Sprintf("[dry-run] would execute: glab api %s --hostname %s --method PUT -f description=<%d bytes>; then re-read to verify",
				endpoint, hostname, len(req.Body)),
		}, nil
	}
	if _, err := runGlabAPIContext(ctx, req.Repo, hostname, endpoint, "--method", "PUT", "-f", "description="+req.Body); err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab"}, err
	}
	written, _, webURL, err := readGlabArtifactBody(ctx, req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab"}, err
	}
	digest := bodysync.SHA256Body(written)
	resolved := providerutil.FirstNonEmpty(webURL, strings.TrimSpace(req.URL))
	if bodysync.NormalizeBody(written) != bodysync.NormalizeBody(req.Body) {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "gitlab", URL: resolved, VerifiedBodySHA256: digest},
			fmt.Errorf("gitlab body replacement was not verified: the readback differs from what was written")
	}
	return port.IssueProviderReplaceArtifactBodyResult{
		OK: true, Provider: "gitlab", URL: resolved, Updated: true, VerifiedBodySHA256: digest,
	}, nil
}

// VerifyChildHierarchy asks GitLab whether the child work item hangs off the
// parent issue, reusing the observation CreateChild verifies against.
func (Provider) VerifyChildHierarchy(_ context.Context, req port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	hostname, projectPath, parentIID, err := parseGitLabIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderChildHierarchyResult{Provider: "gitlab"}, err
	}
	_, _, childIID, err := parseGitLabIssueURL(req.ChildURL)
	if err != nil {
		return port.IssueProviderChildHierarchyResult{Provider: "gitlab"}, fmt.Errorf("child_url must be a GitLab issue or work item URL")
	}
	_, ok := observeGitLabChild(req.Repo, hostname, projectPath, parentIID, childIID)
	return port.IssueProviderChildHierarchyResult{Provider: "gitlab", OK: true, Verified: ok}, nil
}

func readGlabArtifactBody(ctx context.Context, repo, hostname, endpoint string) (body, state, webURL string, err error) {
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint)
	if err != nil {
		return "", "", "", err
	}
	var payload struct {
		Description string `json:"description"`
		State       string `json:"state"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", "", "", fmt.Errorf("parse glab artifact description: %w", err)
	}
	return payload.Description, payload.State, payload.WebURL, nil
}
