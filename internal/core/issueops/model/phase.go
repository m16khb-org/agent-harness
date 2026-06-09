package model

type IssueOpsPhase string

const (
	IssueOpsPhaseProblem     IssueOpsPhase = "problem"
	IssueOpsPhaseGrill       IssueOpsPhase = "grill"
	IssueOpsPhasePlan        IssueOpsPhase = "plan"
	IssueOpsPhaseImplement   IssueOpsPhase = "implement"
	IssueOpsPhaseAISlopClean IssueOpsPhase = "ai-slop-clean"
	IssueOpsPhaseFeedback    IssueOpsPhase = "feedback"
	IssueOpsPhasePR          IssueOpsPhase = "pr"
	IssueOpsPhaseDone        IssueOpsPhase = "done"
)

var IssueOpsPhases = []IssueOpsPhase{
	IssueOpsPhaseProblem,
	IssueOpsPhaseGrill,
	IssueOpsPhasePlan,
	IssueOpsPhaseImplement,
	IssueOpsPhaseAISlopClean,
	IssueOpsPhaseFeedback,
	IssueOpsPhasePR,
	IssueOpsPhaseDone,
}

func KnownIssueOpsPhase(phase IssueOpsPhase) bool {
	for _, known := range IssueOpsPhases {
		if known == phase {
			return true
		}
	}
	return false
}

func IssueOpsPhaseRank(phase IssueOpsPhase) int {
	for i, known := range IssueOpsPhases {
		if known == phase {
			return i + 1
		}
	}
	return 0
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	switch phase {
	case IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback, IssueOpsPhasePR:
		return true
	default:
		return false
	}
}

// IssueOpsPhaseResettableOnStaleWorktree reports whether a cycle in this phase
// may be reset to a fresh problem-phase record when its isolated worktree was
// deleted. This is intentionally NARROWER than IssueOpsPhaseExpectsWorktree: the
// pr phase is excluded because its durable work product (the created issue/PR
// and remote artifact) lives remotely, not in the worktree — resetting it would
// silently destroy the remote linkage. A pr-phase cycle with a deleted worktree
// is resumed instead (the worktree-guard deadlock for it is handled separately
// via the liveness bypass, which still covers all worktree-expecting phases).
func IssueOpsPhaseResettableOnStaleWorktree(phase IssueOpsPhase) bool {
	switch phase {
	case IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback:
		return true
	default:
		return false
	}
}
