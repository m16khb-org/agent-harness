package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/artifacttemplate"
	"agent-harness/internal/core/looprun"
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
type IssueOpsCompatibilityReview = issueops.IssueOpsCompatibilityReview
type IssueOpsCompatibilityReviewRequest = issueops.IssueOpsCompatibilityReviewRequest
type IssueOpsDevilsAdvocateReviewRequest = issueops.IssueOpsDevilsAdvocateReviewRequest
type IssueOpsDomainReview = issueops.IssueOpsDomainReview
type IssueOpsDomainReviewRequest = issueops.IssueOpsDomainReviewRequest
type IssueOpsPlanPrepRequest = issueops.IssueOpsPlanPrepRequest
type IssueOpsPlanPrepItemRequest = issueops.IssueOpsPlanPrepItemRequest
type IssueOpsActor = issueops.IssueOpsActor
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
type LegacyResetRowCounts = issueops.LegacyResetRowCounts
type LegacyRemoteCreateClaim = issueops.LegacyRemoteCreateClaim
type LegacyOrcaTask = issueops.LegacyOrcaTask
type LegacyResetPreview = issueops.LegacyResetPreview
type LegacyResetStatus = issueops.LegacyResetStatus
type LegacyResetResult = issueops.LegacyResetResult
type LegacyResetRemoteReconcileRequest = issueops.LegacyResetRemoteReconcileRequest
type LegacyResetRemoteDependencies = issueops.LegacyResetRemoteDependencies
type LegacyResetRemoteReconcileResult = issueops.LegacyResetRemoteReconcileResult
type LegacyResetOrcaReconcileRequest = issueops.LegacyResetOrcaReconcileRequest
type LegacyResetOrcaDependencies = issueops.LegacyResetOrcaDependencies
type LegacyResetOrcaReconcileResult = issueops.LegacyResetOrcaReconcileResult
type LegacyResetDrainCycleRequest = issueops.LegacyResetDrainCycleRequest
type LegacyResetDrainCycleResult = issueops.LegacyResetDrainCycleResult
type LegacyResetActivationBeginRequest = issueops.LegacyResetActivationBeginRequest
type LegacyResetActivationSealRequest = issueops.LegacyResetActivationSealRequest
type LegacyResetActivationResult = issueops.LegacyResetActivationResult
type ResetRequiredError = issueops.ResetRequiredError

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

func ResolveRecordProvider(record IssueOpsRecord) string {
	return issueops.ResolveRecordProvider(record)
}

func UmbrellaBranchGateReason(record IssueOpsRecord) string {
	return issueops.UmbrellaBranchGateReason(record)
}

func StageIssueOpsArtifact(stateRoot, id, name string, content []byte) (IssueOpsRecord, error) {
	return issueops.StageIssueOpsArtifact(stateRoot, id, name, content)
}

func StagedIssueOpsArtifactNames(stateRoot, id string) ([]string, error) {
	return issueops.StagedIssueOpsArtifactNames(stateRoot, id)
}

func UnstageIssueOpsArtifact(stateRoot, id, name string) (IssueOpsRecord, error) {
	return issueops.UnstageIssueOpsArtifact(stateRoot, id, name)
}

type IssueOpsImplementationReviewRequest = issueops.IssueOpsImplementationReviewRequest

func RecordIssueOpsImplementationReview(stateRoot, id string, req IssueOpsImplementationReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsImplementationReview(stateRoot, id, req)
}

type IssueOpsListResult = issueops.IssueOpsListResult
type IssueOpsListEntry = issueops.IssueOpsListEntry

func ListIssueOpsCycles(stateRoot, repo string) (IssueOpsListResult, error) {
	return issueops.ListIssueOpsCycles(stateRoot, repo)
}

func IncrementIssueOpsSourceMisdirect(stateRoot, id string) (int, error) {
	return issueops.IncrementIssueOpsSourceMisdirect(stateRoot, id)
}

type IssueOpsCleanupFinishRequest = issueops.CleanupFinishRequest
type IssueOpsCleanupFinishDeps = issueops.CleanupFinishDeps
type IssueOpsCleanupFinishResult = issueops.CleanupFinishResult

func IssueOpsCleanupFinish(ctx context.Context, stateRoot string, req IssueOpsCleanupFinishRequest, deps IssueOpsCleanupFinishDeps) (IssueOpsCleanupFinishResult, error) {
	return issueops.CleanupFinish(ctx, stateRoot, req, deps)
}

type ExecutionOrcaProvisioner = issueops.ExecutionOrcaProvisioner
type ExecutionOrcaOwnerInspector = issueops.ExecutionOrcaOwnerInspector
type IssueOpsCleanupAbandonRequest = issueops.CleanupAbandonRequest
type IssueOpsCleanupAbandonDeps = issueops.CleanupAbandonDeps
type IssueOpsCleanupAbandonResult = issueops.CleanupAbandonResult

