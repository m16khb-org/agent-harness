package lifecycle

import (
	remoteartifactcontract "agent-harness/internal/contract/remoteartifact"
)

// 원격 artifact 명령 해석은 저장소를 읽는다. 구현은 composition root가 설치한다.
var (
	KoreanBlockReason                func(tool, command, repo string) string
	VCSIssueLinkingBlockReason       func(tool, command, repo string) string
	IssueEditTargetFromCommand       func(tool, command, repo string) (string, bool)
	PullRequestBranchInfoFromCommand func(tool, command, repo string) (remoteartifactcontract.PullRequestBranchInfo, bool)
	StagedCheckDecision              func(tool, repo, command string) (string, string)
)
