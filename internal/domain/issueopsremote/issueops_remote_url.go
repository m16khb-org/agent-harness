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
	return ValidateArtifactMatchesProject(ProjectKey(issueURL, provider, "issue"), artifactURL, provider, kind)
}

// ValidProjectKey reports whether a caller-supplied project key has the shape
// ProjectKey produces: a lowercase host followed by at least one path segment,
// with no scheme, whitespace, or control characters.
func ValidProjectKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" || key != strings.ToLower(key) || strings.ContainsAny(key, "\x00\r\n\t :?#") {
		return false
	}
	if strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || strings.Contains(key, "//") {
		return false
	}
	parts := SplitURLPath(key)
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// EffectiveProjectKey resolves the project that must own the artifact: the
// cycle's sealed code project when it declared one, otherwise the issue's own
// project. An empty code project key therefore keeps the historical behaviour
// for every cycle whose issue and code live in the same place.
func EffectiveProjectKey(codeProjectKey, issueURL, provider string) string {
	if key := strings.TrimSpace(codeProjectKey); key != "" {
		return key
	}
	return ProjectKey(issueURL, provider, "issue")
}

// ValidateArtifactMatchesProject binds an artifact to the project that owns the
// code, which is the issue's project only when both live in the same place. A
// cycle whose issue is filed in a planning project and whose branch lives in a
// service project passes its sealed code project key here instead, so the
// artifact is still pinned to one project — just the right one.
func ValidateArtifactMatchesProject(projectKey, artifactURL, provider, kind string) error {
	projectKey = strings.TrimSpace(projectKey)
	artifactKey := ProjectKey(artifactURL, provider, kind)
	if projectKey == "" || artifactKey == "" {
		return fmt.Errorf("remote issue and artifact project authority must both be canonical")
	}
	if projectKey != artifactKey {
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
