package remoteverify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"agent-harness/internal/adapter/provider/remoteparse"
)

func fetchGitHubPullRequestArtifact(artifactURL string) (liveRemoteArtifact, error) {
	out, err := runRemoteVerifyCommand(func() *exec.Cmd {
		return exec.Command("gh", "pr", "view", artifactURL, "--json", "url,labels,assignees,state,mergedAt,headRefName,headRefOid,baseRefName")
	})
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitHub PR through gh failed: %w", commandOutputError(err))
	}
	var payload struct {
		URL         string `json:"url"`
		State       string `json:"state"`
		MergedAt    string `json:"mergedAt"`
		HeadRefName string `json:"headRefName"`
		HeadRefOid  string `json:"headRefOid"`
		BaseRefName string `json:"baseRefName"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("decode GitHub PR verification: %w", err)
	}
	artifact := liveRemoteArtifact{
		URL:         payload.URL,
		Merged:      strings.EqualFold(payload.State, "MERGED") || strings.TrimSpace(payload.MergedAt) != "",
		HeadRefName: payload.HeadRefName,
		HeadRefOID:  payload.HeadRefOid,
		BaseRefName: payload.BaseRefName,
	}
	for _, label := range payload.Labels {
		artifact.Labels = append(artifact.Labels, label.Name)
	}
	for _, assignee := range payload.Assignees {
		artifact.Assignees = append(artifact.Assignees, assignee.Login, assignee.Name)
	}
	return artifact, nil
}

func fetchGitHubIssueArtifact(artifactURL string) (liveRemoteArtifact, error) {
	out, err := runRemoteVerifyCommand(func() *exec.Cmd {
		return exec.Command("gh", "issue", "view", artifactURL, "--json", "url,labels,assignees,state")
	})
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitHub issue through gh failed: %w", commandOutputError(err))
	}
	var payload struct {
		URL    string `json:"url"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("decode GitHub issue verification: %w", err)
	}
	// Issues never merge; Merged stays false so only label/assignee evidence is
	// checked by VerifyRemoteArtifactLive.
	artifact := liveRemoteArtifact{URL: payload.URL}
	for _, label := range payload.Labels {
		artifact.Labels = append(artifact.Labels, label.Name)
	}
	for _, assignee := range payload.Assignees {
		artifact.Assignees = append(artifact.Assignees, assignee.Login, assignee.Name)
	}
	return artifact, nil
}

func fetchGitLabIssueArtifact(artifactURL string) (liveRemoteArtifact, error) {
	parsed, err := url.Parse(artifactURL)
	if err != nil {
		return liveRemoteArtifact{}, err
	}
	parts := remoteparse.SplitGitLabIssuePath(parsed.EscapedPath())
	if parts.Project == "" || parts.IID == "" || parts.Kind != "issues" {
		return liveRemoteArtifact{}, fmt.Errorf("remote artifact url must be a GitLab issue URL")
	}
	endpoint := "projects/" + url.PathEscape(parts.Project) + "/issues/" + parts.IID
	out, err := runRemoteVerifyCommand(func() *exec.Cmd {
		return exec.Command("glab", "api", endpoint, "--hostname", parsed.Hostname())
	})
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitLab issue through glab failed: %w", commandOutputError(err))
	}
	var payload struct {
		WebURL    string   `json:"web_url"`
		State     string   `json:"state"`
		Labels    []string `json:"labels"`
		Assignees []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("decode GitLab issue verification: %w", err)
	}
	artifact := liveRemoteArtifact{URL: payload.WebURL, Labels: payload.Labels}
	for _, assignee := range payload.Assignees {
		if assignee.ID != 0 {
			artifact.Assignees = append(artifact.Assignees, strconv.Itoa(assignee.ID))
		}
		artifact.Assignees = append(artifact.Assignees, assignee.Username, assignee.Name)
	}
	return artifact, nil
}

func fetchGitLabMergeRequestArtifact(artifactURL string) (liveRemoteArtifact, error) {
	parsed, err := url.Parse(artifactURL)
	if err != nil {
		return liveRemoteArtifact{}, err
	}
	parts := remoteparse.SplitGitLabMRPath(parsed.EscapedPath())
	if parts.Project == "" || parts.IID == "" {
		return liveRemoteArtifact{}, fmt.Errorf("remote artifact url must be a GitLab merge request URL")
	}
	endpoint := "projects/" + url.PathEscape(parts.Project) + "/merge_requests/" + parts.IID
	out, err := runRemoteVerifyCommand(func() *exec.Cmd {
		return exec.Command("glab", "api", endpoint, "--hostname", parsed.Hostname())
	})
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitLab MR through glab failed: %w", commandOutputError(err))
	}
	var payload struct {
		WebURL       string   `json:"web_url"`
		State        string   `json:"state"`
		MergedAt     string   `json:"merged_at"`
		SourceBranch string   `json:"source_branch"`
		TargetBranch string   `json:"target_branch"`
		SHA          string   `json:"sha"`
		Labels       []string `json:"labels"`
		Assignees    []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("decode GitLab MR verification: %w", err)
	}
	artifact := liveRemoteArtifact{
		URL:         payload.WebURL,
		Labels:      payload.Labels,
		Merged:      strings.EqualFold(payload.State, "merged") || strings.TrimSpace(payload.MergedAt) != "",
		HeadRefName: payload.SourceBranch,
		HeadRefOID:  payload.SHA,
		BaseRefName: payload.TargetBranch,
	}
	for _, assignee := range payload.Assignees {
		if assignee.ID != 0 {
			artifact.Assignees = append(artifact.Assignees, strconv.Itoa(assignee.ID))
		}
		artifact.Assignees = append(artifact.Assignees, assignee.Username, assignee.Name)
	}
	return artifact, nil
}
