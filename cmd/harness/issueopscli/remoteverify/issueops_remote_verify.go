package remoteverify

import (
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

type liveRemoteArtifact struct {
	URL       string
	Labels    []string
	Assignees []string
	Merged    bool
}

func VerifyRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
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

func VerifyRemoteArtifactMergedLive(artifact core.IssueOpsRemoteArtifactVerification) error {
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
