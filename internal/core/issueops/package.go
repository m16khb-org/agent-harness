package issueops

import "agent-harness/internal/core/issueops/model"

type IssueOpsStartRequest = model.IssueOpsStartRequest
type IssueOpsFeedbackItem = model.IssueOpsFeedbackItem
type IssueOpsIssueLink = model.IssueOpsIssueLink
type IssueOpsBranchPrepareStep = model.IssueOpsBranchPrepareStep
type IssueOpsBranchPrepare = model.IssueOpsBranchPrepare
type IssueOpsBranchPrepareRequest = model.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = model.IssueOpsRemoteArtifactVerification
type IssueOpsRemoteArtifactVerificationRequest = model.IssueOpsRemoteArtifactVerificationRequest
type IssueOpsIntentContract = model.IssueOpsIntentContract
type IssueOpsIntentRecordRequest = model.IssueOpsIntentRecordRequest
type IssueOpsDesignReview = model.IssueOpsDesignReview
type IssueOpsDesignReviewRequest = model.IssueOpsDesignReviewRequest
type IssueOpsRecord = model.IssueOpsRecord
type IssueOpsReadiness = model.IssueOpsReadiness
type IssueOpsCleanupStatusRequest = model.IssueOpsCleanupStatusRequest
type IssueOpsCleanupStatus = model.IssueOpsCleanupStatus
type IssueOpsPhase = model.IssueOpsPhase

const (
	IssueOpsPhaseProblem     = model.IssueOpsPhaseProblem
	IssueOpsPhaseGrill       = model.IssueOpsPhaseGrill
	IssueOpsPhasePlan        = model.IssueOpsPhasePlan
	IssueOpsPhaseImplement   = model.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean = model.IssueOpsPhaseAISlopClean
	IssueOpsPhaseFeedback    = model.IssueOpsPhaseFeedback
	IssueOpsPhasePR          = model.IssueOpsPhasePR
	IssueOpsPhaseDone        = model.IssueOpsPhaseDone
)

var IssueOpsPhases = model.IssueOpsPhases
