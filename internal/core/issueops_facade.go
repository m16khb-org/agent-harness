package core

import (
	"context"
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/artifacttemplate"
	"agent-harness/internal/core/looprun"
	"agent-harness/internal/core/workpool"
)

type IssueOpsStartRequest = issueops.IssueOpsStartRequest
type IssueOpsFeedbackItem = issueops.IssueOpsFeedbackItem
type IssueOpsIssueLink = issueops.IssueOpsIssueLink
type IssueOpsBranchPrepareStep = issueops.IssueOpsBranchPrepareStep
type IssueOpsBranchPrepare = issueops.IssueOpsBranchPrepare
type IssueOpsBranchPrepareRequest = issueops.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = issueops.IssueOpsRemoteArtifactVerification
type IssueOpsRemoteArtifactVerificationRequest = issueops.IssueOpsRemoteArtifactVerificationRequest
type IssueOpsIntentContract = issueops.IssueOpsIntentContract
type IssueOpsIntentRecordRequest = issueops.IssueOpsIntentRecordRequest
type IssueOpsDesignReview = issueops.IssueOpsDesignReview
type IssueOpsDesignReviewRequest = issueops.IssueOpsDesignReviewRequest
type IssueOpsDecision = issueops.IssueOpsDecision
type IssueOpsDecisionRecordRequest = issueops.IssueOpsDecisionRecordRequest
type IssueOpsExecutionDecision = issueops.IssueOpsExecutionDecision
type IssueOpsExecutionDecisionRecordRequest = issueops.IssueOpsExecutionDecisionRecordRequest
type IssueOpsSubAgentPlan = issueops.IssueOpsSubAgentPlan
type IssueOpsCompatibilityReview = issueops.IssueOpsCompatibilityReview
type IssueOpsCompatibilityReviewRequest = issueops.IssueOpsCompatibilityReviewRequest
type IssueOpsDevilsAdvocateReviewRequest = issueops.IssueOpsDevilsAdvocateReviewRequest
type IssueOpsDomainReview = issueops.IssueOpsDomainReview
type IssueOpsDomainReviewRequest = issueops.IssueOpsDomainReviewRequest
type IssueOpsPlanPrepRequest = issueops.IssueOpsPlanPrepRequest
type IssueOpsPlanPrepItemRequest = issueops.IssueOpsPlanPrepItemRequest
type IssueOpsWorktreeToolPreparation = issueops.IssueOpsWorktreeToolPreparation
type IssueOpsActor = issueops.IssueOpsActor
type IssueOpsHandoffPrepareRequest = issueops.IssueOpsHandoffPrepareRequest
type IssueOpsHandoffPrepareResult = issueops.IssueOpsHandoffPrepareResult
type IssueOpsHandoffPrepareClock = issueops.IssueOpsHandoffPrepareClock
type IssueOpsOrcaWorktreeClient = issueops.IssueOpsOrcaWorktreeClient
type IssueOpsLegacyWorktreeMigration = issueops.IssueOpsLegacyWorktreeMigration
type IssueOpsLegacyWorktreeMigrationRequest = issueops.IssueOpsLegacyWorktreeMigrationRequest
type IssueOpsLegacyWorktreeMigrationResult = issueops.IssueOpsLegacyWorktreeMigrationResult
type IssueOpsHandoffStartRequest = issueops.IssueOpsHandoffStartRequest
type IssueOpsHandoffStartResult = issueops.IssueOpsHandoffStartResult
type IssueOpsHandoffStartClock = issueops.IssueOpsHandoffStartClock
type IssueOpsOrcaDispatchClient = issueops.IssueOpsOrcaDispatchClient
type IssueOpsWorkerDoneProjectionClient = issueops.IssueOpsWorkerDoneProjectionClient
type IssueOpsHandoffClaimRequest = issueops.IssueOpsHandoffClaimRequest
type IssueOpsHandoffAcknowledgeRequest = issueops.IssueOpsHandoffAcknowledgeRequest
type IssueOpsHeartbeatRequest = issueops.IssueOpsHeartbeatRequest
type IssueOpsHandoffFinishRequest = issueops.IssueOpsHandoffFinishRequest
type IssueOpsHandoffAcceptRequest = issueops.IssueOpsHandoffAcceptRequest
type IssueOpsHandoffRecoverRequest = issueops.IssueOpsHandoffRecoverRequest
type IssueOpsHandoffRecoverResult = issueops.IssueOpsHandoffRecoverResult
type IssueOpsOwnershipCleanupPreviewRequest = issueops.IssueOpsOwnershipCleanupPreviewRequest
type IssueOpsOwnershipCleanupPreview = issueops.IssueOpsOwnershipCleanupPreview
type IssueOpsOwnershipCleanupApproveRequest = issueops.IssueOpsOwnershipCleanupApproveRequest
type IssueOpsOwnershipCleanupRecordRequest = issueops.IssueOpsOwnershipCleanupRecordRequest
type IssueOpsHandoffPublishRequest = issueops.IssueOpsHandoffPublishRequest
type IssueOpsHandoffPublicationReader = issueops.IssueOpsHandoffPublicationReader
type GitIssueOpsHandoffPublicationReader = issueops.GitIssueOpsHandoffPublicationReader
type IssueOpsPublicationPushTarget = issueops.IssueOpsPublicationPushTarget
type IssueOpsExecutionHandoffPublishReceipt = issueops.IssueOpsExecutionHandoffPublishReceipt
type IssueOpsExecutionHandoffCleanupReceipt = issueops.IssueOpsExecutionHandoffCleanupReceipt
type IssueOpsExecutionHandoffCleanup = issueops.IssueOpsExecutionHandoffCleanup
type IssueOpsRemoteCreateClaim = issueops.IssueOpsRemoteCreateClaim
type IssueOpsRecord = issueops.IssueOpsRecord
type IssueOpsDelegationContract = issueops.IssueOpsDelegationContract
type IssueOpsChildCycleRef = issueops.IssueOpsChildCycleRef
type IssueOpsChildStartRequest = issueops.IssueOpsChildStartRequest
type IssueOpsChildStartResult = issueops.IssueOpsChildStartResult
type IssueOpsChildStatusEntry = issueops.IssueOpsChildStatusEntry
type IssueOpsChildStatusResult = issueops.IssueOpsChildStatusResult
type IssueOpsChildValidationResult = issueops.IssueOpsChildValidationResult
type IssueOpsReadiness = issueops.IssueOpsReadiness
type IssueOpsCleanupStatusRequest = issueops.IssueOpsCleanupStatusRequest
type IssueOpsCleanupStatus = issueops.IssueOpsCleanupStatus
type IssueOpsCloseChildrenRequest = issueops.IssueOpsCloseChildrenRequest
type IssueOpsCloseChildResult = issueops.IssueOpsCloseChildResult
type IssueOpsCloseChildrenResult = issueops.IssueOpsCloseChildrenResult
type IssueOpsStaleScanRequest = issueops.IssueOpsStaleScanRequest
type IssueOpsStaleScanResult = issueops.IssueOpsStaleScanResult

