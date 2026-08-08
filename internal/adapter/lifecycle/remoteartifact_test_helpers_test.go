package lifecycle

import (
	commandguardadapter "agent-harness/internal/adapter/commandguard"
	remoteartifactadapter "agent-harness/internal/adapter/remoteartifact"
)

// production wiring과 같은 규칙을 설치한다.
func init() {
	KoreanBlockReason = remoteartifactadapter.KoreanBlockReason
	VCSIssueLinkingBlockReason = remoteartifactadapter.VCSIssueLinkingBlockReason
	IssueEditTargetFromCommand = remoteartifactadapter.IssueEditTargetFromCommand
	PullRequestBranchInfoFromCommand = remoteartifactadapter.PullRequestBranchInfoFromCommand
	StagedCheckDecision = commandguardadapter.StagedCheckDecision
}
