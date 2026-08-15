package issueops

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"agent-harness/internal/contract/issueops"
	remote "agent-harness/internal/domain/issueopsremote"
)

// SyncRemoteIssueGraph posts a comment on the cycle's parent issue listing all
// typed related-issue links from the cycle's issue graph. This mirrors the
// local typed graph to the remote provider so collaborators can traverse the
// decision structure without local harness state.
func SyncRemoteIssueGraph(record issueops.IssueOpsRecord) (map[string]any, error) {
	issueURL := strings.TrimSpace(record.IssueURL)
	if issueURL == "" {
		return nil, fmt.Errorf("cannot sync graph: no issue_url on cycle")
	}
	if len(record.IssueLinks) == 0 {
		return map[string]any{
			"ok":      true,
			"synced":  false,
			"message": "no issue graph links to sync",
		}, nil
	}
	provider := ResolveRecordProvider(record)
	if provider == "" {
		return nil, fmt.Errorf("cannot determine provider from cycle")
	}

	var body strings.Builder
	body.WriteString("## Related Issue Graph\n\n")
	body.WriteString("This issue graph was recorded by agent-harness IssueOps:\n\n")
	for _, link := range record.IssueLinks {
		label := linkTypeLabel(link.Type)
		body.WriteString(fmt.Sprintf("- **%s**: %s", label, link.URL))
		if link.Title != "" {
			body.WriteString(fmt.Sprintf(" (%s)", link.Title))
		}
		body.WriteString("\n")
	}
	body.WriteString(fmt.Sprintf("\nCycle: `%s`", record.ID))

	switch provider {
	case "github":
		return syncGitHubGraph(issueURL, body.String())
	case "gitlab":
		return syncGitLabGraph(issueURL, body.String())
	default:
		return nil, fmt.Errorf("sync not supported for provider %q", provider)
	}
}

func syncGitHubGraph(issueURL, body string) (map[string]any, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI is not installed")
	}
	cmd := exec.Command("gh", "issue", "comment", issueURL, "--body", body)
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh issue comment failed: %s", stderr)
	}
	commentURL := strings.TrimSpace(string(out))
	return map[string]any{
		"ok":          true,
		"synced":      true,
		"provider":    "github",
		"comment_url": commentURL,
		"link_count":  getLinkCount(body),
	}, nil
}

func syncGitLabGraph(issueURL, body string) (map[string]any, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return nil, fmt.Errorf("glab CLI is not installed")
	}
	cmd := exec.Command("glab", "issue", "note", issueURL, "--message", body)
	out, err := cmd.Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("glab issue note failed: %s", stderr)
	}
	noteURL := strings.TrimSpace(string(out))
	return map[string]any{
		"ok":         true,
		"synced":     true,
		"provider":   "gitlab",
		"note_url":   noteURL,
		"link_count": getLinkCount(body),
	}, nil
}

func getLinkCount(body string) int {
	count := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- **") {
			count++
		}
	}
	return count
}

func linkTypeLabel(linkType string) string {
	switch linkType {
	case "depends-on":
		return "Depends on"
	case "blocks":
		return "Blocks"
	case "supersedes":
		return "Supersedes"
	case "follows-up":
		return "Follows up"
	case "duplicates":
		return "Duplicates"
	case "splits-from":
		return "Splits from"
	case "implements":
		return "Implements"
	default:
		return linkType
	}
}

// ResolveRecordProvider infers the VCS provider for a lifecycle record:
// branch-prepare/remote-artifact evidence first, then the issue URL host.
// URL 추론은 부분 문자열 매칭이라 "gitlab"이 들어간 self-hosted 도메인은
// gitlab으로 해석되고, 그 문자열이 없는 커스텀 도메인만 ""로 떨어져
// 명시 --provider가 필요하다.
func ResolveRecordProvider(record issueops.IssueOpsRecord) string {
	if record.BranchPrepare != nil && record.BranchPrepare.Provider != "" {
		return record.BranchPrepare.Provider
	}
	if record.RemoteArtifact != nil && record.RemoteArtifact.Provider != "" {
		return record.RemoteArtifact.Provider
	}
	if strings.Contains(record.IssueURL, "github.com") {
		return "github"
	}
	if strings.Contains(record.IssueURL, "gitlab") {
		return "gitlab"
	}
	return ""
}

// InferProviderFromRepoRemotes는 저장소의 remote URL을 관측해 provider를 단일
// 판별한다. record가 provider를 모를 때만 쓰이며, 최초 이슈 생성의 bootstrap
// 순환을 끊는 경로다(#300).
//
// 관측은 여기(adapter)가, 판별 규칙은 domain이 소유한다.
func InferProviderFromRepoRemotes(repo string) (string, error) {
	code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "remote", "-v")
	if code != 0 {
		return "", fmt.Errorf("cannot determine provider: repository remotes could not be observed; pass --provider github|gitlab")
	}
	urls := []string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			urls = append(urls, fields[1])
		}
	}
	return remote.ProviderFromRemoteURLs(urls)
}

func ResolveProviderProjectAuthority(repo, provider string) (string, error) {
	code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("cannot determine project authority from origin remote")
	}
	projectKey, err := remote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(out), provider)
	if err != nil {
		return "", fmt.Errorf("cannot determine project authority: %w", err)
	}
	return projectKey, nil
}
