package remote

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var scpRemotePattern = regexp.MustCompile(`^[^@/:]+@([^/:[\]]+):(.+)$`)

func ValidateChildMatchesParent(parentURL, childURL string) error {
	provider := ProviderFromURL(parentURL)
	if provider == "" {
		return nil
	}
	if IssueNumber(childURL) == "" {
		return fmt.Errorf("child issue url must be a numeric %s issue or work item URL", provider)
	}
	parentKey := ProjectKey(parentURL, provider, "issue")
	childKey := ProjectKey(childURL, provider, "issue")
	if parentKey == "" || childKey == "" {
		return fmt.Errorf("child issue url must be a %s issue URL", provider)
	}
	if parentKey != childKey {
		return fmt.Errorf("child issue url must match linked parent issue project")
	}
	return nil
}

func ProjectKey(rawURL, provider, kind string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Scheme != "https" {
		return ""
	}
	host := normalizedWebHost(parsed)
	parts := SplitURLPath(strings.ToLower(parsed.EscapedPath()))
	switch provider + ":" + kind {
	case "github:issue":
		if len(parts) != 4 || parts[2] != "issues" || !isDecimalString(parts[3]) {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "github:pr":
		if len(parts) != 4 || parts[2] != "pull" || !isDecimalString(parts[3]) {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "gitlab:issue":
		if strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) < 5 || parts[len(parts)-3] != "-" || !isGitLabIssueLikePath(parts[len(parts)-2]) || !isDecimalString(parts[len(parts)-1]) {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	case "gitlab:mr":
		if strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) < 5 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" || !isDecimalString(parts[len(parts)-1]) {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	default:
		return ""
	}
}

func ProjectKeyFromGitRemoteURL(rawURL, provider string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.ContainsAny(rawURL, "\x00\r\n\t ") {
		return "", fmt.Errorf("git remote URL is empty or contains whitespace")
	}
	host, remotePath := "", ""
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "https" && parsed.Scheme != "ssh" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("git remote URL scheme or authority is unsupported")
		}
		if parsed.Scheme == "https" {
			if parsed.User != nil {
				return "", fmt.Errorf("git remote HTTPS URL must not contain userinfo")
			}
			host = normalizedWebHost(parsed)
		} else {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return "", fmt.Errorf("git remote SSH URL must not contain password userinfo")
			}
			host = strings.ToLower(parsed.Hostname())
		}
		remotePath = strings.TrimPrefix(parsed.EscapedPath(), "/")
	} else if match := scpRemotePattern.FindStringSubmatch(rawURL); len(match) == 3 {
		host, remotePath = strings.ToLower(match[1]), match[2]
	} else {
		return "", fmt.Errorf("git remote URL must be HTTPS, SSH, or SCP syntax")
	}
	remotePath = strings.TrimSuffix(strings.Trim(remotePath, "/"), ".git")
	parts := SplitURLPath(strings.ToLower(remotePath))
	if provider == "github" {
		if len(parts) != 2 {
			return "", fmt.Errorf("git remote URL does not identify one GitHub project")
		}
	} else if provider == "gitlab" {
		if host == "github.com" || len(parts) < 2 {
			return "", fmt.Errorf("git remote URL does not identify one GitLab project")
		}
	} else {
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
	return host + "/" + strings.Join(parts, "/"), nil
}

func ValidateGitRemoteMatchesIssue(issueURL, remoteURL, provider string) error {
	issueKey := ProjectKey(issueURL, provider, "issue")
	if issueKey == "" {
		return fmt.Errorf("linked issue project authority is missing or malformed")
	}
	remoteKey, err := ProjectKeyFromGitRemoteURL(remoteURL, provider)
	if err != nil {
		return err
	}
	if provider == "gitlab" && gitRemoteUsesSSHTransport(remoteURL) {
		parsedIssue, parseErr := url.Parse(issueURL)
		if parseErr != nil {
			return fmt.Errorf("linked issue project authority is malformed")
		}
		port := parsedIssue.Port()
		if port != "" && port != "443" && port != "80" {
			return fmt.Errorf("GitLab SSH remote proves host and project but not nondefault web port %q; configure a trusted host profile mapping", port)
		}
	}
	if remoteKey != issueKey {
		return fmt.Errorf("git push target must match linked issue project authority")
	}
	return nil
}

func normalizedWebHost(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port != "" && port != "443" {
		return net.JoinHostPort(host, port)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func gitRemoteUsesSSHTransport(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	return err == nil && parsed.Scheme == "ssh" || (err != nil || parsed.Scheme == "") && scpRemotePattern.MatchString(strings.TrimSpace(rawURL))
}

func ProviderFromURL(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	if host == "github.com" && strings.Contains(path, "/issues/") {
		return "github"
	}
	if strings.Contains(host, "gitlab") || strings.Contains(path, "/-/issues/") || strings.Contains(path, "/-/work_items/") {
		return "gitlab"
	}
	return ""
}
func IssueNumber(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if isGitLabIssueLikePath(part) && i+1 < len(parts) {
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

func isGitLabIssueLikePath(part string) bool {
	return part == "issues" || part == "work_items"
}
