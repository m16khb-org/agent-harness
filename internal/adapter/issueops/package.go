package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/issueops/active"
	"agent-harness/internal/adapter/issueops/artifactverify"
	"agent-harness/internal/adapter/issueops/branchprepare"
	"agent-harness/internal/adapter/issueops/cleanupchildren"
	"agent-harness/internal/adapter/issueops/cleanupstatus"
	"agent-harness/internal/adapter/issueops/compatibilityreview"
	"agent-harness/internal/adapter/issueops/devilsadvocate"
	"agent-harness/internal/adapter/issueops/intentdesign"
	"agent-harness/internal/adapter/issueops/linking"
	"agent-harness/internal/adapter/issueops/start"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/stringlist"
	"agent-harness/internal/port"
)

const (
	IssueOpsCurrentSchemaVersion     = issueops.IssueOpsCurrentSchemaVersion
	IssueOpsPhaseCompatibilityReview = issueops.IssueOpsPhaseCompatibilityReview
	IssueOpsPhaseFeedback            = issueops.IssueOpsPhaseFeedback
)

var IssueOpsPhases = issueops.IssueOpsPhases

const IssueOpsDesignReviewEvidenceExample = intentdesign.DesignReviewEvidenceExample

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest) (issueops.IssueOpsRecord, error) {
	return verifyIssueOpsRemoteArtifact(stateRoot, id, req, nil)
}

func VerifyIssueOpsRemoteArtifactWithActor(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return verifyIssueOpsRemoteArtifact(stateRoot, id, req, &actor)
}

