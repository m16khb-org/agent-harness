package harnessapp

import (
	commandguardadapter "agent-harness/internal/adapter/commandguard"
	"agent-harness/internal/adapter/lifecycle"
	remoteartifactadapter "agent-harness/internal/adapter/remoteartifact"
)

// configureRemoteArtifactRules는 원격 artifact 명령 해석과 staged check 판정을 설치한다.
func configureRemoteArtifactRules() {
	lifecycle.KoreanBlockReason = remoteartifactadapter.KoreanBlockReason
	lifecycle.VCSIssueLinkingBlockReason = remoteartifactadapter.VCSIssueLinkingBlockReason
	lifecycle.IssueEditTargetFromCommand = remoteartifactadapter.IssueEditTargetFromCommand
	lifecycle.PullRequestBranchInfoFromCommand = remoteartifactadapter.PullRequestBranchInfoFromCommand
	lifecycle.StagedCheckDecision = commandguardadapter.StagedCheckDecision
}
