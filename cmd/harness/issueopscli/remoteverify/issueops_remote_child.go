package remoteverify

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"agent-harness/cmd/harness/issueopscli/remoteparse"
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
	if strings.Contains(parsed.Hostname(), "gitlab") || strings.Contains(parsed.EscapedPath(), "/-/issues/") {
		return VerifyGitLabIssueLive(parsed)
	}
	return nil
}

func VerifyGitHubIssueLive(issueURL string) error {
	if _, err := exec.Command("gh", "issue", "view", strings.TrimSpace(issueURL), "--json", "url,state,title").Output(); err != nil {
		return fmt.Errorf("verify GitHub child issue through gh failed: %w", commandOutputError(err))
	}
	return nil
}

func VerifyGitLabIssueLive(parsed *url.URL) error {
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

func SetChildIssueVerifier(verifier func(string) error) func(string) error {
	previous := childIssueVerifier
	childIssueVerifier = verifier
	return previous
}
