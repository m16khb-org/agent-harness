package issueops

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/branchprepare"
	"agent-harness/internal/core/issueops/cleanupchildren"
	"agent-harness/internal/core/issueops/cleanupstatus"
	"agent-harness/internal/core/issueops/compatibilityreview"
	"agent-harness/internal/core/issueops/devilsadvocate"
	"agent-harness/internal/core/issueops/intentdesign"
	"agent-harness/internal/core/issueops/linking"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/start"
	"agent-harness/internal/core/issueops/stringlist"
	"agent-harness/internal/port"
	"context"
)

type IssueOpsStartRequest = model.IssueOpsStartRequest
type IssueOpsFeedbackItem = model.IssueOpsFeedbackItem
type SkillRoutingEntry = model.SkillRoutingEntry
type IssueOpsIssueLink = model.IssueOpsIssueLink
type IssueOpsBranchPrepareStep = model.IssueOpsBranchPrepareStep
type IssueOpsBranchPrepare = model.IssueOpsBranchPrepare
type IssueOpsBranchPrepareRequest = model.IssueOpsBranchPrepareRequest
type Execution = model.Execution
type Workspace = model.Workspace
type WriteLease = model.WriteLease
type NativeActor = model.NativeActor
type NativeProcessReceipt = model.NativeProcessReceipt
type IssueOpsRemoteArtifactVerification = model.IssueOpsRemoteArtifactVerification
type IssueOpsRemoteArtifactVerificationRequest = model.IssueOpsRemoteArtifactVerificationRequest
type IssueOpsIntentContract = model.IssueOpsIntentContract
type IssueOpsIntentRecordRequest = model.IssueOpsIntentRecordRequest
type IssueOpsDesignReview = model.IssueOpsDesignReview
type IssueOpsDesignReviewRequest = model.IssueOpsDesignReviewRequest
type IssueOpsDecision = model.IssueOpsDecision
type IssueOpsDecisionRecordRequest = model.IssueOpsDecisionRecordRequest
type IssueOpsCompatibilityReview = model.IssueOpsCompatibilityReview
type IssueOpsDevilsAdvocateReview = model.IssueOpsDevilsAdvocateReview
type IssueOpsCompatibilityReviewRequest = model.IssueOpsCompatibilityReviewRequest
type IssueOpsDevilsAdvocateReviewRequest = model.IssueOpsDevilsAdvocateReviewRequest
type IssueOpsPlanPrep = model.IssueOpsPlanPrep
type IssueOpsPlanPrepItem = model.IssueOpsPlanPrepItem
type IssueOpsPlanPrepRequest = model.IssueOpsPlanPrepRequest
type IssueOpsPlanPrepItemRequest = model.IssueOpsPlanPrepItemRequest
type IssueOpsRecord = model.IssueOpsRecord
type IssueOpsRemoteCompletion = model.IssueOpsRemoteCompletion
type IssueOpsCleanupFinishFailure = model.IssueOpsCleanupFinishFailure
type IssueOpsImplementationReview = model.IssueOpsImplementationReview
type IssueOpsRegressEvent = model.IssueOpsRegressEvent
type IssueOpsDelegationContract = model.IssueOpsDelegationContract
type IssueOpsChildCycleRef = model.IssueOpsChildCycleRef
type IssueOpsChildStartRequest = model.IssueOpsChildStartRequest
type IssueOpsChildStartResult = model.IssueOpsChildStartResult
type IssueOpsChildStatusEntry = model.IssueOpsChildStatusEntry
type IssueOpsChildStatusResult = model.IssueOpsChildStatusResult
type IssueOpsChildValidationResult = model.IssueOpsChildValidationResult
type IssueOpsReadiness = model.IssueOpsReadiness
type IssueOpsDomainReview = model.IssueOpsDomainReview
type IssueOpsDomainReviewRequest = model.IssueOpsDomainReviewRequest
type IssueOpsPhaseLedger = model.IssueOpsPhaseLedger
type IssueOpsPhaseLedgerEntry = model.IssueOpsPhaseLedgerEntry
type IssueOpsCleanupStatusRequest = model.IssueOpsCleanupStatusRequest
type IssueOpsCleanupStatus = model.IssueOpsCleanupStatus
type IssueOpsCloseChildrenRequest = model.IssueOpsCloseChildrenRequest
type IssueOpsCloseChildResult = model.IssueOpsCloseChildResult
type IssueOpsCloseChildrenResult = model.IssueOpsCloseChildrenResult
type IssueOpsPhase = model.IssueOpsPhase

