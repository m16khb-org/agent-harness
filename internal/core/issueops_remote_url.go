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

func validateIssueOpsChildMatchesParent(parentURL, childURL string) error {
	provider := issueOpsProviderFromURL(parentURL)
	if provider == "" {
		return nil
	}
	if issueOpsIssueNumber(childURL) == "" {
		return fmt.Errorf("child issue url must be a numeric %s issue URL", provider)
	}
	parentKey := issueOpsRemoteProjectKey(parentURL, provider, "issue")
	childKey := issueOpsRemoteProjectKey(childURL, provider, "issue")
	if parentKey == "" || childKey == "" {
		return fmt.Errorf("child issue url must be a %s issue URL", provider)
	}
	if parentKey != childKey {
		return fmt.Errorf("child issue url must match linked parent issue project")
	}
	return nil
}

func issueOpsRemoteProjectKey(rawURL, provider, kind string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	parts := splitIssueOpsURLPath(strings.ToLower(parsed.EscapedPath()))
	switch provider + ":" + kind {
	case "github:issue":
		if host != "github.com" || len(parts) != 4 || parts[2] != "issues" {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "github:pr":
		if host != "github.com" || len(parts) != 4 || parts[2] != "pull" {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "gitlab:issue":
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "issues" {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	case "gitlab:mr":
		if host == "github.com" || len(parts) < 4 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	default:
		return ""
	}
}

func splitIssueOpsURLPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func cleanIssueOpsRemoteValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func invalidIssueOpsRemoteAssignee(values []string) string {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "@me", "me", "self", "@self", "current_user", "current-user":
			return value
		}
	}
	return ""
}

func issueOpsProviderFromURL(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	if host == "github.com" && strings.Contains(path, "/issues/") {
		return "github"
	}
	if strings.Contains(host, "gitlab") || strings.Contains(path, "/-/issues/") {
		return "gitlab"
	}
	return ""
}

func issueOpsIssueNumber(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if part == "issues" && i+1 < len(parts) {
			number := parts[i+1]
			for _, r := range number {
				if r < '0' || r > '9' {
					return ""
				}
			}
			return number
		}
	}
	return ""
}