type IssueOpsPhase = issueops.IssueOpsPhase

const (
	IssueOpsCurrentSchemaVersion     = issueops.IssueOpsCurrentSchemaVersion
	IssueOpsPhaseProblem             = issueops.IssueOpsPhaseProblem
	IssueOpsPhaseGrill               = issueops.IssueOpsPhaseGrill
	IssueOpsPhasePlan                = issueops.IssueOpsPhasePlan
	IssueOpsPhaseCompatibilityReview = issueops.IssueOpsPhaseCompatibilityReview
	IssueOpsPhaseImplement           = issueops.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean         = issueops.IssueOpsPhaseAISlopClean
	IssueOpsPhaseFeedback            = issueops.IssueOpsPhaseFeedback
	IssueOpsPhasePR                  = issueops.IssueOpsPhasePR
	IssueOpsPhaseDone                = issueops.IssueOpsPhaseDone
)

var IssueOpsPhases = issueops.IssueOpsPhases

const IssueOpsDesignReviewEvidenceExample = issueops.IssueOpsDesignReviewEvidenceExample

type IssueOpsBenchmarkFixture = issueops.IssueOpsBenchmarkFixture
type IssueOpsBenchmarkArtifact = issueops.IssueOpsBenchmarkArtifact
type SkillRouting = issueops.SkillRouting
type IssueOpsDimensionScore = issueops.IssueOpsDimensionScore
type IssueOpsBenchmarkScore = issueops.IssueOpsBenchmarkScore
type IssueOpsBenchmarkRunRequest = issueops.IssueOpsBenchmarkRunRequest
type IssueOpsBenchmarkRunResult = issueops.IssueOpsBenchmarkRunResult
type IssueOpsBenchmarkCompareResult = issueops.IssueOpsBenchmarkCompareResult
type IssueOpsAutoresearchCandidate = issueops.IssueOpsAutoresearchCandidate
type IssueOpsAutoresearchGateRequest = issueops.IssueOpsAutoresearchGateRequest
type IssueOpsAutoresearchGateResult = issueops.IssueOpsAutoresearchGateResult
type IssueOpsLLMJudgeRequest = issueops.IssueOpsLLMJudgeRequest
type RecordedRun = issueops.RecordedRun
type RecordedOutcomes = issueops.RecordedOutcomes
type FixtureReliability = issueops.FixtureReliability
type PassPowKPoint = issueops.PassPowKPoint
type ReliabilityReport = issueops.ReliabilityReport
type IssueOpsJudgeMap = issueops.IssueOpsJudgeMap
type JudgeSample = issueops.JudgeSample
type ConsensusVerdict = issueops.ConsensusVerdict