func verifyIssueOpsRemoteArtifact(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func ActiveIssueOpsCycleForBranch(repo, branch string) (issueops.IssueOpsRecord, bool) {
	return active.CycleForBranch(issueOpsActiveStore(), repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCycleForRepo(repo string) (issueops.IssueOpsRecord, bool) {
	return active.LinkedWorktreeCycleForRepo(issueOpsActiveStore(), repo)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []issueops.IssueOpsRecord {
	return active.LinkedWorktreeCyclesForRepo(issueOpsActiveStore(), repo)
}

// IssueOpsCycleWorktreeMissing reports whether a record is a worktree-phase
// cycle whose linked worktree directory has been deleted (a stale cycle that
// must not retain guard authority over the source checkout).
func IssueOpsCycleWorktreeMissing(record issueops.IssueOpsRecord) bool {
	return active.WorktreePhaseHasMissingWorktree(record)
}

func issueOpsActiveStore() active.Store {
	return active.Store{
		StateRoot: IssueOpsStateRoot,
		// Hooks must still see a corrupt v1 record so they fail closed instead of
		// silently dropping the execution guard. Command paths use ReadIssueOps,
		// which validates the record before operating on it.
		Read:    readIssueOpsUnchecked,
		Scan:    ScanReadableIssueOps,
		NewID:   newIssueOpsID,
		ListIDs: ListIssueOpsIDs,
	}
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req issueops.IssueOpsCleanupStatusRequest) (issueops.IssueOpsCleanupStatus, error) {
	return cleanupstatus.ByID(issueOpsCleanupStatusStore(), stateRoot, id, req)
}

func IssueOpsCleanupStatusForRecord(record issueops.IssueOpsRecord, req issueops.IssueOpsCleanupStatusRequest) issueops.IssueOpsCleanupStatus {
	return cleanupstatus.ForRecord(record, req)
}

func FinalizeIssueOpsCleanupStatus(status issueops.IssueOpsCleanupStatus) issueops.IssueOpsCleanupStatus {
	return cleanupstatus.Finalize(status)
}

func IssueOpsRemoteArtifactMissing(record issueops.IssueOpsRecord) []string {
	return cleanupstatus.RemoteArtifactMissing(record)
}

func CloseIssueOpsChildren(stateRoot, id string, req issueops.IssueOpsCloseChildrenRequest, provider func(string) (port.IssueProvider, error)) (issueops.IssueOpsCloseChildrenResult, error) {
	var result issueops.IssueOpsCloseChildrenResult
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

func issueOpsRemoteArtifactMissing(record issueops.IssueOpsRecord) []string {
	return cleanupstatus.RemoteArtifactMissing(record)
}

func issueOpsCleanupStatusStore() cleanupstatus.Store {
	return cleanupstatus.Store{
		Read: ReadIssueOps,
	}
}

func PrepareIssueOpsBranch(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest) (issueops.IssueOpsRecord, error) {
	return prepareIssueOpsBranch(stateRoot, id, req, nil)
}

func PrepareIssueOpsBranchWithActor(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return prepareIssueOpsBranch(stateRoot, id, req, &actor)
}

func prepareIssueOpsBranch(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func issueOpsBranchPrepareStore() branchprepare.Store {
	return branchprepare.Store{
		Read:             ReadIssueOps,
		TouchWrite:       touchAndWriteIssueOps,
		ValidateIssueURL: linking.ValidateIssueURL,
		ResolveBaseCommit: func(repo, revision string) (string, error) {
			code, stdout, stderr := GitCmd(
				repo,
				"rev-parse",
				"--verify",
				"--end-of-options",
				strings.TrimSpace(revision)+"^{commit}",
			)
			if code != 0 {
				return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(stderr))
			}
			resolved := strings.TrimSpace(stdout)
			if resolved == "" {
				return "", fmt.Errorf("git rev-parse returned an empty commit OID")
			}
			return resolved, nil
		},
		UmbrellaForChildIssue: func(repo, childIssueURL string) (issueops.IssueOpsRecord, bool) {
			return active.UmbrellaCycleForChildIssue(issueOpsActiveStore(), repo, childIssueURL)
		},
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

func StartIssueOps(stateRoot string, req issueops.IssueOpsStartRequest) (issueops.IssueOpsRecord, error) {
	id := issueOpsStartLockID(req.Repo, req.Branch)
	var rec issueops.IssueOpsRecord
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

func RecordIssueOpsIntent(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsIntent(stateRoot, id, req, nil)
}

func RecordIssueOpsIntentWithActor(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsIntent(stateRoot, id, req, &actor)
}

func recordIssueOpsIntent(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func RecordIssueOpsPlanPrep(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, nil)
}

func RecordIssueOpsPlanPrepWithActor(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, &actor)
}

func recordIssueOpsPlanPrep(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func RecordIssueOpsDesignReview(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDesignReviewWithActor(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDesignReview(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsIssue(stateRoot, id, issueURL, nil)
}

func LinkIssueOpsIssueWithActor(stateRoot, id, issueURL string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsIssue(stateRoot, id, issueURL, &actor)
}

func linkIssueOpsIssue(stateRoot, id, issueURL string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func LinkIssueOpsPlan(stateRoot, id, planPath string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, nil)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, &actor)
}

func linkIssueOpsPlan(stateRoot, id, planPath string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validatePlanLinkMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var writeErr error
		rec, writeErr = linking.LinkPlan(issueOpsLinkingStore(), stateRoot, id, planPath)
		return writeErr
	})
	return rec, err
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, nil)
}

func LinkIssueOpsWorktreeWithActor(stateRoot, id, worktreePath string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, &actor)
}

func linkIssueOpsWorktree(stateRoot, id, worktreePath string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, nil)
}

func RecordIssueOpsCompatibilityReviewWithActor(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, &actor)
}

func recordIssueOpsCompatibilityReview(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDevilsAdvocateReviewWithActor(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDevilsAdvocateReview(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, nil)
}

func LinkIssueOpsChildWithActor(stateRoot, id, childURL, title string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, &actor)
}

func linkIssueOpsChild(stateRoot, id, childURL, title string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title, nil)
}

func LinkIssueOpsRelatedWithActor(stateRoot, id, linkType, relatedURL, title string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title, &actor)
}

func linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
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
func LastActiveAt(record issueops.IssueOpsRecord) string {
	if strings.TrimSpace(record.UpdatedAt) != "" {
		return record.UpdatedAt
	}
	return record.CreatedAt
}
