package core

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/artifacttemplate"
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

func IssueOpsChildStatus(stateRoot, parentID string, repair bool) (IssueOpsChildStatusResult, error) {
	return issueops.IssueOpsChildStatus(stateRoot, parentID, repair)
}

func AcceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string) (IssueOpsChildValidationResult, error) {
	return issueops.AcceptIssueOpsChild(stateRoot, parentID, childID, evidence)
}

func RejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string) (IssueOpsChildValidationResult, error) {
	return issueops.RejectIssueOpsChild(stateRoot, parentID, childID, reason, evidence)
}

func DropIssueOpsChild(stateRoot, parentID, childID, reason string) (IssueOpsChildValidationResult, error) {
	return issueops.DropIssueOpsChild(stateRoot, parentID, childID, reason)
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

func RecordIssueOpsPlanPrep(stateRoot, id string, req IssueOpsPlanPrepRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsPlanPrep(stateRoot, id, req)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDesignReview(stateRoot, id, req)
}

func RecordIssueOpsExecutionDecision(stateRoot, id string, req IssueOpsExecutionDecisionRecordRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsExecutionDecision(stateRoot, id, req)
}

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req IssueOpsCompatibilityReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsCompatibilityReview(stateRoot, id, req)
}

func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDevilsAdvocateReview(stateRoot, id, req)
}

func RecordIssueOpsDomainReview(stateRoot, id string, req IssueOpsDomainReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDomainReview(stateRoot, id, req)
}

func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification)
}

func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (IssueOpsRecord, error) {
	return issueops.ResolveIssueOpsFeedback(stateRoot, id, index, resolution)
}

func RegressIssueOpsForReplan(stateRoot, id, reason string) (IssueOpsRecord, error) {
	return issueops.RegressIssueOpsForReplan(stateRoot, id, reason)
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

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsPlan(stateRoot, id, planPath)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsWorktree(stateRoot, id, worktreePath)
}

func RecordIssueOpsWorktreeTools(stateRoot, id string, prep IssueOpsWorktreeToolPreparation) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsWorktreeTools(stateRoot, id, prep)
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsChild(stateRoot, id, childURL, title)
}

func LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title)
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	return issueops.PrepareIssueOpsBranch(stateRoot, id, req)
}

func validateIssueOpsIssueBranch(branch string) error {
	return issueops.ValidateIssueOpsIssueBranch(branch)
}

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsFeedback(stateRoot, id, source, body, classification)
}

func MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id)
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

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return issueops.VerifyIssueOpsRemoteArtifact(stateRoot, id, req)
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	return issueops.ValidateIssueOpsRemoteArtifactVerification(stateRoot, id, req)
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsPRReadiness(record)
}

func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithPoolGate(issueops.IssueOpsStrictPRReadiness(record), record.ID)
}

func IssueOpsStrictPRReadinessWithState(stateRoot string, record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithPoolGate(issueops.IssueOpsStrictPRReadinessWithState(stateRoot, record), record.ID)
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

func ScanStaleIssueOpsCycles(req IssueOpsStaleScanRequest) IssueOpsStaleScanResult {
	return issueops.ScanStaleIssueOpsCycles(req)
}

func ForceDoneIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ForceDoneIssueOps(stateRoot, id)
}

func AddIssueOpsDecision(stateRoot, id string, req IssueOpsDecisionRecordRequest) (IssueOpsRecord, error) {
	return issueops.AddIssueOpsDecision(stateRoot, id, req)
}

type SkillRoutingEntry = issueops.SkillRoutingEntry

// RecordIssueOpsRouting captures a live (phase, skill) routing pairing on the
// cycle record so skill_routing_fidelity can score real activation.
func RecordIssueOpsRouting(stateRoot, id, phase, skill string) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsRouting(stateRoot, id, phase, skill)
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

func IssueOpsLastActiveAt(record IssueOpsRecord) string {
	return issueops.LastActiveAt(record)
}
