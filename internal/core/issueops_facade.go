package core

import "agent-harness/internal/core/issueops"

type IssueOpsStartRequest = issueops.IssueOpsStartRequest
type IssueOpsFeedbackItem = issueops.IssueOpsFeedbackItem
type IssueOpsIssueLink = issueops.IssueOpsIssueLink
type IssueOpsBranchPrepareStep = issueops.IssueOpsBranchPrepareStep
type IssueOpsBranchPrepare = issueops.IssueOpsBranchPrepare
type IssueOpsBranchPrepareRequest = issueops.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = issueops.IssueOpsRemoteArtifactVerification
type IssueOpsRemoteArtifactVerificationRequest = issueops.IssueOpsRemoteArtifactVerificationRequest
type IssueOpsRecord = issueops.IssueOpsRecord
type IssueOpsReadiness = issueops.IssueOpsReadiness
type IssueOpsCleanupStatusRequest = issueops.IssueOpsCleanupStatusRequest
type IssueOpsCleanupStatus = issueops.IssueOpsCleanupStatus

type IssueOpsPhase = issueops.IssueOpsPhase

const (
	IssueOpsPhaseProblem     = issueops.IssueOpsPhaseProblem
	IssueOpsPhaseGrill       = issueops.IssueOpsPhaseGrill
	IssueOpsPhasePlan        = issueops.IssueOpsPhasePlan
	IssueOpsPhaseImplement   = issueops.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean = issueops.IssueOpsPhaseAISlopClean
	IssueOpsPhaseFeedback    = issueops.IssueOpsPhaseFeedback
	IssueOpsPhasePR          = issueops.IssueOpsPhasePR
	IssueOpsPhaseDone        = issueops.IssueOpsPhaseDone
)

var IssueOpsPhases = issueops.IssueOpsPhases

type IssueOpsBenchmarkFixture = issueops.IssueOpsBenchmarkFixture
type IssueOpsBenchmarkArtifact = issueops.IssueOpsBenchmarkArtifact
type IssueOpsDimensionScore = issueops.IssueOpsDimensionScore
type IssueOpsBenchmarkScore = issueops.IssueOpsBenchmarkScore
type IssueOpsBenchmarkRunRequest = issueops.IssueOpsBenchmarkRunRequest
type IssueOpsBenchmarkRunResult = issueops.IssueOpsBenchmarkRunResult
type IssueOpsBenchmarkCompareResult = issueops.IssueOpsBenchmarkCompareResult
type IssueOpsAutoresearchCandidate = issueops.IssueOpsAutoresearchCandidate
type IssueOpsAutoresearchGateRequest = issueops.IssueOpsAutoresearchGateRequest
type IssueOpsAutoresearchGateResult = issueops.IssueOpsAutoresearchGateResult
type IssueOpsAgyJudgeRequest = issueops.IssueOpsAgyJudgeRequest

type IssueOpsRemoteArtifact = issueops.IssueOpsRemoteArtifact
type IssueOpsRemoteIssueCandidate = issueops.IssueOpsRemoteIssueCandidate
type IssueOpsRemoteLabelCandidate = issueops.IssueOpsRemoteLabelCandidate
type IssueOpsRemoteScoringRequest = issueops.IssueOpsRemoteScoringRequest
type IssueOpsRemoteScoredItem = issueops.IssueOpsRemoteScoredItem
type IssueOpsRemoteScoringResult = issueops.IssueOpsRemoteScoringResult
type IssueOpsRemoteAgyJudgeRequest = issueops.IssueOpsRemoteAgyJudgeRequest

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	return issueops.StartIssueOps(stateRoot, req)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ReadIssueOps(stateRoot, id)
}

func IssueOpsStateRoot() string {
	return issueops.IssueOpsStateRoot()
}

func newIssueOpsID(repo, branch string) string {
	return issueops.NewIssueOpsID(repo, branch)
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

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsChild(stateRoot, id, childURL, title)
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
	return issueops.IssueOpsStrictPRReadiness(record)
}

func IssueOpsImplementationReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueops.IssueOpsImplementationReadiness(record)
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

func RunIssueOpsAgyJudge(req IssueOpsAgyJudgeRequest) (IssueOpsBenchmarkScore, error) {
	return issueops.RunIssueOpsAgyJudge(req)
}

func buildIssueOpsAgyJudgePrompt(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) (string, error) {
	return issueops.BuildIssueOpsAgyJudgePrompt(fixture, artifact)
}

func DecodeIssueOpsRemoteScoringRequest(data []byte) (IssueOpsRemoteScoringRequest, error) {
	return issueops.DecodeIssueOpsRemoteScoringRequest(data)
}

func ScoreIssueOpsRemoteCandidates(req IssueOpsRemoteScoringRequest) (IssueOpsRemoteScoringResult, error) {
	return issueops.ScoreIssueOpsRemoteCandidates(req)
}

func RunIssueOpsRemoteAgyJudge(req IssueOpsRemoteAgyJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	return issueops.RunIssueOpsRemoteAgyJudge(req)
}
