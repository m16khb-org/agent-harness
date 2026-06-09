package issueops

import (
	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/branchprepare"
	"agent-harness/internal/core/issueops/cleanupstatus"
	"agent-harness/internal/core/issueops/intentdesign"
	"agent-harness/internal/core/issueops/linking"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/start"
	"agent-harness/internal/core/issueops/stringlist"
)

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
type IssueOpsDecision = model.IssueOpsDecision
type IssueOpsDecisionRecordRequest = model.IssueOpsDecisionRecordRequest
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

const IssueOpsDesignReviewEvidenceExample = intentdesign.DesignReviewEvidenceExample

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return artifactverify.Verify(issueOpsArtifactStore(), stateRoot, id, req)
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return artifactverify.Validate(issueOpsArtifactStore(), stateRoot, id, req)
}

func issueOpsArtifactStore() artifactverify.Store {
	return artifactverify.Store{
		Read:       ReadIssueOps,
		TouchWrite: touchAndWriteIssueOps,
	}
}

func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	return active.CycleForBranch(issueOpsActiveStore(), repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCycleForRepo(repo string) (IssueOpsRecord, bool) {
	return active.LinkedWorktreeCycleForRepo(issueOpsActiveStore(), repo)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	return active.LinkedWorktreeCyclesForRepo(issueOpsActiveStore(), repo)
}

func issueOpsActiveStore() active.Store {
	return active.Store{
		StateRoot: IssueOpsStateRoot,
		Read:      ReadIssueOps,
		NewID:     newIssueOpsID,
	}
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req IssueOpsCleanupStatusRequest) (IssueOpsCleanupStatus, error) {
	return cleanupstatus.ByID(issueOpsCleanupStatusStore(), stateRoot, id, req)
}

func IssueOpsCleanupStatusForRecord(record IssueOpsRecord, req IssueOpsCleanupStatusRequest) IssueOpsCleanupStatus {
	return cleanupstatus.ForRecord(record, req)
}

func issueOpsRemoteArtifactMissing(record IssueOpsRecord) []string {
	return cleanupstatus.RemoteArtifactMissing(record)
}

func issueOpsCleanupStatusStore() cleanupstatus.Store {
	return cleanupstatus.Store{
		Read: ReadIssueOps,
	}
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	return branchprepare.Prepare(issueOpsBranchPrepareStore(), stateRoot, id, req)
}

func ValidateIssueOpsIssueBranch(branch string) error {
	return validateIssueOpsIssueBranch(branch)
}

func validateIssueOpsIssueBranch(branch string) error {
	return branchprepare.ValidateBranch(branch)
}

func issueOpsBranchPrepareSteps(provider, issueURL, branch, baseBranch string) []IssueOpsBranchPrepareStep {
	return branchprepare.Steps(provider, issueURL, branch, baseBranch)
}

func issueOpsBranchPrepareStore() branchprepare.Store {
	return branchprepare.Store{
		Read:             ReadIssueOps,
		TouchWrite:       touchAndWriteIssueOps,
		ValidateIssueURL: linking.ValidateIssueURL,
	}
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	return start.Start(issueOpsStartStore(), stateRoot, req)
}

func issueOpsStartStore() start.Store {
	return start.Store{
		Read:           ReadIssueOps,
		Write:          writeIssueOps,
		NewID:          newIssueOpsID,
		ValidateBranch: validateIssueOpsIssueBranch,
	}
}

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	return intentdesign.RecordIntent(issueOpsIntentDesignStore(), stateRoot, id, req)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return intentdesign.RecordDesignReview(issueOpsIntentDesignStore(), stateRoot, id, req)
}

func cleanIssueOpsTextValues(values []string) []string {
	return intentdesign.CleanTextValues(values)
}

func issueOpsIntentDesignStore() intentdesign.Store {
	return intentdesign.Store{
		Read:          ReadIssueOps,
		TouchWrite:    touchAndWriteIssueOps,
		PlanReadiness: IssueOpsPlanReadiness,
	}
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	return linking.LinkIssue(issueOpsLinkingStore(), stateRoot, id, issueURL)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return linking.LinkPlan(issueOpsLinkingStore(), stateRoot, id, planPath)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return linking.LinkWorktree(issueOpsLinkingStore(), stateRoot, id, worktreePath)
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	return linking.LinkChild(issueOpsLinkingStore(), stateRoot, id, childURL, title)
}

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (IssueOpsRecord, error) {
	return linking.LinkRelated(issueOpsLinkingStore(), stateRoot, id, linkType, relatedURL, title)
}

func issueOpsLinkingStore() linking.Store {
	return linking.Store{
		Read:                   ReadIssueOps,
		TouchWrite:             touchAndWriteIssueOps,
		PlanReadiness:          IssueOpsPlanReadiness,
		PhaseRank:              issueOpsPhaseRank,
		BranchEvidenceMissing:  issueOpsBranchEvidenceMissing,
		DesignReviewMissing:    issueOpsDesignReviewMissing,
		PlanPathExists:         issueOpsPlanPathExists,
		PlanPathInsideWorktree: issueOpsPlanPathInsideWorktree,
		WorktreePathValid:      issueOpsWorktreePathValid,
		UniqueSorted:           stringlist.UniqueSorted,
	}
}
