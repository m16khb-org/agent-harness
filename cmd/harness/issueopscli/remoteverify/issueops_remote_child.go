package remoteverify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	policydomain "agent-harness/internal/domain/policy"
	"agent-harness/internal/domain/remoteparse"
)

var childIssueVerifier = VerifyChildIssueLive

func VerifyChildIssueBeforeLink(childURL string) error {
	if childIssueVerifier == nil {
		return nil
	}
	return childIssueVerifier(childURL)
}

func VerifyChildIssueLive(childURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(childURL))
	if err != nil {
		return err
	}
	if parsed.Hostname() == "github.com" {
		return VerifyGitHubIssueLive(childURL)
	}
	if strings.Contains(parsed.Hostname(), "gitlab") || strings.Contains(parsed.EscapedPath(), "/-/issues/") || strings.Contains(parsed.EscapedPath(), "/-/work_items/") {
		return VerifyGitLabIssueLive(parsed)
	}
	return nil
}

func VerifyGitHubIssueLive(issueURL string) error {
	if _, err := runRemoteVerifyCommand(context.Background(), func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "gh", "issue", "view", strings.TrimSpace(issueURL), "--json", "url,state,title")
	}); err != nil {
		return fmt.Errorf("verify GitHub child issue through gh failed: %w", commandOutputError(err))
	}
	return nil
}

func VerifyGitLabIssueLive(parsed *url.URL) error {
	parts := remoteparse.SplitGitLabIssuePath(parsed.EscapedPath())
	if parts.Project == "" || parts.IID == "" {
		return fmt.Errorf("child_url must be a GitLab issue or work item URL")
	}
	kind := parts.Kind
	if kind == "" {
		kind = "issues"
	}
	if kind == "work_items" {
		if _, err := fetchGitLabIssueEndpoint(parsed.Hostname(), parts.Project, kind, parts.IID); err == nil {
			return nil
		}
		out, err := fetchGitLabIssueEndpoint(parsed.Hostname(), parts.Project, "issues", parts.IID)
		if err != nil {
			return fmt.Errorf("verify GitLab child issue through glab failed: %w", commandOutputError(err))
		}
		if err := verifyGitLabIssuePayloadIsTask(out, parts.IID); err != nil {
			return err
		}
		return nil
	}
	if _, err := fetchGitLabIssueEndpoint(parsed.Hostname(), parts.Project, kind, parts.IID); err != nil {
		return fmt.Errorf("verify GitLab child issue through glab failed: %w", commandOutputError(err))
	}
	return nil
}

func fetchGitLabIssueEndpoint(hostname, project, kind, iid string) ([]byte, error) {
	endpoint := "projects/" + url.PathEscape(project) + "/" + kind + "/" + iid
	return runRemoteVerifyCommand(context.Background(), func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "glab", "api", endpoint, "--hostname", hostname)
	})
}

func verifyGitLabIssuePayloadIsTask(out []byte, iid string) error {
	if len(strings.TrimSpace(string(out))) == 0 {
		return fmt.Errorf("gitlab issues/%s fallback did not return task metadata", iid)
	}
	var payload struct {
		Type      string `json:"type"`
		IssueType string `json:"issue_type"`
		WebURL    string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return fmt.Errorf("decode GitLab issue fallback for work item %s: %w", iid, err)
	}
	if strings.EqualFold(payload.Type, "TASK") || strings.EqualFold(payload.IssueType, "task") {
		return nil
	}
	return fmt.Errorf(
		"gitlab issues/%s fallback did not return a Task work item: type=%q issue_type=%q",
		iid,
		policydomain.BoundedDiagnostic(payload.Type, 256),
		policydomain.BoundedDiagnostic(payload.IssueType, 256),
	)
}

func SetChildIssueVerifier(verifier func(string) error) func(string) error {
	previous := childIssueVerifier
	childIssueVerifier = verifier
	return previous
}