type IssueOpsRemoteArtifact = issueops.IssueOpsRemoteArtifact
type IssueOpsRemoteIssueCandidate = issueops.IssueOpsRemoteIssueCandidate
type IssueOpsRemoteLabelCandidate = issueops.IssueOpsRemoteLabelCandidate
type IssueOpsRemoteScoringRequest = issueops.IssueOpsRemoteScoringRequest
type IssueOpsRemoteScoredItem = issueops.IssueOpsRemoteScoredItem
type IssueOpsRemoteScoringResult = issueops.IssueOpsRemoteScoringResult
type IssueOpsRemoteLLMJudgeRequest = issueops.IssueOpsRemoteLLMJudgeRequest
type IssueOpsArtifactKind = artifacttemplate.IssueOpsArtifactKind
type IssueOpsTemplateKind = artifacttemplate.IssueOpsTemplateKind
type IssueOpsTemplateInput = artifacttemplate.IssueOpsTemplateInput
type IssueOpsTemplateResult = artifacttemplate.IssueOpsTemplateResult
type IssueOpsTemplateValidation = artifacttemplate.IssueOpsTemplateValidation

const (
	IssueOpsArtifactIssue = artifacttemplate.IssueOpsArtifactIssue
	IssueOpsArtifactChild = artifacttemplate.IssueOpsArtifactChild
	IssueOpsArtifactPR    = artifacttemplate.IssueOpsArtifactPR

	IssueOpsTemplateBug                = artifacttemplate.IssueOpsTemplateBug
	IssueOpsTemplateFeature            = artifacttemplate.IssueOpsTemplateFeature
	IssueOpsTemplateProposal           = artifacttemplate.IssueOpsTemplateProposal
	IssueOpsTemplateImplementationTask = artifacttemplate.IssueOpsTemplateImplementationTask
	IssueOpsTemplateChildTask          = artifacttemplate.IssueOpsTemplateChildTask
	IssueOpsTemplatePullRequest        = artifacttemplate.IssueOpsTemplatePullRequest
)

func RenderIssueOpsTemplate(input IssueOpsTemplateInput) IssueOpsTemplateResult {
	return artifacttemplate.Render(input)
}

func ValidateIssueOpsTemplate(input IssueOpsTemplateInput) IssueOpsTemplateValidation {
	return artifacttemplate.Validate(input)
}

func ParseIssueOpsTemplateFields(values []string) (map[string]string, error) {
	return artifacttemplate.ParseFieldAssignments(values)
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	return issueops.StartIssueOps(stateRoot, req)
}

func StartIssueOpsChild(stateRoot string, req IssueOpsChildStartRequest) (IssueOpsChildStartResult, error) {
	return issueops.StartIssueOpsChild(stateRoot, req)
}

func StartIssueOpsChildWithActor(stateRoot string, req IssueOpsChildStartRequest, actor IssueOpsActor) (IssueOpsChildStartResult, error) {
	return issueops.StartIssueOpsChildWithActor(stateRoot, req, actor)
}

func IssueOpsChildStatus(stateRoot, parentID string, repair bool) (IssueOpsChildStatusResult, error) {
	return issueops.IssueOpsChildStatus(stateRoot, parentID, repair)
}

func IssueOpsChildStatusWithActor(stateRoot, parentID string, repair bool, actor IssueOpsActor) (IssueOpsChildStatusResult, error) {
	return issueops.IssueOpsChildStatusWithActor(stateRoot, parentID, repair, actor)
}

func AcceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string) (IssueOpsChildValidationResult, error) {
	return issueops.AcceptIssueOpsChild(stateRoot, parentID, childID, evidence)
}

func AcceptIssueOpsChildWithActor(stateRoot, parentID, childID string, evidence []string, actor IssueOpsActor) (IssueOpsChildValidationResult, error) {
	return issueops.AcceptIssueOpsChildWithActor(stateRoot, parentID, childID, evidence, actor)
}

func RejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string) (IssueOpsChildValidationResult, error) {
	return issueops.RejectIssueOpsChild(stateRoot, parentID, childID, reason, evidence)
}

func RejectIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, evidence []string, actor IssueOpsActor) (IssueOpsChildValidationResult, error) {
	return issueops.RejectIssueOpsChildWithActor(stateRoot, parentID, childID, reason, evidence, actor)
}

func DropIssueOpsChild(stateRoot, parentID, childID, reason string) (IssueOpsChildValidationResult, error) {
	return issueops.DropIssueOpsChild(stateRoot, parentID, childID, reason)
}

func DropIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, actor IssueOpsActor) (IssueOpsChildValidationResult, error) {
	return issueops.DropIssueOpsChildWithActor(stateRoot, parentID, childID, reason, actor)
}

func IssueOpsStatus(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.IssueOpsStatus(stateRoot, id)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ReadIssueOps(stateRoot, id)
}

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsIntent(stateRoot, id, req)
}

