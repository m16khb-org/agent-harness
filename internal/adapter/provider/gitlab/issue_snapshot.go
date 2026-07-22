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
	hostname, projectPath, iid, err := parseGitLabIssueURL(req.URL)
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
	if strings.TrimSpace(payload.WebURL) != issueURL {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("glab issue snapshot URL does not match the linked issue")
	}
	return port.ExecutionIssueSnapshot{URL: issueURL, Body: payload.Description}, nil
}

var _ port.ExecutionIssueSnapshotReader = Provider{}
