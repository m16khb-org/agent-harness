package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/internal/adapter/provider/providerutil"
	"agent-harness/internal/port"
)

func (Provider) ReadIssueSnapshot(ctx context.Context, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return port.ExecutionIssueSnapshot{}, err
	}
	issueURL := strings.TrimSpace(req.URL)
	if issueURL == "" {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("issue URL is required")
	}
	out, err := providerutil.RunBoundedReadback(req.Repo, "gh", "issue", "view", issueURL, "--json", "url,body")
	if err != nil {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("gh issue snapshot read failed: %w", err)
	}
	var payload struct {
		URL  string `json:"url"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("parse gh issue snapshot: %w", err)
	}
	if strings.TrimSpace(payload.URL) != issueURL {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("gh issue snapshot URL does not match the linked issue")
	}
	return port.ExecutionIssueSnapshot{URL: issueURL, Body: payload.Body}, nil
}

var _ port.ExecutionIssueSnapshotReader = Provider{}