func RecordIssueOpsIntentWithActor(stateRoot, id string, req IssueOpsIntentRecordRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsIntentWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsPlanPrep(stateRoot, id string, req IssueOpsPlanPrepRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsPlanPrep(stateRoot, id, req)
}

func RecordIssueOpsPlanPrepWithActor(stateRoot, id string, req IssueOpsPlanPrepRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsPlanPrepWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDesignReview(stateRoot, id, req)
}

func RecordIssueOpsDesignReviewWithActor(stateRoot, id string, req IssueOpsDesignReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDesignReviewWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsExecutionDecision(stateRoot, id string, req IssueOpsExecutionDecisionRecordRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsExecutionDecision(stateRoot, id, req)
}

func RecordIssueOpsExecutionDecisionWithActor(stateRoot, id string, req IssueOpsExecutionDecisionRecordRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsExecutionDecisionWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req IssueOpsCompatibilityReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsCompatibilityReview(stateRoot, id, req)
}

func RecordIssueOpsCompatibilityReviewWithActor(stateRoot, id string, req IssueOpsCompatibilityReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsCompatibilityReviewWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDevilsAdvocateReview(stateRoot, id, req)
}

func RecordIssueOpsDevilsAdvocateReviewWithActor(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDevilsAdvocateReviewWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsDomainReview(stateRoot, id string, req IssueOpsDomainReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDomainReview(stateRoot, id, req)
}

func RecordIssueOpsDomainReviewWithActor(stateRoot, id string, req IssueOpsDomainReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDomainReviewWithActor(stateRoot, id, req, actor)
}

func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification)
}

func RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, id string, categories, verification []string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, id, categories, verification, actor)
}

func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (IssueOpsRecord, error) {
	return issueops.ResolveIssueOpsFeedback(stateRoot, id, index, resolution)
}

func ResolveIssueOpsFeedbackWithActor(stateRoot, id string, index int, resolution string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.ResolveIssueOpsFeedbackWithActor(stateRoot, id, index, resolution, actor)
}

func RegressIssueOpsForReplan(stateRoot, id, reason string) (IssueOpsRecord, error) {
	return issueops.RegressIssueOpsForReplan(stateRoot, id, reason)
}

func RegressIssueOpsForReplanWithActor(stateRoot, id, reason string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RegressIssueOpsForReplanWithActor(stateRoot, id, reason, actor)
}

func IssueOpsStateRoot() string {
	return issueops.IssueOpsStateRoot()
}

func NewIssueOpsID(repo, branch string) string {
	return issueops.NewIssueOpsID(repo, branch)
}

func newIssueOpsID(repo, branch string) string {
	return issueops.NewIssueOpsID(repo, branch)
}

func WriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return issueops.WriteIssueOps(stateRoot, record)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return issueops.WriteIssueOps(stateRoot, record)
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsIssue(stateRoot, id, issueURL)
}

func LinkIssueOpsIssueWithActor(stateRoot, id, issueURL string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsIssueWithActor(stateRoot, id, issueURL, actor)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsPlan(stateRoot, id, planPath)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsPlanWithActor(stateRoot, id, planPath, actor)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsWorktree(stateRoot, id, worktreePath)
}

func LinkIssueOpsWorktreeWithActor(stateRoot, id, worktreePath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsWorktreeWithActor(stateRoot, id, worktreePath, actor)
}

func RecordIssueOpsWorktreeTools(stateRoot, id string, prep IssueOpsWorktreeToolPreparation) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsWorktreeTools(stateRoot, id, prep)
}

func RecordIssueOpsWorktreeToolsWithActor(stateRoot, id string, actor IssueOpsActor, prep IssueOpsWorktreeToolPreparation) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsWorktreeToolsWithActor(stateRoot, id, actor, prep)
}

func PrepareIssueOpsHandoffWorktree(ctx context.Context, stateRoot string, req IssueOpsHandoffPrepareRequest, client IssueOpsOrcaWorktreeClient, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffPrepareResult, error) {
	return issueops.PrepareIssueOpsHandoffWorktree(ctx, stateRoot, req, client, clock)
}

type IssueOpsExecutionWorkspaceReconcileRequest = issueops.IssueOpsExecutionWorkspaceReconcileRequest
type IssueOpsExecutionWorkspaceRecoveryClient = issueops.IssueOpsExecutionWorkspaceRecoveryClient

func ReconcileIssueOpsExecutionWorkspace(ctx context.Context, stateRoot string, req IssueOpsExecutionWorkspaceReconcileRequest, client IssueOpsExecutionWorkspaceRecoveryClient, now string) (IssueOpsRecord, error) {
	return issueops.ReconcileIssueOpsExecutionWorkspace(ctx, stateRoot, req, client, now)
}

func ValidateReadyWorkspacePreparationActor(record IssueOpsRecord, actor IssueOpsActor) error {
	return issueops.ValidateReadyWorkspacePreparationActor(record, actor)
}