// IssueOpsCleanupAbandon은 폐기된 비-done 사이클의 로컬 레코드 수명을 종료한다.
// 원격 무접촉 경로이므로 IssueProvider를 받지 않는다(#106).
func IssueOpsCleanupAbandon(ctx context.Context, stateRoot string, req IssueOpsCleanupAbandonRequest, deps IssueOpsCleanupAbandonDeps) (IssueOpsCleanupAbandonResult, error) {
	return issueops.CleanupAbandon(ctx, stateRoot, req, deps)
}

type IssueOpsCleanupRemoteBranchRequest = issueops.CleanupRemoteBranchRequest
type IssueOpsCleanupRemoteBranchDeps = issueops.CleanupRemoteBranchDeps
type IssueOpsCleanupRemoteBranchResult = issueops.CleanupRemoteBranchResult
type IssueOpsCleanupRemoteBranchArtifactHead = issueops.CleanupRemoteBranchArtifactHead

// IssueOpsCleanupRemoteBranch는 머지 검증된 사이클의 원격 브랜치를 typed
// 경로로 삭제한다(#116). 원격 삭제는 git 직접 호출이고 provider는 감사 라인
// 반영에만 쓰인다.
func IssueOpsCleanupRemoteBranch(ctx context.Context, stateRoot string, req IssueOpsCleanupRemoteBranchRequest, deps IssueOpsCleanupRemoteBranchDeps) (IssueOpsCleanupRemoteBranchResult, error) {
	return issueops.CleanupRemoteBranch(ctx, stateRoot, req, deps)
}

func ReflectIssueOpsCleanupAudit(stateRoot string, record IssueOpsRecord, completion IssueProviderCompletionSection, audit string, prov IssueProvider) error {
	return issueops.ReflectCleanupAudit(stateRoot, record, completion, audit, prov)
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

type IssueOpsPruneResult = issueops.IssueOpsPruneResult

func PruneIssueOps(stateRoot string, maxAge time.Duration, confirm bool) (IssueOpsPruneResult, error) {
	return issueops.PruneIssueOps(stateRoot, maxAge, confirm)
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

func PreviewLegacyReset(stateDir string, targetSchema int) (LegacyResetPreview, error) {
	return issueops.PreviewLegacyReset(stateDir, targetSchema)
}

func StatusLegacyReset(stateDir string, targetSchema int) (LegacyResetStatus, error) {
	return issueops.StatusLegacyReset(stateDir, targetSchema)
}

func BeginLegacyResetActivation(stateDir string, req LegacyResetActivationBeginRequest) (LegacyResetActivationResult, error) {
	return issueops.BeginLegacyResetActivation(stateDir, req)
}

func SealLegacyResetActivation(stateDir string, req LegacyResetActivationSealRequest) (LegacyResetActivationResult, error) {
	return issueops.SealLegacyResetActivation(stateDir, req)
}

func ConfirmLegacyReset(stateDir string, targetSchema int, expectedFingerprint string) (LegacyResetResult, error) {
	return issueops.ConfirmLegacyReset(stateDir, targetSchema, expectedFingerprint)
}

func ConfirmLegacyResetWithOrca(ctx context.Context, stateDir string, targetSchema int, expectedFingerprint string, deps LegacyResetOrcaDependencies) (LegacyResetResult, error) {
	return issueops.ConfirmLegacyResetWithOrca(ctx, stateDir, targetSchema, expectedFingerprint, deps)
}

func ReconcileLegacyRemoteClaim(ctx context.Context, stateDir string, req LegacyResetRemoteReconcileRequest, deps LegacyResetRemoteDependencies) (LegacyResetRemoteReconcileResult, error) {
	return issueops.ReconcileLegacyRemoteClaim(ctx, stateDir, req, deps)
}

func ReconcileLegacyOrcaTask(ctx context.Context, stateDir string, req LegacyResetOrcaReconcileRequest, deps LegacyResetOrcaDependencies) (LegacyResetOrcaReconcileResult, error) {
	return issueops.ReconcileLegacyOrcaTask(ctx, stateDir, req, deps)
}

func DrainLegacyCycle(stateDir string, req LegacyResetDrainCycleRequest) (LegacyResetDrainCycleResult, error) {
	return issueops.DrainLegacyCycle(stateDir, req)
}

func DrainLegacyCycleWithOrca(ctx context.Context, stateDir string, req LegacyResetDrainCycleRequest, deps LegacyResetOrcaDependencies) (LegacyResetDrainCycleResult, error) {
	return issueops.DrainLegacyCycleWithOrca(ctx, stateDir, req, deps)
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

func VerifyIssueOpsRemoteArtifactWithActor(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return issueops.VerifyIssueOpsRemoteArtifactWithActor(stateRoot, id, req, actor)
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return issueops.ValidateIssueOpsRemoteArtifactVerification(stateRoot, id, req)
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsPRReadiness(record)
}

func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithLoopGate(issueops.IssueOpsStrictPRReadiness(record), record.Repo)
}

func IssueOpsStrictPRReadinessWithState(stateRoot string, record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithLoopGate(issueops.IssueOpsStrictPRReadinessWithState(stateRoot, record), record.Repo)
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

func IssueOpsLastActiveAt(record IssueOpsRecord) string {
	return issueops.LastActiveAt(record)
}
