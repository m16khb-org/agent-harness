package issueops

import "agent-harness/internal/core/issueops/benchmark"

type IssueOpsBenchmarkFixture = benchmark.IssueOpsBenchmarkFixture
type IssueOpsBenchmarkArtifact = benchmark.IssueOpsBenchmarkArtifact
type SkillRouting = benchmark.SkillRouting
type IssueOpsDimensionScore = benchmark.IssueOpsDimensionScore
type IssueOpsBenchmarkScore = benchmark.IssueOpsBenchmarkScore
type IssueOpsBenchmarkRunRequest = benchmark.IssueOpsBenchmarkRunRequest
type IssueOpsBenchmarkRunResult = benchmark.IssueOpsBenchmarkRunResult
type IssueOpsBenchmarkCompareResult = benchmark.IssueOpsBenchmarkCompareResult
type IssueOpsAutoresearchCandidate = benchmark.IssueOpsAutoresearchCandidate
type IssueOpsAutoresearchGateRequest = benchmark.IssueOpsAutoresearchGateRequest
type IssueOpsAutoresearchGateResult = benchmark.IssueOpsAutoresearchGateResult
type IssueOpsLLMJudgeRequest = benchmark.IssueOpsLLMJudgeRequest
type RecordedRun = benchmark.RecordedRun
type RecordedOutcomes = benchmark.RecordedOutcomes
type FixtureReliability = benchmark.FixtureReliability
type PassPowKPoint = benchmark.PassPowKPoint
type ReliabilityReport = benchmark.ReliabilityReport
type IssueOpsJudgeMap = benchmark.IssueOpsJudgeMap
type JudgeSample = benchmark.JudgeSample
type ConsensusVerdict = benchmark.ConsensusVerdict

func LoadIssueOpsBenchmarkFixtures(dir string) ([]IssueOpsBenchmarkFixture, error) {
	return benchmark.LoadIssueOpsBenchmarkFixtures(dir)
}

func ComputeReliability(rec RecordedOutcomes, alpha float64) (ReliabilityReport, error) {
	return benchmark.ComputeReliability(rec, alpha)
}

func ScoreSpread(scores []float64) (float64, float64, float64) {
	return benchmark.ScoreSpread(scores)
}

func ValidateJudgeProvenance(judge IssueOpsJudgeMap, scoredRunID, stateRoot string) error {
	return benchmark.ValidateJudgeProvenance(judge, scoredRunID, stateRoot)
}

func JudgeDownwardOverrideRate(deterministic, judge IssueOpsBenchmarkScore) (float64, int) {
	return benchmark.JudgeDownwardOverrideRate(deterministic, judge)
}

func ConsensusJudgeVerdict(samples []JudgeSample) (ConsensusVerdict, error) {
	return benchmark.ConsensusJudgeVerdict(samples)
}

func ScoreIssueOpsBenchmarkArtifact(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) IssueOpsBenchmarkScore {
	return benchmark.ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
}

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	return benchmark.RunIssueOpsBenchmark(req)
}

func SaveIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	return benchmark.SaveIssueOpsBenchmarkRun(stateRoot, result)
}

func ReadIssueOpsBenchmarkRun(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
	return benchmark.ReadIssueOpsBenchmarkRun(stateRoot, id)
}

func FinalizeIssueOpsBenchmarkRunResult(result IssueOpsBenchmarkRunResult) IssueOpsBenchmarkRunResult {
	return benchmark.FinalizeIssueOpsBenchmarkRunResult(result)
}

func MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge IssueOpsBenchmarkScore) IssueOpsBenchmarkScore {
	return benchmark.MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge)
}

func CompareIssueOpsBenchmarkRuns(baseline, candidate IssueOpsBenchmarkRunResult) IssueOpsBenchmarkCompareResult {
	return benchmark.CompareIssueOpsBenchmarkRuns(baseline, candidate)
}

func EvaluateIssueOpsAutoresearchGate(req IssueOpsAutoresearchGateRequest) IssueOpsAutoresearchGateResult {
	return benchmark.EvaluateIssueOpsAutoresearchGate(req)
}

func RunIssueOpsLLMJudge(req IssueOpsLLMJudgeRequest) (IssueOpsBenchmarkScore, error) {
	return benchmark.RunIssueOpsLLMJudge(req)
}

func DecodeIssueOpsBenchmarkJudgeJSON(out []byte) (IssueOpsBenchmarkScore, error) {
	return benchmark.DecodeIssueOpsBenchmarkJudgeJSON(out)
}

func BuildIssueOpsLLMJudgePrompt(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) (string, error) {
	return benchmark.BuildIssueOpsLLMJudgePrompt(fixture, artifact)
}
