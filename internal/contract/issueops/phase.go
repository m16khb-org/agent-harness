package issueops

type IssueOpsPhase string

const (
	IssueOpsPhaseProblem             IssueOpsPhase = "problem"
	IssueOpsPhaseGrill               IssueOpsPhase = "grill"
	IssueOpsPhasePlan                IssueOpsPhase = "plan"
	IssueOpsPhaseCompatibilityReview IssueOpsPhase = "compatibility-review"
	IssueOpsPhaseImplement           IssueOpsPhase = "implement"
	IssueOpsPhaseAISlopClean         IssueOpsPhase = "ai-slop-clean"
	IssueOpsPhaseFeedback            IssueOpsPhase = "feedback"
	IssueOpsPhasePR                  IssueOpsPhase = "pr"
	IssueOpsPhaseDone                IssueOpsPhase = "done"
)

var IssueOpsPhases = []IssueOpsPhase{
	IssueOpsPhaseProblem,
	IssueOpsPhaseGrill,
	IssueOpsPhasePlan,
	IssueOpsPhaseCompatibilityReview,
	IssueOpsPhaseImplement,
	IssueOpsPhaseAISlopClean,
	IssueOpsPhaseFeedback,
	IssueOpsPhasePR,
	IssueOpsPhaseDone,
}