const (
	IssueOpsCurrentSchemaVersion     = model.IssueOpsCurrentSchemaVersion
	IssueOpsPhaseProblem             = model.IssueOpsPhaseProblem
	IssueOpsPhaseGrill               = model.IssueOpsPhaseGrill
	IssueOpsPhasePlan                = model.IssueOpsPhasePlan
	IssueOpsPhaseCompatibilityReview = model.IssueOpsPhaseCompatibilityReview
	IssueOpsPhaseImplement           = model.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean         = model.IssueOpsPhaseAISlopClean
	IssueOpsPhaseFeedback            = model.IssueOpsPhaseFeedback
	IssueOpsPhasePR                  = model.IssueOpsPhasePR
	IssueOpsPhaseDone                = model.IssueOpsPhaseDone
)

var IssueOpsPhases = model.IssueOpsPhases

const IssueOpsDesignReviewEvidenceExample = intentdesign.DesignReviewEvidenceExample

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return verifyIssueOpsRemoteArtifact(stateRoot, id, req, nil)
}

func VerifyIssueOpsRemoteArtifactWithActor(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return verifyIssueOpsRemoteArtifact(stateRoot, id, req, &actor)
}

func verifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validatePostTransferMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = artifactverify.Verify(issueOpsArtifactStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = artifactverify.Validate(issueOpsArtifactStore(), stateRoot, id, req)
		return e
	})
	return rec, err
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

// IssueOpsCycleWorktreeMissing reports whether a record is a worktree-phase
// cycle whose linked worktree directory has been deleted (a stale cycle that
// must not retain guard authority over the source checkout).
func IssueOpsCycleWorktreeMissing(record IssueOpsRecord) bool {
	return active.WorktreePhaseHasMissingWorktree(record)
}

func issueOpsActiveStore() active.Store {
	return active.Store{
		StateRoot: IssueOpsStateRoot,
		// Hooks must still see a corrupt v1 record so they fail closed instead of
		// silently dropping the execution guard. Command paths use ReadIssueOps,
		// which validates the record before operating on it.
		Read:    readIssueOpsUnchecked,
		NewID:   newIssueOpsID,
		ListIDs: ListIssueOpsIDs,
	}
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req IssueOpsCleanupStatusRequest) (IssueOpsCleanupStatus, error) {
	return cleanupstatus.ByID(issueOpsCleanupStatusStore(), stateRoot, id, req)
}

func IssueOpsCleanupStatusForRecord(record IssueOpsRecord, req IssueOpsCleanupStatusRequest) IssueOpsCleanupStatus {
	return cleanupstatus.ForRecord(record, req)
}

func CloseIssueOpsChildren(stateRoot, id string, req IssueOpsCloseChildrenRequest, provider func(string) (port.IssueProvider, error)) (IssueOpsCloseChildrenResult, error) {
	var result IssueOpsCloseChildrenResult
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		result, e = cleanupchildren.ByID(cleanupchildren.Store{
			Read:       ReadIssueOps,
			TouchWrite: touchAndWriteIssueOps,
			Provider:   provider,
		}, stateRoot, id, req)
		return e
	})
	return result, err
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
	return prepareIssueOpsBranch(stateRoot, id, req, nil)
}

func PrepareIssueOpsBranchWithActor(stateRoot, id string, req IssueOpsBranchPrepareRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return prepareIssueOpsBranch(stateRoot, id, req, &actor)
}

func prepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = branchprepare.Prepare(issueOpsBranchPrepareStore(), stateRoot, id, req)
		return e
	})
	return rec, err
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

