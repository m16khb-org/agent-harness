package core

import (
	"fmt"
	"net/url"
	"strings"
)

func validateRemoteArtifactURL(artifactURL, provider, kind string) error {
	if artifactURL == "" {
		return fmt.Errorf("remote artifact url is required")
	}
	if strings.ContainsAny(artifactURL, "\x00\r\n\t ") {
		return fmt.Errorf("remote artifact url must not contain whitespace or control characters")
	}
	parsed, err := url.Parse(artifactURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("remote artifact url must be an http(s) URL")
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.EscapedPath())
	switch provider + ":" + kind {
	case "github:pr":
		parts := splitIssueOpsURLPath(path)
		if host != "github.com" || len(parts) != 4 || parts[2] != "pull" || !isDecimalString(parts[3]) {
			return fmt.Errorf("remote artifact url must be a GitHub pull request URL")
		}
	case "gitlab:mr":
		parts := splitIssueOpsURLPath(path)
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" || !isDecimalString(parts[len(parts)-1]) {
			return fmt.Errorf("remote artifact url must be a GitLab merge request URL")
		}
	default:
		return fmt.Errorf("remote artifact provider/kind combination is unsupported")
	}
	return nil
}

func validateRemoteArtifactMatchesIssue(issueURL, artifactURL, provider, kind string) error {
	issueKey := issueOpsRemoteProjectKey(issueURL, provider, "issue")
	artifactKey := issueOpsRemoteProjectKey(artifactURL, provider, kind)
	if issueKey == "" || artifactKey == "" {
		return nil
	}
	if issueKey != artifactKey {
		return fmt.Errorf("remote artifact url must match linked issue project")
	}
	return nil
}
