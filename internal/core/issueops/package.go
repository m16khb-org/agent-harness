package issueops

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/active"
	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/branchprepare"
	"agent-harness/internal/core/issueops/cleanupchildren"
	"agent-harness/internal/core/issueops/cleanupstatus"
	"agent-harness/internal/core/issueops/compatibilityreview"
	"agent-harness/internal/core/issueops/devilsadvocate"
	"agent-harness/internal/core/issueops/executiondecision"
	"agent-harness/internal/core/issueops/intentdesign"
	"agent-harness/internal/core/issueops/linking"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/issueops/session"
	"agent-harness/internal/core/issueops/start"
	"agent-harness/internal/core/issueops/stringlist"
	"agent-harness/internal/port"
)

type IssueOpsStartRequest = model.IssueOpsStartRequest
type IssueOpsFeedbackItem = model.IssueOpsFeedbackItem
type SkillRoutingEntry = model.SkillRoutingEntry
type IssueOpsIssueLink = model.IssueOpsIssueLink
type IssueOpsBranchPrepareStep = model.IssueOpsBranchPrepareStep
type IssueOpsBranchPrepare = model.IssueOpsBranchPrepare
type IssueOpsBranchPrepareRequest = model.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = model.IssueOpsRemoteArtifactVerification
type IssueOpsRemoteArtifactVerificationRequest = model.IssueOpsRemoteArtifactVerificationRequest
type IssueOpsRemoteCreateClaim = model.IssueOpsRemoteCreateClaim
type IssueOpsIntentContract = model.IssueOpsIntentContract
type IssueOpsIntentRecordRequest = model.IssueOpsIntentRecordRequest
type IssueOpsDesignReview = model.IssueOpsDesignReview
type IssueOpsDesignReviewRequest = model.IssueOpsDesignReviewRequest
type IssueOpsDecision = model.IssueOpsDecision
type IssueOpsDecisionRecordRequest = model.IssueOpsDecisionRecordRequest
type IssueOpsExecutionDecision = model.IssueOpsExecutionDecision
type IssueOpsExecutionDecisionRecordRequest = model.IssueOpsExecutionDecisionRecordRequest
type IssueOpsSubAgentPlan = model.IssueOpsSubAgentPlan
type IssueOpsCompatibilityReview = model.IssueOpsCompatibilityReview
type IssueOpsDevilsAdvocateReview = model.IssueOpsDevilsAdvocateReview
type IssueOpsCompatibilityReviewRequest = model.IssueOpsCompatibilityReviewRequest
type IssueOpsDevilsAdvocateReviewRequest = model.IssueOpsDevilsAdvocateReviewRequest
type IssueOpsPlanPrep = model.IssueOpsPlanPrep
type IssueOpsPlanPrepItem = model.IssueOpsPlanPrepItem
type IssueOpsPlanPrepRequest = model.IssueOpsPlanPrepRequest
type IssueOpsPlanPrepItemRequest = model.IssueOpsPlanPrepItemRequest
type IssueOpsWorktreeToolPreparation = model.IssueOpsWorktreeToolPreparation
type IssueOpsRecord = model.IssueOpsRecord
type IssueOpsRegressEvent = model.IssueOpsRegressEvent
type IssueOpsDelegationContract = model.IssueOpsDelegationContract
type IssueOpsChildCycleRef = model.IssueOpsChildCycleRef
type IssueOpsChildStartRequest = model.IssueOpsChildStartRequest
type IssueOpsChildStartResult = model.IssueOpsChildStartResult
type IssueOpsChildStatusEntry = model.IssueOpsChildStatusEntry
type IssueOpsChildStatusResult = model.IssueOpsChildStatusResult
type IssueOpsChildValidationResult = model.IssueOpsChildValidationResult
type IssueOpsHostSessionIdentity = model.IssueOpsHostSessionIdentity
type IssueOpsOrcaIdentity = model.IssueOpsOrcaIdentity
type IssueOpsExecutionHandoffPendingOperation = model.IssueOpsExecutionHandoffPendingOperation
type IssueOpsExecutionHandoffResult = model.IssueOpsExecutionHandoffResult
type IssueOpsExecutionHandoffFailure = model.IssueOpsExecutionHandoffFailure
type IssueOpsExecutionHandoffCancellation = model.IssueOpsExecutionHandoffCancellation
type IssueOpsExecutionHandoffPublishReceipt = model.IssueOpsExecutionHandoffPublishReceipt
type IssueOpsExecutionHandoffCleanupReceipt = model.IssueOpsExecutionHandoffCleanupReceipt
type IssueOpsExecutionHandoffCleanup = model.IssueOpsExecutionHandoffCleanup
type IssueOpsOrcaCleanupArtifact = model.IssueOpsOrcaCleanupArtifact
type IssueOpsExecutionHandoffPriorAttempt = model.IssueOpsExecutionHandoffPriorAttempt
type IssueOpsExecutionHandoff = model.IssueOpsExecutionHandoff
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
type IssueOpsResumeResult = model.IssueOpsResumeResult
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
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = artifactverify.Verify(issueOpsArtifactStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
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

func ActiveIssueOpsSupervisedHandoffCyclesForRepo(repo string) []IssueOpsRecord {
	return active.SupervisedHandoffCyclesForRepo(issueOpsActiveStore(), repo)
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
		// Hooks must still see a corrupt handoff so they fail closed instead of
		// silently dropping the ownership guard. Command paths use ReadIssueOps,
		// which validates the envelope before operating on it.
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
	err := withIssueOpsLock(stateRoot, id, func() error {
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
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
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
	err := withIssueOpsLock(stateRoot, id, func() error {
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
		WorktreeValid:  issueOpsWorktreePathValid,
	}
}

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = intentdesign.RecordIntent(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsPlanPrep(stateRoot, id string, req IssueOpsPlanPrepRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = intentdesign.RecordPlanPrep(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
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
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = linking.LinkIssue(issueOpsLinkingStore(), stateRoot, id, issueURL)
		return e
	})
	return rec, err
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return linkIssueOpsPlanWithCoordinatorCheckpoint(stateRoot, id, planPath)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = linking.LinkWorktree(issueOpsLinkingStore(), stateRoot, id, worktreePath)
		return e
	})
	if err == nil {
		// Persist the session-to-cycle binding so hook guards can resolve the
		// expected worktree after session restarts (the read-side fallback
		// existed but nothing wrote the binding - ISSUEOPS_AUDIT 2.1/2.2).
		var bindErr error
		if rec.Delegation != nil {
			bindErr = BindScopedIssueOpsSession(rec.Repo, rec.ID, rec.Branch, worktreePath)
		} else {
			bindErr = BindIssueOpsSession(rec.Repo, rec.ID, rec.Branch, worktreePath)
		}
		if bindErr != nil {
			return rec, bindErr
		}
	}
	return rec, err
}

func RecordIssueOpsWorktreeTools(stateRoot, id string, prep IssueOpsWorktreeToolPreparation) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if strings.TrimSpace(record.WorktreePath) == "" {
			return fmt.Errorf("worktree_path is required")
		}
		if strings.TrimSpace(prep.WorktreePath) == "" {
			return fmt.Errorf("prepared worktree_path is required")
		}
		if strings.TrimSpace(prep.WorktreePath) != strings.TrimSpace(record.WorktreePath) {
			return fmt.Errorf("prepared worktree_path must match linked worktree_path")
		}
		prep.ID = record.ID
		prep.WorktreePath = strings.TrimSpace(prep.WorktreePath)
		if prep.PreparedAt == "" {
			prep.PreparedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		record.WorktreeTools = &prep
		if ready := IssueOpsImplementationReadiness(record); ready.Ready && issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhaseImplement) {
			record.Phase = IssueOpsPhaseImplement
		}
		var writeErr error
		rec, writeErr = touchAndWriteIssueOps(stateRoot, record)
		return writeErr
	})
	return rec, err
}

