package remote

import (
	"fmt"
	"net/url"
	"strings"
)

func ValidateArtifactURL(artifactURL, provider, kind string) error {
	if artifactURL == "" {
		return fmt.Errorf("remote artifact url is required")
	}
	if strings.ContainsAny(artifactURL, "\x00\r\n\t ") {
		return fmt.Errorf("remote artifact url must not contain whitespace or control characters")
	}
	parsed, err := url.Parse(artifactURL)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != artifactURL {
		return fmt.Errorf("remote artifact url must be a canonical HTTPS URL")
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.EscapedPath())
	switch provider + ":" + kind {
	case "github:pr":
		parts := SplitURLPath(path)
		if len(parts) != 4 || parts[2] != "pull" || !isDecimalString(parts[3]) {
			return fmt.Errorf("remote artifact url must be a GitHub pull request URL")
		}
	case "gitlab:mr":
		parts := SplitURLPath(path)
		if host == "github.com" || len(parts) < 5 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" || !isDecimalString(parts[len(parts)-1]) {
			return fmt.Errorf("remote artifact url must be a GitLab merge request URL")
		}
	default:
		return fmt.Errorf("remote artifact provider/kind combination is unsupported")
	}
	return nil
}

func ValidateArtifactMatchesIssue(issueURL, artifactURL, provider, kind string) error {
	issueKey := ProjectKey(issueURL, provider, "issue")
	artifactKey := ProjectKey(artifactURL, provider, kind)
	if issueKey == "" || artifactKey == "" {
		return fmt.Errorf("remote issue and artifact project authority must both be canonical")
	}
	if issueKey != artifactKey {
		return fmt.Errorf("remote artifact url must match linked issue project")
	}
	return nil
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
