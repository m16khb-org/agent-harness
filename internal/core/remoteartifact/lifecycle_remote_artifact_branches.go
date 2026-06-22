package remoteartifact

import "strings"

type PullRequestBranchInfo struct {
	Provider   string
	Kind       string
	HeadBranch string
	BaseBranch string
}

func PullRequestBranchInfoFromCommand(tool, command, repo string) (PullRequestBranchInfo, bool) {
	if !remoteArtifactGateAppliesToTool(tool) {
		return PullRequestBranchInfo{}, false
	}
	artifact, ok := parseGHRemoteArtifactCommand(command, repo)
	if !ok || artifact.action != "create" {
		return PullRequestBranchInfo{}, false
	}
	if artifact.kind != "pr" && artifact.kind != "mr" {
		return PullRequestBranchInfo{}, false
	}
	return PullRequestBranchInfo{
		Provider:   strings.TrimSpace(artifact.provider),
		Kind:       strings.TrimSpace(artifact.kind),
		HeadBranch: strings.TrimSpace(artifact.headBranch),
		BaseBranch: strings.TrimSpace(artifact.baseBranch),
	}, true
}