func RecordIssueOpsExecutionDecision(stateRoot, id string, req IssueOpsExecutionDecisionRecordRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = executiondecision.Record(issueOpsExecutionDecisionStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func issueOpsExecutionDecisionStore() executiondecision.Store {
	return executiondecision.Store{
		Read:       ReadIssueOps,
		TouchWrite: touchAndWriteIssueOps,
	}
}

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req IssueOpsCompatibilityReviewRequest) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
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
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = devilsadvocate.Record(devilsadvocate.Store{Read: ReadIssueOps, TouchWrite: touchAndWriteIssueOps}, stateRoot, id, req)
		return e
	})
	return rec, err
}

// unbindIssueOpsSessionForCycle clears the matching session binding scope only
// when it still points at the given cycle. Delegated children own scoped
// bindings; non-delegated cycles own the primary repo binding.
func unbindIssueOpsSessionForCycle(record IssueOpsRecord) {
	if record.Delegation != nil {
		_ = session.UnbindScopedForCycle(issueOpsSessionStore(), record.Repo, record.ID)
		return
	}
	_ = session.UnbindForCycle(issueOpsSessionStore(), record.Repo, record.ID)
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = linking.LinkChild(issueOpsLinkingStore(), stateRoot, id, childURL, title)
		return e
	})
	return rec, err
}

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
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

