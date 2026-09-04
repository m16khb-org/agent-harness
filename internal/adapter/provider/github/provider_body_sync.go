package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

// ghArtifactCommand maps a sync kind onto the gh noun that addresses it. The
// set is closed: "mr" is GitLab's name for the same artifact and must not reach
// this adapter.
func ghArtifactCommand(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case "issue", "child":
		return "issue", nil
	case "pr":
		return "pr", nil
	}
	return "", fmt.Errorf("unsupported github artifact kind %q (want issue|child|pr)", kind)
}

// ReadArtifactBody reads one artifact's live body and lifecycle state.
func (Provider) ReadArtifactBody(ctx context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	noun, err := ghArtifactCommand(req.Kind)
	if err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return port.IssueProviderArtifactBody{}, fmt.Errorf("artifact url is required")
	}
	body, state, err := readGhArtifactBody(ctx, req.Repo, noun, url)
	if err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	return port.IssueProviderArtifactBody{
		Provider: "github", Kind: req.Kind, URL: url, Body: body, State: state,
	}, nil
}

// ReplaceArtifactBody replaces the whole body and verifies the write by reading
// the body back. The returned digest is hashed from that fresh read, so a write
// the provider silently dropped cannot report success.
func (Provider) ReplaceArtifactBody(ctx context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	noun, err := ghArtifactCommand(req.Kind)
	if err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github"}, err
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github"}, fmt.Errorf("artifact url is required")
	}
	if strings.TrimSpace(req.Body) == "" {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github"}, fmt.Errorf("replacement body is empty")
	}
	if !req.Confirm {
		return port.IssueProviderReplaceArtifactBodyResult{
			OK: true, Provider: "github", URL: url,
			Preview: providerutil.DryRunPreview("gh", noun, "edit", url, "--body-file", "-") +
				"; " + providerutil.DryRunPreview("gh", noun, "view", url, "--json", "body,state"),
		}, nil
	}
	if err := runGhArtifactBodyEdit(ctx, req.Repo, noun, url, req.Body); err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github"}, err
	}
	written, _, err := readGhArtifactBody(ctx, req.Repo, noun, url)
	if err != nil {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github"}, err
	}
	digest := bodysync.SHA256Body(written)
	if bodysync.NormalizeBody(written) != bodysync.NormalizeBody(req.Body) {
		return port.IssueProviderReplaceArtifactBodyResult{Provider: "github", URL: url, VerifiedBodySHA256: digest},
			fmt.Errorf("github body replacement was not verified: the readback differs from what was written")
	}
	return port.IssueProviderReplaceArtifactBodyResult{
		OK: true, Provider: "github", URL: url, Updated: true, VerifiedBodySHA256: digest,
	}, nil
}

// VerifyChildHierarchy asks GitHub whether the child issue is attached to the
// parent as a sub-issue, using the same listing CreateChild verifies against.
func (Provider) VerifyChildHierarchy(_ context.Context, req port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	owner, repoName, parentNumber, err := parseGitHubIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderChildHierarchyResult{Provider: "github"}, err
	}
	_, _, childNumber, err := parseGitHubIssueURL(req.ChildURL)
	if err != nil {
		return port.IssueProviderChildHierarchyResult{Provider: "github"}, fmt.Errorf("child_url must be a GitHub issue URL")
	}
	children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + parentNumber + "/sub_issues"}, "sub-issue verification")
	if err != nil {
		return port.IssueProviderChildHierarchyResult{Provider: "github"}, err
	}
	return port.IssueProviderChildHierarchyResult{
		Provider: "github", OK: true, Verified: githubIssueListContains(children, 0, childNumber),
	}, nil
}

func readGhArtifactBody(ctx context.Context, repo, noun, url string) (body, state string, err error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", "", fmt.Errorf("gh CLI is not installed; install it from https://cli.github.com")
	}
	cmd := exec.CommandContext(ctx, "gh", noun, "view", url, "--json", "body,state")
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("gh %s view failed: %s", noun, ghExecStderr(err))
	}
	var payload struct {
		Body  string `json:"body"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", "", fmt.Errorf("parse gh %s body: %w", noun, err)
	}
	return payload.Body, payload.State, nil
}

// runGhArtifactBodyEdit sends the body on stdin rather than as an argument: a
// full replacement body is unbounded in a way a managed section is not, and an
// argv-sized body would fail at the OS boundary instead of at ours.
func runGhArtifactBodyEdit(ctx context.Context, repo, noun, url, body string) error {
	cmd := exec.CommandContext(ctx, "gh", noun, "edit", url, "--body-file", "-")
	cmd.Stdin = strings.NewReader(body)
	if repo != "" {
		cmd.Dir = repo
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh %s edit failed: %s", noun, ghExecStderr(err))
	}
	return nil
}