// issueOpsStartLockID computes the lock id used by StartIssueOps. It must
// mirror start.Start's record-id derivation exactly: trim repo+branch and
// abs-normalize the repo (filepath.Abs) before hashing, so that a relative and
// the equivalent absolute repo path take the SAME lock and serialize on the
// SAME record. newIssueOpsID does no abs-normalization, so hashing the raw repo
// here would let concurrent relative/absolute starts hold different locks while
// read-modify-writing one record (lost-update TOCTOU).
func issueOpsStartLockID(repo, branch string) string {
	repo = strings.TrimSpace(repo)
	if abs, err := filepath.Abs(repo); err == nil {
		repo = abs
	}
	return newIssueOpsID(repo, strings.TrimSpace(branch))
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	id := issueOpsStartLockID(req.Repo, req.Branch)
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = start.Start(issueOpsStartStore(), stateRoot, req)
		return e
	})
	return rec, err
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
	return recordIssueOpsIntent(stateRoot, id, req, nil)
}

func RecordIssueOpsIntentWithActor(stateRoot, id string, req IssueOpsIntentRecordRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsIntent(stateRoot, id, req, &actor)
}

func recordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordIntent(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsPlanPrep(stateRoot, id string, req IssueOpsPlanPrepRequest) (IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, nil)
}

func RecordIssueOpsPlanPrepWithActor(stateRoot, id string, req IssueOpsPlanPrepRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, &actor)
}

func recordIssueOpsPlanPrep(stateRoot, id string, req IssueOpsPlanPrepRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordPlanPrep(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDesignReviewWithActor(stateRoot, id string, req IssueOpsDesignReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordDesignReview(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
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
	return linkIssueOpsIssue(stateRoot, id, issueURL, nil)
}

func LinkIssueOpsIssueWithActor(stateRoot, id, issueURL string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsIssue(stateRoot, id, issueURL, &actor)
}

func linkIssueOpsIssue(stateRoot, id, issueURL string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkIssue(issueOpsLinkingStore(), stateRoot, id, issueURL)
		return e
	})
	return rec, err
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, nil)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, &actor)
}

func linkIssueOpsPlan(stateRoot, id, planPath string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateExecutionMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var writeErr error
		rec, writeErr = linking.LinkPlan(issueOpsLinkingStore(), stateRoot, id, planPath)
		return writeErr
	})
	return rec, err
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, nil)
}

func LinkIssueOpsWorktreeWithActor(stateRoot, id, worktreePath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, &actor)
}

func linkIssueOpsWorktree(stateRoot, id, worktreePath string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkWorktree(issueOpsLinkingStore(), stateRoot, id, worktreePath)
		return e
	})
	return rec, err
}

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req IssueOpsCompatibilityReviewRequest) (IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, nil)
}

func RecordIssueOpsCompatibilityReviewWithActor(stateRoot, id string, req IssueOpsCompatibilityReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, &actor)
}

func recordIssueOpsCompatibilityReview(stateRoot, id string, req IssueOpsCompatibilityReviewRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = compatibilityreview.Record(issueOpsCompatibilityReviewStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func issueOpsCompatibilityReviewStore() compatibilityreview.Store {
	return compatibilityreview.Store{
		Read:       ReadIssueOps,
		TouchWrite: touchAndWriteIssueOps,
		Ready:      IssueOpsCompatibilityReviewReadiness,
		PhaseRank:  issueOpsPhaseRank,
	}
}

func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest) (IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDevilsAdvocateReviewWithActor(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDevilsAdvocateReview(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = devilsadvocate.Record(devilsadvocate.Store{Read: ReadIssueOps, TouchWrite: touchAndWriteIssueOps}, stateRoot, id, req)
		return e
	})
	return rec, err
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, nil)
}

func LinkIssueOpsChildWithActor(stateRoot, id, childURL, title string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, &actor)
}

func linkIssueOpsChild(stateRoot, id, childURL, title string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkChild(issueOpsLinkingStore(), stateRoot, id, childURL, title)
		return e
	})
	return rec, err
}

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (IssueOpsRecord, error) {
	return linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title, nil)
}

func LinkIssueOpsRelatedWithActor(stateRoot, id, linkType, relatedURL, title string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title, &actor)
}

func linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkRelated(issueOpsLinkingStore(), stateRoot, id, linkType, relatedURL, title)
		return e
	})
	return rec, err
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

// LastActiveAt returns the latest durable lifecycle timestamp.
func LastActiveAt(record IssueOpsRecord) string {
	if strings.TrimSpace(record.UpdatedAt) != "" {
		return record.UpdatedAt
	}
	return record.CreatedAt
}