func MigrateIssueOpsLegacyWorktree(ctx context.Context, stateRoot string, req IssueOpsLegacyWorktreeMigrationRequest, client IssueOpsOrcaWorktreeClient, clock IssueOpsHandoffPrepareClock) (IssueOpsLegacyWorktreeMigrationResult, error) {
	return issueops.MigrateIssueOpsLegacyWorktree(ctx, stateRoot, req, client, clock)
}

func StartIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffStartRequest, client IssueOpsOrcaDispatchClient, clock IssueOpsHandoffStartClock) (IssueOpsHandoffStartResult, error) {
	return issueops.StartIssueOpsHandoff(ctx, stateRoot, req, client, clock)
}

func ClaimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest) (IssueOpsRecord, error) {
	return issueops.ClaimIssueOpsHandoff(stateRoot, req)
}

func AcknowledgeIssueOpsHandoffContext(stateRoot string, req IssueOpsHandoffAcknowledgeRequest) (IssueOpsRecord, error) {
	return issueops.AcknowledgeIssueOpsHandoffContext(stateRoot, req)
}

func FinishIssueOpsHandoffWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffFinishRequest, client IssueOpsWorkerDoneProjectionClient) (IssueOpsRecord, error) {
	return issueops.FinishIssueOpsHandoffWithProjection(ctx, stateRoot, req, client)
}

func CompleteIssueOpsOwnershipTransferWithProjection(ctx context.Context, stateRoot string, req IssueOpsHandoffFinishRequest, client IssueOpsWorkerDoneProjectionClient) (IssueOpsRecord, error) {
	return issueops.CompleteIssueOpsOwnershipTransferWithProjection(ctx, stateRoot, req, client)
}

func PreviewIssueOpsOwnershipCleanup(stateRoot string, req IssueOpsOwnershipCleanupPreviewRequest) (IssueOpsOwnershipCleanupPreview, error) {
	return issueops.PreviewIssueOpsOwnershipCleanup(stateRoot, req)
}

func ApproveIssueOpsOwnershipCleanup(stateRoot string, req IssueOpsOwnershipCleanupApproveRequest) (IssueOpsRecord, error) {
	return issueops.ApproveIssueOpsOwnershipCleanup(stateRoot, req)
}

func RecordIssueOpsOwnershipCleanup(ctx context.Context, stateRoot string, req IssueOpsOwnershipCleanupRecordRequest, client any) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsOwnershipCleanup(ctx, stateRoot, req, client)
}

func AcceptIssueOpsHandoff(stateRoot string, req IssueOpsHandoffAcceptRequest) (IssueOpsRecord, error) {
	return issueops.AcceptIssueOpsHandoff(stateRoot, req)
}

func RecoverIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffRecoverRequest, client any, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffRecoverResult, error) {
	return issueops.RecoverIssueOpsHandoff(ctx, stateRoot, req, client, clock)
}

func RecordIssueOpsHandoffPublishReceipt(ctx context.Context, stateRoot string, req IssueOpsHandoffPublishRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, clock IssueOpsHandoffPrepareClock) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsHandoffPublishReceipt(ctx, stateRoot, req, reader, lease, clock)
}

func ValidateIssueOpsHandoffPublication(ctx context.Context, stateRoot string, record IssueOpsRecord, provider, head, base string, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient) error {
	return issueops.ValidateIssueOpsHandoffPublication(ctx, stateRoot, record, provider, head, base, reader, lease)
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsChild(stateRoot, id, childURL, title)
}

func LinkIssueOpsChildWithActor(stateRoot, id, childURL, title string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsChildWithActor(stateRoot, id, childURL, title, actor)
}

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title)
}

func LinkIssueOpsRelatedWithActor(stateRoot, id, linkType, relatedURL, title string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsRelatedWithActor(stateRoot, id, linkType, relatedURL, title, actor)
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	return issueops.PrepareIssueOpsBranch(stateRoot, id, req)
}

func PrepareIssueOpsBranchWithActor(stateRoot, id string, req IssueOpsBranchPrepareRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.PrepareIssueOpsBranchWithActor(stateRoot, id, req, actor)
}

func validateIssueOpsIssueBranch(branch string) error {
	return issueops.ValidateIssueOpsIssueBranch(branch)
}

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsFeedback(stateRoot, id, source, body, classification)
}

func AddIssueOpsFeedbackWithActor(stateRoot, id, source, body, classification string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsFeedbackWithActor(stateRoot, id, source, body, classification, actor)
}

func MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id)
}

func MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, id string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, id, actor)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	if IssueOpsPhase(strings.TrimSpace(to)) == IssueOpsPhasePR {
		record, err := issueops.ReadIssueOps(stateRoot, id)
		if err != nil {
			return record, err
		}
		if record.Phase != IssueOpsPhasePR {
			if ready := IssueOpsStrictPRReadiness(record); !ready.Ready {
				return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
			}
		}
	}
	return issueops.AdvanceIssueOpsPhase(stateRoot, id, to)
}

func AdvanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor IssueOpsActor) (IssueOpsRecord, error) {
	if IssueOpsPhase(strings.TrimSpace(to)) == IssueOpsPhasePR {
		record, err := issueops.ReadIssueOps(stateRoot, id)
		if err != nil {
			return record, err
		}
		if record.Phase != IssueOpsPhasePR {
			if ready := IssueOpsStrictPRReadiness(record); !ready.Ready {
				return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
			}
		}
	}
	return issueops.AdvanceIssueOpsPhaseWithActor(stateRoot, id, to, actor)
}

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return issueops.VerifyIssueOpsRemoteArtifact(stateRoot, id, req)
}

var ClaimIssueOpsRemoteCreate = issueops.ClaimIssueOpsRemoteCreate

type IssueOpsRemoteCreateClaimRequest = issueops.IssueOpsRemoteCreateClaimRequest

var ClearIssueOpsRemoteCreateClaimPreInvocation = issueops.ClearIssueOpsRemoteCreateClaimPreInvocation
var MarkIssueOpsRemoteCreateUnknown = issueops.MarkIssueOpsRemoteCreateUnknown
var FinalizeIssueOpsRemoteCreateClaim = issueops.FinalizeIssueOpsRemoteCreateClaim

type IssueOpsRemotePullRequestCreateFunc = issueops.IssueOpsRemotePullRequestCreateFunc
type IssueOpsRemoteCreateReconcileRequest = issueops.IssueOpsRemoteCreateReconcileRequest
type IssueOpsRemoteCreateCandidate = issueops.IssueOpsRemoteCreateCandidate
type IssueOpsRemoteCreateProbeResult = issueops.IssueOpsRemoteCreateProbeResult
type IssueOpsRemoteCreateProbe = issueops.IssueOpsRemoteCreateProbe

func CreateIssueOpsRemotePullRequest(ctx context.Context, stateRoot, id, provider string, request IssueProviderCreatePullRequestRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, create IssueOpsRemotePullRequestCreateFunc) (IssueProviderCreatePullRequestResult, error) {
	return issueops.CreateIssueOpsRemotePullRequest(ctx, stateRoot, id, provider, request, reader, lease, create)
}

func ReconcileIssueOpsRemoteCreate(ctx context.Context, stateRoot string, req IssueOpsRemoteCreateReconcileRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, probe IssueOpsRemoteCreateProbe) (IssueOpsRecord, error) {
	return issueops.ReconcileIssueOpsRemoteCreate(ctx, stateRoot, req, reader, lease, probe)
}

var ProjectIssueOpsRemoteCreateProbeResult = issueops.ProjectIssueOpsRemoteCreateProbeResult

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return issueops.ValidateIssueOpsRemoteArtifactVerification(stateRoot, id, req)
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsPRReadiness(record)
}

func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithLoopGate(issueOpsStrictPRReadinessWithPoolGate(issueops.IssueOpsStrictPRReadiness(record), record.ID), record.Repo)
}

func IssueOpsStrictPRReadinessWithState(stateRoot string, record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithLoopGate(issueOpsStrictPRReadinessWithPoolGate(issueops.IssueOpsStrictPRReadinessWithState(stateRoot, record), record.ID), record.Repo)
}

func issueOpsStrictPRReadinessWithPoolGate(ready IssueOpsReadiness, parentID string) IssueOpsReadiness {
	missing, warnings := workpool.ParentGateMissing(parentID)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	ready.Missing = uniqSorted(append(append([]string{}, ready.Missing...), missing...))
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func issueOpsStrictPRReadinessWithLoopGate(ready IssueOpsReadiness, repo string) IssueOpsReadiness {
	missing, warnings := looprun.RepoGateMissing(repo)
	if len(missing) == 0 && len(warnings) == 0 {
		return ready
	}
	ready.Missing = uniqSorted(append(append([]string{}, ready.Missing...), missing...))
	ready.Warnings = append(ready.Warnings, warnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func IssueOpsImplementationReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsImplementationReadiness(record)
}

func IssueOpsCompatibilityReviewReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsCompatibilityReviewReadiness(record)
}

func IssueOpsPlanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsPlanReadiness(record)
}

func IssueOpsAISlopCleanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsAISlopCleanReadiness(record)
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	return issueops.IssueOpsPhaseExpectsWorktree(phase)
}

func IssueOpsCleanupStatusByID(stateRoot, id string, req IssueOpsCleanupStatusRequest) (IssueOpsCleanupStatus, error) {
	return issueops.IssueOpsCleanupStatusByID(stateRoot, id, req)
}

func IssueOpsCleanupStatusForRecord(record IssueOpsRecord, req IssueOpsCleanupStatusRequest) IssueOpsCleanupStatus {
	return issueops.IssueOpsCleanupStatusForRecord(record, req)
}

