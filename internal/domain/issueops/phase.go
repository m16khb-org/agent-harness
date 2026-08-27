package issueops

import issueopscontract "agent-harness/internal/contract/issueops"

func KnownIssueOpsPhase(phase issueopscontract.IssueOpsPhase) bool {
	return IssueOpsPhaseRank(phase) != 0
}

func IssueOpsPhaseRank(phase issueopscontract.IssueOpsPhase) int {
	for index, known := range issueopscontract.IssueOpsPhases {
		if phase == known {
			return index + 1
		}
	}
	return 0
}

// IssueOpsPhaseResettableOnStaleWorktree intentionally excludes the PR phase:
// its durable work product lives remotely and must be resumed, not reset.
func IssueOpsPhaseResettableOnStaleWorktree(phase issueopscontract.IssueOpsPhase) bool {
	switch phase {
	case issueopscontract.IssueOpsPhaseImplement, issueopscontract.IssueOpsPhaseAISlopClean, issueopscontract.IssueOpsPhaseFeedback:
		return true
	default:
		return false
	}
}
