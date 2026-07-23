package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"agent-harness/internal/adapter/provider/providerutil"
	"agent-harness/internal/port"
)

func (Provider) ReadIssueSnapshot(ctx context.Context, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return port.ExecutionIssueSnapshot{}, err
	}
	hostname, projectPath, iid, _, err := splitGitLabIssueURL(req.URL)
	if err != nil {
		return port.ExecutionIssueSnapshot{}, err
	}
	endpoint := "projects/" + url.PathEscape(projectPath) + "/issues/" + iid
	args := []string{"api", endpoint}
	if hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	out, err := providerutil.RunBoundedReadback(req.Repo, "glab", args...)
	if err != nil {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("glab issue snapshot read failed: %w", err)
	}
	var payload struct {
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("parse glab issue snapshot: %w", err)
	}
	issueURL := strings.TrimSpace(req.URL)
	if !sameGitLabIssueSnapshotIdentity(issueURL, payload.WebURL) {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("glab issue snapshot URL does not match the linked issue")
	}
	return port.ExecutionIssueSnapshot{URL: issueURL, Body: payload.Description}, nil
}

func sameGitLabIssueSnapshotIdentity(left, right string) bool {
	leftHost, leftProject, leftIID, _, leftErr := splitGitLabIssueURL(left)
	rightHost, rightProject, rightIID, _, rightErr := splitGitLabIssueURL(right)
	return leftErr == nil && rightErr == nil &&
		strings.EqualFold(leftHost, rightHost) &&
		strings.EqualFold(gitLabSnapshotAuthority(left), gitLabSnapshotAuthority(right)) &&
		leftProject == rightProject &&
		leftIID == rightIID
}

func gitLabSnapshotAuthority(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Host == "" && !strings.Contains(trimmed, "://") {
		parsed, err = url.Parse("https://" + trimmed)
		if err != nil {
			return ""
		}
	}
	return parsed.Host
}

var _ port.ExecutionIssueSnapshotReader = Provider{}