// SessionBinding exposes the session-to-cycle binding layer for multi-session
// continuity. See internal/core/issueops/session for semantics.
type SessionBinding = session.Binding

func BindIssueOpsSession(repo, cycleID, branch, expectedWorktree string) error {
	return session.Bind(issueOpsSessionStore(), repo, cycleID, branch, expectedWorktree)
}

func BindScopedIssueOpsSession(repo, cycleID, branch, expectedWorktree string) error {
	return session.BindScoped(issueOpsSessionStore(), repo, cycleID, branch, expectedWorktree)
}

func BindIssueOpsSessionForCycle(repo, cycleID string) error {
	rec, err := ReadIssueOps(IssueOpsStateRoot(), strings.TrimSpace(cycleID))
	if err != nil || !rec.OK {
		if err != nil {
			return err
		}
		return fmt.Errorf("issueops cycle not found: %s", strings.TrimSpace(cycleID))
	}
	bindRepo := strings.TrimSpace(repo)
	if bindRepo == "" {
		bindRepo = rec.Repo
	}
	expectedWorktree := strings.TrimSpace(rec.WorktreePath)
	if rec.Delegation != nil {
		if binding, err := ReadScopedIssueOpsSession(bindRepo, rec.ID); err == nil && strings.TrimSpace(binding.ExpectedWorktree) != "" {
			expectedWorktree = binding.ExpectedWorktree
		}
		return BindScopedIssueOpsSession(bindRepo, rec.ID, rec.Branch, expectedWorktree)
	}
	return BindIssueOpsSession(bindRepo, rec.ID, rec.Branch, expectedWorktree)
}

func ReadIssueOpsSession(repo string) (SessionBinding, error) {
	return session.Read(issueOpsSessionStore(), repo)
}

func ReadScopedIssueOpsSession(repo, cycleID string) (SessionBinding, error) {
	return session.ReadScoped(issueOpsSessionStore(), repo, cycleID)
}

func UnbindIssueOpsSession(repo string) error {
	return session.Unbind(issueOpsSessionStore(), repo)
}

func UnbindScopedIssueOpsSessionForCycle(repo, cycleID string) error {
	return session.UnbindScopedForCycle(issueOpsSessionStore(), repo, cycleID)
}

func ListIssueOpsSessionBindings(repo string) ([]SessionBinding, error) {
	return session.ListBindings(issueOpsSessionStore(), repo)
}

// ExpectedWorktreeFromSession returns the expected worktree for the current
// session, falling back to the cycle record's linked worktree.
func ExpectedWorktreeFromSession(repo string, cycleWorktree func() string) string {
	return session.ExpectedWorktree(issueOpsSessionStore(), repo, cycleWorktree)
}

func ExpectedWorktreeEnvGuidance(worktree string) string {
	worktree = strings.TrimSpace(worktree)
	if worktree == "" {
		return ""
	}
	return "export HARNESS_EXPECTED_WORKTREE=" + worktree
}

// ActiveSessionCycleID returns the cycle ID bound to the current session, or
// empty when unbound.
func ActiveSessionCycleID(repo string) string {
	return session.ActiveCycleID(issueOpsSessionStore(), repo)
}

func issueOpsSessionStore() session.Store {
	return session.Store{
		StateRoot: IssueOpsStateRoot,
	}
}

