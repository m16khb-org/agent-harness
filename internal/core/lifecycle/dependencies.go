package lifecycle

import (
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/nextaction"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
	"agent-harness/internal/core/state"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectProfile = projectdocs.ProjectProfile
type NextActionJudgementTriggerResult = nextaction.NextActionJudgementTriggerResult
type NumberedNextActionsDecisionResult = nextaction.NumberedNextActionsDecisionResult
type NextActionCandidate = nextaction.NextActionCandidate
type NextActionAutoProceedResult = nextaction.NextActionAutoProceedResult
type NextActionAutoProceedLLMRequest = nextaction.NextActionAutoProceedLLMRequest

type IssueOpsRecord = issueops.IssueOpsRecord
type IssueOpsStartRequest = issueops.IssueOpsStartRequest
type IssueOpsIntentRecordRequest = issueops.IssueOpsIntentRecordRequest
type IssueOpsDesignReviewRequest = issueops.IssueOpsDesignReviewRequest
type IssueOpsBranchPrepareRequest = issueops.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = issueops.IssueOpsRemoteArtifactVerification
type IssueOpsPhase = issueops.IssueOpsPhase

const (
	IssueOpsPhaseProblem     = issueops.IssueOpsPhaseProblem
	IssueOpsPhaseGrill       = issueops.IssueOpsPhaseGrill
	IssueOpsPhasePlan        = issueops.IssueOpsPhasePlan
	IssueOpsPhaseImplement   = issueops.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean = issueops.IssueOpsPhaseAISlopClean
	IssueOpsPhasePR          = issueops.IssueOpsPhasePR
	IssueOpsPhaseDone        = issueops.IssueOpsPhaseDone
)

func StateDir() string {
	return state.StateDir()
}

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	return nextaction.BuildNumberedNextActionsDecision(message, enforce, source)
}

func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	return nextaction.BuildNextActionJudgementTrigger(message)
}

func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	return nextaction.EvaluateNextActionAutoProceed(message, threshold)
}

func EvaluateNextActionAutoProceedLLM(req NextActionAutoProceedLLMRequest, threshold float64) (NextActionAutoProceedResult, error) {
	return nextaction.EvaluateNextActionAutoProceedLLM(req, threshold)
}

func IssueOpsStateRoot() string {
	return issueops.IssueOpsStateRoot()
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	return issueops.StartIssueOps(stateRoot, req)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ReadIssueOps(stateRoot, id)
}

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsIntent(stateRoot, id, req)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDesignReview(stateRoot, id, req)
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsIssue(stateRoot, id, issueURL)
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	return issueops.PrepareIssueOpsBranch(stateRoot, id, req)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsWorktree(stateRoot, id, worktreePath)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsPlan(stateRoot, id, planPath)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	return issueops.AdvanceIssueOpsPhase(stateRoot, id, to)
}

func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	return issueops.ActiveIssueOpsCycleForBranch(repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	return issueops.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	return issueops.IssueOpsPhaseExpectsWorktree(phase)
}

func validateIssueOpsIssueBranch(branch string) error {
	return issueops.ValidateIssueOpsIssueBranch(branch)
}

func newIssueOpsID(repo, branch string) string {
	return issueops.NewIssueOpsID(repo, branch)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return issueops.WriteIssueOps(stateRoot, record)
}
