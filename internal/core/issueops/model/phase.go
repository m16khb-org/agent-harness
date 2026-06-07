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
