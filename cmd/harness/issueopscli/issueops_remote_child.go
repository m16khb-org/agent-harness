package issueopscli

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"agent-harness/cmd/harness/issueopscli/remoteparse"
)

var issueOpsChildIssueVerifier = verifyIssueOpsChildIssueLive

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	if issueOpsChildIssueVerifier == nil {
		return nil
	}
	return issueOpsChildIssueVerifier(childURL)
}

func verifyIssueOpsChildIssueLive(childURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(childURL))
	if err != nil {
		return err
	}
	if parsed.Hostname() == "github.com" {
		return verifyGitHubIssueLive(childURL)
	}
	if strings.Contains(parsed.Hostname(), "gitlab") || strings.Contains(parsed.EscapedPath(), "/-/issues/") {
		return verifyGitLabIssueLive(parsed)
	}
	return nil
}

func verifyGitHubIssueLive(issueURL string) error {
	if _, err := exec.Command("gh", "issue", "view", strings.TrimSpace(issueURL), "--json", "url,state,title").Output(); err != nil {
		return fmt.Errorf("verify GitHub child issue through gh failed: %w", commandOutputError(err))
	}
	return nil
}

func verifyGitLabIssueLive(parsed *url.URL) error {
	parts := remoteparse.SplitGitLabIssuePath(parsed.EscapedPath())
	if parts.Project == "" || parts.IID == "" {
		return fmt.Errorf("child_url must be a GitLab issue URL")
	}
	endpoint := "projects/" + url.PathEscape(parts.Project) + "/issues/" + parts.IID
	cmd := exec.Command("glab", "api", endpoint, "--hostname", parsed.Hostname())
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("verify GitLab child issue through glab failed: %w", commandOutputError(err))
	}
	return nil
}
