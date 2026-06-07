package remoteverify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"agent-harness/cmd/harness/issueopscli/remoteparse"
)

func fetchGitHubPullRequestArtifact(artifactURL string) (liveRemoteArtifact, error) {
	out, err := exec.Command("gh", "pr", "view", artifactURL, "--json", "url,labels,assignees,state,mergedAt").Output()
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitHub PR through gh failed: %w", commandOutputError(err))
	}
	var payload struct {
		URL      string `json:"url"`
		State    string `json:"state"`
		MergedAt string `json:"mergedAt"`
		Labels   []struct {
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
	artifact := liveRemoteArtifact{URL: payload.URL, Merged: strings.EqualFold(payload.State, "MERGED") || strings.TrimSpace(payload.MergedAt) != ""}
	for _, label := range payload.Labels {
		artifact.Labels = append(artifact.Labels, label.Name)
	}
	for _, assignee := range payload.Assignees {
		artifact.Assignees = append(artifact.Assignees, assignee.Login, assignee.Name)
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
	cmd := exec.Command("glab", "api", endpoint, "--hostname", parsed.Hostname())
	out, err := cmd.Output()
	if err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("verify GitLab MR through glab failed: %w", commandOutputError(err))
	}
	var payload struct {
		WebURL    string   `json:"web_url"`
		State     string   `json:"state"`
		MergedAt  string   `json:"merged_at"`
		Labels    []string `json:"labels"`
		Assignees []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return liveRemoteArtifact{}, fmt.Errorf("decode GitLab MR verification: %w", err)
	}
	artifact := liveRemoteArtifact{URL: payload.WebURL, Labels: payload.Labels, Merged: strings.EqualFold(payload.State, "merged") || strings.TrimSpace(payload.MergedAt) != ""}
	for _, assignee := range payload.Assignees {
		if assignee.ID != 0 {
			artifact.Assignees = append(artifact.Assignees, strconv.Itoa(assignee.ID))
		}
		artifact.Assignees = append(artifact.Assignees, assignee.Username, assignee.Name)
	}
	return artifact, nil
}
