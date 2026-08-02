package issueopscompletion

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	completioncontract "agent-harness/internal/contract/issueopscompletion"
)

func ValidateArtifact(record completioncontract.RecordSnapshot, requestedURL string) error {
	artifact := record.Artifact
	if artifact == nil {
		return fmt.Errorf("execution completion requires a durable verified remote artifact")
	}
	if strings.TrimSpace(artifact.VerifiedAt) == "" {
		return fmt.Errorf("execution completion requires remote artifact verification")
	}
	baseBranch := strings.TrimSpace(record.BaseBranch)
	if baseBranch == "" {
		return fmt.Errorf("execution completion requires a prepared target branch")
	}
	if artifact.TargetBranch != baseBranch {
		return fmt.Errorf("remote artifact target branch must match prepared base branch")
	}
	if artifact.Provider != "github" && artifact.Provider != "gitlab" {
		return fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if (artifact.Provider == "github" && artifact.Kind != "pr") || (artifact.Provider == "gitlab" && artifact.Kind != "mr") {
		return fmt.Errorf("remote artifact kind must match its provider")
	}
	if err := validateArtifactURL(artifact.URL, artifact.Provider, artifact.Kind); err != nil {
		return err
	}
	if artifactProjectKey(artifact.URL, artifact.Provider, artifact.Kind) != artifactProjectKey(record.IssueURL, artifact.Provider, "issue") {
		return fmt.Errorf("remote artifact url must match linked issue project")
	}
	if !slices.Equal(canonicalValues(artifact.Labels), artifact.Labels) || len(artifact.Labels) == 0 {
		return fmt.Errorf("remote artifact labels are not canonical")
	}
	if !slices.Equal(canonicalValues(artifact.Assignees), artifact.Assignees) || len(artifact.Assignees) == 0 {
		return fmt.Errorf("remote artifact assignees are not canonical")
	}
	for _, assignee := range artifact.Assignees {
		switch strings.ToLower(assignee) {
		case "@me", "me", "self", "@self", "current_user", "current-user":
			return fmt.Errorf("remote artifact assignee must be a verified provider user")
		}
	}
	if artifact.URL != strings.TrimSpace(requestedURL) {
		return fmt.Errorf("completion remote_artifact_url must match the durable verified artifact")
	}
	return nil
}

func validateArtifactURL(raw, provider, kind string) error {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\t ") {
		return fmt.Errorf("remote artifact url must be a canonical HTTPS URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != raw {
		return fmt.Errorf("remote artifact url must be a canonical HTTPS URL")
	}
	parts := splitPath(parsed.EscapedPath())
	switch provider + ":" + kind {
	case "github:pr":
		if strings.ToLower(parsed.Hostname()) != "github.com" || len(parts) != 4 || parts[2] != "pull" || !decimal(parts[3]) {
			return fmt.Errorf("remote artifact url must be a GitHub pull request URL")
		}
	case "gitlab:mr":
		if strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) < 5 || parts[len(parts)-3] != "-" || parts[len(parts)-2] != "merge_requests" || !decimal(parts[len(parts)-1]) {
			return fmt.Errorf("remote artifact url must be a GitLab merge request URL")
		}
	default:
		return fmt.Errorf("remote artifact provider/kind combination is unsupported")
	}
	return nil
}

func artifactProjectKey(raw, provider, kind string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return ""
	}
	parts := splitPath(parsed.EscapedPath())
	switch provider {
	case "github":
		if len(parts) != 4 || (kind == "issue" && parts[2] != "issues") || (kind == "pr" && parts[2] != "pull") || !decimal(parts[3]) {
			return ""
		}
		return strings.ToLower(parsed.Host + "/" + parts[0] + "/" + parts[1])
	case "gitlab":
		separator := slices.Index(parts, "-")
		if separator < 2 || separator != len(parts)-3 || !decimal(parts[len(parts)-1]) ||
			(kind == "issue" && parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") ||
			(kind == "mr" && parts[len(parts)-2] != "merge_requests") {
			return ""
		}
		return strings.ToLower(parsed.Host + "/" + strings.Join(parts[:separator], "/"))
	default:
		return ""
	}
}

func canonicalValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsRune(value, '\x00') {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func splitPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, strings.ToLower(part))
		}
	}
	return result
}

func decimal(value string) bool {
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