func CloseIssueOpsChildren(stateRoot, id string, req IssueOpsCloseChildrenRequest, provider func(string) (IssueProvider, error)) (IssueOpsCloseChildrenResult, error) {
	return issueops.CloseIssueOpsChildren(stateRoot, id, req, provider)
}

func ForceReleaseIssueOps(stateRoot, id, reason string) (IssueOpsRecord, error) {
	return issueops.ForceReleaseIssueOps(stateRoot, id, reason)
}

type ForceReleaseCASRequest = issueops.ForceReleaseCASRequest
type ForceReleaseCASResult = issueops.ForceReleaseCASResult

func ForceReleaseIssueOpsCAS(stateRoot, id, reason string, req ForceReleaseCASRequest) (ForceReleaseCASResult, error) {
	return issueops.ForceReleaseIssueOpsCAS(stateRoot, id, reason, req)
}

func ScanStaleIssueOpsCycles(req IssueOpsStaleScanRequest) IssueOpsStaleScanResult {
	return issueops.ScanStaleIssueOpsCycles(req)
}

func ForceDoneIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ForceDoneIssueOps(stateRoot, id)
}

func AddIssueOpsDecision(stateRoot, id string, req IssueOpsDecisionRecordRequest) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsDecision(stateRoot, id, req)
}

func AddIssueOpsDecisionWithActor(stateRoot, id string, req IssueOpsDecisionRecordRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsDecisionWithActor(stateRoot, id, req, actor)
}

type SkillRoutingEntry = issueops.SkillRoutingEntry

// RecordIssueOpsRouting captures a live (phase, skill) routing pairing on the
// cycle record so skill_routing_fidelity can score real activation.
func RecordIssueOpsRouting(stateRoot, id, phase, skill string) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsRouting(stateRoot, id, phase, skill)
}

func RecordIssueOpsRoutingWithActor(stateRoot, id, phase, skill string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsRoutingWithActor(stateRoot, id, phase, skill, actor)
}

// RoutingTraceAsSkillRouting projects a cycle's live routing trace onto the
// benchmark SkillRouting shape for scoring against a real run.
func RoutingTraceAsSkillRouting(record IssueOpsRecord) []SkillRouting {
	return issueops.RoutingTraceAsSkillRouting(record)
}

type RoutingFidelityResult = issueops.RoutingFidelityResult

// ScoreLiveRoutingFidelity scores a cycle's recorded live routing trace against
// expected (phase, skill) pairings, replacing the benchmark's synthesized trace
// with real observed activation.
func ScoreLiveRoutingFidelity(record IssueOpsRecord, expected []SkillRouting) RoutingFidelityResult {
	return issueops.ScoreLiveRoutingFidelity(record, expected)
}

// AutoRecordSkillRouting best-effort records a skill activation against the
// active session-bound cycle for repo (no-op when there is none). It is
// fail-open and safe to call from non-blocking hook paths.
func AutoRecordSkillRouting(repo, skill string) bool {
	return issueops.AutoRecordSkillRouting(repo, skill)
}

func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	return issueops.ActiveIssueOpsCycleForBranch(repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCycleForRepo(repo string) (IssueOpsRecord, bool) {
	return issueops.ActiveIssueOpsLinkedWorktreeCycleForRepo(repo)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	return issueops.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func LoadIssueOpsBenchmarkFixtures(dir string) ([]IssueOpsBenchmarkFixture, error) {
	return issueops.LoadIssueOpsBenchmarkFixtures(dir)
}

func ComputeReliability(rec RecordedOutcomes, alpha float64) (ReliabilityReport, error) {
	return issueops.ComputeReliability(rec, alpha)
}

func ScoreSpread(scores []float64) (float64, float64, float64) {
	return issueops.ScoreSpread(scores)
}

func ValidateJudgeProvenance(judge IssueOpsJudgeMap, scoredRunID, stateRoot string) error {
	return issueops.ValidateJudgeProvenance(judge, scoredRunID, stateRoot)
}

func JudgeDownwardOverrideRate(deterministic, judge IssueOpsBenchmarkScore) (float64, int) {
	return issueops.JudgeDownwardOverrideRate(deterministic, judge)
}

func ConsensusJudgeVerdict(samples []JudgeSample) (ConsensusVerdict, error) {
	return issueops.ConsensusJudgeVerdict(samples)
}

func ScoreIssueOpsBenchmarkArtifact(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) IssueOpsBenchmarkScore {
	return issueops.ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
}

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	return issueops.RunIssueOpsBenchmark(req)
}

func SaveIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	return issueops.SaveIssueOpsBenchmarkRun(stateRoot, result)
}

func ReadIssueOpsBenchmarkRun(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
	return issueops.ReadIssueOpsBenchmarkRun(stateRoot, id)
}

func FinalizeIssueOpsBenchmarkRunResult(result IssueOpsBenchmarkRunResult) IssueOpsBenchmarkRunResult {
	return issueops.FinalizeIssueOpsBenchmarkRunResult(result)
}

func MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge IssueOpsBenchmarkScore) IssueOpsBenchmarkScore {
	return issueops.MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge)
}

func CompareIssueOpsBenchmarkRuns(baseline, candidate IssueOpsBenchmarkRunResult) IssueOpsBenchmarkCompareResult {
	return issueops.CompareIssueOpsBenchmarkRuns(baseline, candidate)
}

func EvaluateIssueOpsAutoresearchGate(req IssueOpsAutoresearchGateRequest) IssueOpsAutoresearchGateResult {
	return issueops.EvaluateIssueOpsAutoresearchGate(req)
}

func RunIssueOpsLLMJudge(req IssueOpsLLMJudgeRequest) (IssueOpsBenchmarkScore, error) {
	return issueops.RunIssueOpsLLMJudge(req)
}

func DecodeIssueOpsBenchmarkJudgeJSON(out []byte) (IssueOpsBenchmarkScore, error) {
	return issueops.DecodeIssueOpsBenchmarkJudgeJSON(out)
}

func RenderIssueOpsLLMJudgePrompt(req IssueOpsLLMJudgeRequest) (string, error) {
	return issueops.RenderIssueOpsLLMJudgePrompt(req)
}

func buildIssueOpsLLMJudgePrompt(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) (string, error) {
	return issueops.BuildIssueOpsLLMJudgePrompt(fixture, artifact)
}

func DecodeIssueOpsRemoteScoringRequest(data []byte) (IssueOpsRemoteScoringRequest, error) {
	return issueops.DecodeIssueOpsRemoteScoringRequest(data)
}

func ScoreIssueOpsRemoteCandidates(req IssueOpsRemoteScoringRequest) (IssueOpsRemoteScoringResult, error) {
	return issueops.ScoreIssueOpsRemoteCandidates(req)
}

func RunIssueOpsRemoteLLMJudge(req IssueOpsRemoteLLMJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	return issueops.RunIssueOpsRemoteLLMJudge(req)
}

func RenderIssueOpsRemoteLLMJudgePrompt(req IssueOpsRemoteLLMJudgeRequest) (string, error) {
	return issueops.RenderIssueOpsRemoteLLMJudgePrompt(req)
}

func DecodeIssueOpsRemoteJudgeJSON(out []byte) (IssueOpsRemoteScoringResult, error) {
	return issueops.DecodeIssueOpsRemoteJudgeJSON(out)
}

// Session binding for multi-session continuity.
type SessionBinding = issueops.SessionBinding

func BindIssueOpsSession(repo, cycleID, branch, expectedWorktree string) error {
	return issueops.BindIssueOpsSession(repo, cycleID, branch, expectedWorktree)
}

func BindScopedIssueOpsSession(repo, cycleID, branch, expectedWorktree string) error {
	return issueops.BindScopedIssueOpsSession(repo, cycleID, branch, expectedWorktree)
}

func BindIssueOpsSessionForCycle(repo, cycleID string) error {
	return issueops.BindIssueOpsSessionForCycle(repo, cycleID)
}

func ReadIssueOpsSession(repo string) (SessionBinding, error) {
	return issueops.ReadIssueOpsSession(repo)
}

func ReadScopedIssueOpsSession(repo, cycleID string) (SessionBinding, error) {
	return issueops.ReadScopedIssueOpsSession(repo, cycleID)
}

func UnbindIssueOpsSession(repo string) error {
	return issueops.UnbindIssueOpsSession(repo)
}

func UnbindScopedIssueOpsSessionForCycle(repo, cycleID string) error {
	return issueops.UnbindScopedIssueOpsSessionForCycle(repo, cycleID)
}

func ListIssueOpsSessionBindings(repo string) ([]SessionBinding, error) {
	return issueops.ListIssueOpsSessionBindings(repo)
}

func ActiveSessionCycleID(repo string) string {
	return issueops.ActiveSessionCycleID(repo)
}

func ExpectedWorktreeFromSession(repo string, cycleWorktree func() string) string {
	return issueops.ExpectedWorktreeFromSession(repo, cycleWorktree)
}

func ExpectedWorktreeEnvGuidance(worktree string) string {
	return issueops.ExpectedWorktreeEnvGuidance(worktree)
}

// IssueOpsResume reads the session-to-cycle binding for repo and returns a
// resume result with cycle details, readiness, or suggested cycles.
type IssueOpsResumeResult = issueops.IssueOpsResumeResult

func IssueOpsResume(repo string, ids ...string) IssueOpsResumeResult {
	return issueops.IssueOpsResume(repo, ids...)
}

func RecordIssueOpsHeartbeat(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsHeartbeat(stateRoot, id)
}

func RecordIssueOpsHeartbeatWithRequest(stateRoot string, req IssueOpsHeartbeatRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsHeartbeatWithRequest(stateRoot, req)
}

func IssueOpsLastActiveAt(record IssueOpsRecord) string {
	return issueops.LastActiveAt(record)
}
