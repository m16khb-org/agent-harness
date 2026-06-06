package core

import (
	"fmt"
	"net/url"
	"strings"
)

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
