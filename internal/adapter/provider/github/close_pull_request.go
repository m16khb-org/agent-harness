package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"issueops/internal/port"
)

// ClosePullRequest는 미머지 PR을 닫는다. 머지된 PR은 닫지 않고 사실만
// 돌려준다 — 머지 뒤의 정리는 cleanup finish가 소유한다.
func (Provider) ClosePullRequest(req port.IssueProviderClosePullRequestRequest) (port.IssueProviderClosePullRequestResult, error) {
	artifactURL := strings.TrimSpace(req.ArtifactURL)
	if !validCanonicalGitHubPullRequestURL(artifactURL) {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "github"}, fmt.Errorf("pull request url must be a canonical https://<host>/<owner>/<repo>/pull/<number> url")
	}
	result := port.IssueProviderClosePullRequestResult{OK: true, Provider: "github", ArtifactURL: artifactURL, Kind: "pr"}
	if !req.Confirm {
		result.Preview = fmt.Sprintf("[dry-run] would execute: gh pr close %s; gh pr view %s --json state", artifactURL, artifactURL)
		return result, nil
	}
	state, err := readGhPullRequestState(req.Repo, artifactURL)
	if err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "github"}, err
	}
	result.State = state
	switch {
	case strings.EqualFold(state, "MERGED"):
		result.Merged = true
		return result, nil
	case strings.EqualFold(state, "CLOSED"):
		result.Closed, result.AlreadyClosed = true, true
		return result, nil
	}
	closeCmd := exec.Command("gh", "pr", "close", artifactURL)
	if req.Repo != "" {
		closeCmd.Dir = req.Repo
	}
	if err := closeCmd.Run(); err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "github"}, fmt.Errorf("gh pr close failed: %s", ghExecStderr(err))
	}
	state, err = readGhPullRequestState(req.Repo, artifactURL)
	if err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "github"}, err
	}
	result.State = state
	if !strings.EqualFold(state, "CLOSED") {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "github", ArtifactURL: artifactURL, Kind: "pr", State: state},
			fmt.Errorf("pull request close was not verified: state=%s", state)
	}
	result.Closed = true
	return result, nil
}

func readGhPullRequestState(repo, artifactURL string) (string, error) {
	cmd := exec.Command("gh", "pr", "view", artifactURL, "--json", "state")
	if repo != "" {
		cmd.Dir = repo
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr view failed: %s", ghExecStderr(err))
	}
	var payload struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gh pull request state: %w", err)
	}
	return payload.State, nil
}