// IssueOpsResume reads the session-to-cycle binding for repo and returns a
// resume result. When a session is bound, it reads the cycle record and returns
// its details plus readiness. When unbound, it suggests active cycles for the
// repo (current branch first, then linked-worktree cycles).
func IssueOpsResume(repo string, ids ...string) IssueOpsResumeResult {
	repo = strings.TrimSpace(repo)
	if id := firstIssueOpsResumeID(ids); id != "" {
		return issueOpsResumeByID(repo, id)
	}
	b, err := ReadIssueOpsSession(repo)
	if err != nil {
		return IssueOpsResumeResult{OK: false}
	}
	if b.CycleID != "" {
		rec, err := ReadIssueOps(IssueOpsStateRoot(), b.CycleID)
		if err != nil || !rec.OK {
			return IssueOpsResumeResult{OK: false}
		}
		readiness := IssueOpsImplementationReadiness(rec)
		expectedWorktree := ExpectedWorktreeFromSession(repo, func() string { return rec.WorktreePath })
		return IssueOpsResumeResult{
			OK:               true,
			CycleID:          rec.ID,
			Phase:            rec.Phase,
			Repo:             rec.Repo,
			Branch:           rec.Branch,
			WorktreePath:     rec.WorktreePath,
			IssueURL:         rec.IssueURL,
			PlanPath:         rec.PlanPath,
			Bound:            true,
			Readiness:        &readiness,
			Guidance:         ExpectedWorktreeEnvGuidance(expectedWorktree),
			ExecutionHandoff: rec.ExecutionHandoff,
		}
	}

	// Not bound: suggest active cycles.
	branch := strings.TrimSpace(pathutil.GitBranchFromHead(repo))
	suggested := []string{}

	// First: check active cycle for the current branch.
	if branch != "" {
		if rec, ok := ActiveIssueOpsCycleForBranch(repo, branch); ok {
			suggested = append(suggested, rec.ID)
		}
	}

	// Second: add any linked-worktree cycles for the repo.
	for _, rec := range ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo) {
		suggested = appendUniqueIssueOpsCycleID(suggested, rec.ID)
	}

	if bindings, err := ListIssueOpsSessionBindings(repo); err == nil {
		for _, binding := range bindings {
			suggested = appendUniqueIssueOpsCycleID(suggested, binding.CycleID)
		}
	}

	return IssueOpsResumeResult{
		OK:              len(suggested) > 0,
		Bound:           false,
		SuggestedCycles: suggested,
	}
}

func firstIssueOpsResumeID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.TrimSpace(ids[0])
}

func issueOpsResumeByID(repo, id string) IssueOpsResumeResult {
	rec, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil || !rec.OK {
		return IssueOpsResumeResult{OK: false}
	}
	resumeRepo := strings.TrimSpace(repo)
	if resumeRepo == "" {
		resumeRepo = rec.Repo
	}
	readiness := IssueOpsImplementationReadiness(rec)
	expectedWorktree := strings.TrimSpace(rec.WorktreePath)
	if rec.Delegation != nil {
		if binding, err := ReadScopedIssueOpsSession(resumeRepo, rec.ID); err == nil && strings.TrimSpace(binding.ExpectedWorktree) != "" {
			expectedWorktree = binding.ExpectedWorktree
		}
	} else {
		expectedWorktree = ExpectedWorktreeFromSession(resumeRepo, func() string { return rec.WorktreePath })
	}
	return IssueOpsResumeResult{
		OK:               true,
		CycleID:          rec.ID,
		Phase:            rec.Phase,
		Repo:             rec.Repo,
		Branch:           rec.Branch,
		WorktreePath:     rec.WorktreePath,
		IssueURL:         rec.IssueURL,
		PlanPath:         rec.PlanPath,
		Bound:            true,
		Readiness:        &readiness,
		Guidance:         ExpectedWorktreeEnvGuidance(expectedWorktree),
		ExecutionHandoff: rec.ExecutionHandoff,
	}
}

func appendUniqueIssueOpsCycleID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

// LastActiveAt returns the best liveness timestamp: LastHeartbeatAt or UpdatedAt.
func LastActiveAt(record IssueOpsRecord) string {
	return IssueOpsLastActiveAt(record)
}
