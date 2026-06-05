package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"agent-harness/internal/core"
)

type liveRemoteArtifact struct {
	URL       string
	Labels    []string
	Assignees []string
	Merged    bool
}

func verifyIssueOpsRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	var artifact liveRemoteArtifact
	var err error
	switch provider + ":" + kind {
	case "github:pr":
		artifact, err = fetchGitHubPullRequestArtifact(strings.TrimSpace(req.URL))
	case "gitlab:mr":
		artifact, err = fetchGitLabMergeRequestArtifact(strings.TrimSpace(req.URL))
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if err := requireRemoteValues("label", req.Labels, artifact.Labels); err != nil {
		return err
	}
	if err := requireRemoteValues("assignee", req.Assignees, artifact.Assignees); err != nil {
		return err
	}
	return nil
}

func verifyIssueOpsRemoteArtifactMergedLive(artifact core.IssueOpsRemoteArtifactVerification) error {
	provider := strings.ToLower(strings.TrimSpace(artifact.Provider))
	kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	var live liveRemoteArtifact
	var err error
	switch provider + ":" + kind {
	case "github:pr":
		live, err = fetchGitHubPullRequestArtifact(strings.TrimSpace(artifact.URL))
	case "gitlab:mr":
		live, err = fetchGitLabMergeRequestArtifact(strings.TrimSpace(artifact.URL))
	default:
		return fmt.Errorf("unsupported remote artifact for merge verification: %s:%s", provider, kind)
	}
	if err != nil {
		return err
	}
	if !live.Merged {
		return fmt.Errorf("remote artifact is not verified merged: %s", artifact.URL)
	}
	return nil
}

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
	parts := splitGitLabMRPath(parsed.EscapedPath())
	if parts.project == "" || parts.iid == "" {
		return liveRemoteArtifact{}, fmt.Errorf("remote artifact url must be a GitLab merge request URL")
	}
	endpoint := "projects/" + url.PathEscape(parts.project) + "/merge_requests/" + parts.iid
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

type gitLabMRPathParts struct {
	project string
	iid     string
}

func splitGitLabMRPath(escapedPath string) gitLabMRPathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return gitLabMRPathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return gitLabMRPathParts{project: strings.Join(projectParts, "/"), iid: iid}
	}
	return gitLabMRPathParts{}
}

func requireRemoteValues(kind string, required []string, actual []string) error {
	actualSet := map[string]bool{}
	for _, value := range actual {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			actualSet[value] = true
		}
	}
	missing := []string{}
	for _, value := range required {
		cleaned := strings.TrimSpace(strings.ToLower(value))
		if cleaned != "" && !actualSet[cleaned] {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("remote artifact missing verified %s(s): %s", kind, strings.Join(missing, ", "))
	}
	return nil
}

func commandOutputError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return fmt.Errorf("%s", stderr)
		}
	}
	return err
}
