package vcsissue

import (
	commandguardadapter "agent-harness/internal/adapter/commandguard"
	lifecyclerad "agent-harness/internal/adapter/lifecycle"
	remoteartifactadapter "agent-harness/internal/adapter/remoteartifact"
)

// production wiring과 같은 규칙을 설치한다. 이 package는 pre-tool-use 경로에서
// lifecycle 판정을 거친다.
func init() {
	lifecyclerad.KoreanBlockReason = remoteartifactadapter.KoreanBlockReason
	lifecyclerad.VCSIssueLinkingBlockReason = remoteartifactadapter.VCSIssueLinkingBlockReason
	lifecyclerad.IssueEditTargetFromCommand = remoteartifactadapter.IssueEditTargetFromCommand
	lifecyclerad.PullRequestBranchInfoFromCommand = remoteartifactadapter.PullRequestBranchInfoFromCommand
	lifecyclerad.StagedCheckDecision = commandguardadapter.StagedCheckDecision
}
