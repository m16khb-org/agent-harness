package gitlab

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"issueops/internal/port"
)

// ClosePullRequest closes an open merge request and verifies the final state by
// readback. A merged request is reported, never mutated: cleanup after a merge
// belongs to the finish path, not to abandon.
func (Provider) ClosePullRequest(req port.IssueProviderClosePullRequestRequest) (port.IssueProviderClosePullRequestResult, error) {
	hostname, projectPath, iid, err := parseGitLabMergeRequestURL(req.ArtifactURL)
	if err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "gitlab"}, err
	}
	artifactURL := strings.TrimSpace(req.ArtifactURL)
	endpoint := "projects/" + url.PathEscape(projectPath) + "/merge_requests/" + iid
	result := port.IssueProviderClosePullRequestResult{OK: true, Provider: "gitlab", ArtifactURL: artifactURL, Kind: "mr"}
	if !req.Confirm {
		result.Preview = fmt.Sprintf("[dry-run] would execute: glab api %s --hostname %s --method PUT -f state_event=close; then readback state", endpoint, hostname)
		return result, nil
	}
	state, err := readGlabMergeRequestState(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "gitlab"}, err
	}
	result.State = state
	switch {
	case strings.EqualFold(state, "merged"):
		result.Merged = true
		return result, nil
	case strings.EqualFold(state, "closed"):
		result.Closed, result.AlreadyClosed = true, true
		return result, nil
	}
	if _, err := runGlabAPI(req.Repo, hostname, endpoint, "--method", "PUT", "-f", "state_event=close"); err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "gitlab"}, err
	}
	state, err = readGlabMergeRequestState(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "gitlab"}, err
	}
	result.State = state
	if !strings.EqualFold(state, "closed") {
		return port.IssueProviderClosePullRequestResult{OK: false, Provider: "gitlab", ArtifactURL: artifactURL, Kind: "mr", State: state},
			fmt.Errorf("merge request close was not verified: state=%s", state)
	}
	result.Closed = true
	return result, nil
}

func readGlabMergeRequestState(repo, hostname, endpoint string) (string, error) {
	out, err := runGlabAPI(repo, hostname, endpoint)
	if err != nil {
		return "", err
	}
	var payload struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse glab merge request state: %w", err)
	}
	return payload.State, nil
}
