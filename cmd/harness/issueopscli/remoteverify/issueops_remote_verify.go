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
	// HeadRefName/HeadRefOID는 PR/MR의 source 브랜치와 그 tip이다. cleanup
	// remote-branch가 "이 artifact가 정말 이 브랜치의 것인가"와 "머지 이후
	// push된 커밋이 없는가"를 판정하는 유일한 근거다(#116 게이트 ⑨·⑩).
	HeadRefName string
	HeadRefOID  string
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
	case "github:issue":
		artifact, err = fetchGitHubIssueArtifact(strings.TrimSpace(req.URL))
	case "gitlab:issue":
		artifact, err = fetchGitLabIssueArtifact(strings.TrimSpace(req.URL))
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
	_, err := VerifyRemoteArtifactMergedHeadLive(artifact)
	return err
}

// VerifyRemoteArtifactMergedHeadLive는 머지 검증과 head ref 관측을 한 번의
// readback으로 수행한다. 두 값이 다른 시점의 관측이면 cleanup remote-branch의
// OID CAS가 무의미해지므로 분리된 조회 표면을 두지 않는다(#116).
func VerifyRemoteArtifactMergedHeadLive(artifact core.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
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
		return core.IssueOpsCleanupRemoteBranchArtifactHead{},
			fmt.Errorf("unsupported remote artifact for merge verification: %s:%s", provider, kind)
	}
	if err != nil {
		return core.IssueOpsCleanupRemoteBranchArtifactHead{}, err
	}
	if !live.Merged {
		return core.IssueOpsCleanupRemoteBranchArtifactHead{},
			fmt.Errorf("remote artifact is not verified merged: %s", artifact.URL)
	}
	return core.IssueOpsCleanupRemoteBranchArtifactHead{
		HeadRefName: strings.TrimSpace(live.HeadRefName),
		HeadRefOID:  strings.TrimSpace(live.HeadRefOID),
	}, nil
}
