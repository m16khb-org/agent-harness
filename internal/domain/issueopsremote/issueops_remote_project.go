package remote

import (
	"fmt"
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

func normalizedWebHost(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	// net.JoinHostPort와 같은 규칙이지만 net을 import하지 않는다. 호스트 정규화는
	// 순수 문자열 규칙이고, domain은 net을 의존해서는 안 된다. 아래 IPv6 분기가
	// 이미 같은 대괄호 처리를 하므로 표기도 일관된다.
	if port != "" && port != "443" {
		if strings.Contains(host, ":") {
			return "[" + host + "]:" + port
		}
		return host + ":" + port
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
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

// IssueArtifactDir은 linked issue URL로 결정하는 봉인 아티팩트 디렉터리
// (워크트리 상대, slash)다: `.agent-harness/issues/<n>/artifact`. 번호를 알 수
// 없으면 빈 문자열이며, 읽는 쪽은 그것을 legacy `.agent-harness/artifact`로
// 해석한다(#482).
func IssueArtifactDir(issueURL string) string {
	if n := IssueNumber(issueURL); n != "" {
		return ".agent-harness/issues/" + n + "/artifact"
	}
	return ""
}
