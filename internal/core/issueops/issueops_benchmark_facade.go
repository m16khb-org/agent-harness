package issueops

import "agent-harness/internal/core/issueops/benchmark"

type IssueOpsBenchmarkFixture = benchmark.IssueOpsBenchmarkFixture
type IssueOpsBenchmarkArtifact = benchmark.IssueOpsBenchmarkArtifact
type IssueOpsDimensionScore = benchmark.IssueOpsDimensionScore
type IssueOpsBenchmarkScore = benchmark.IssueOpsBenchmarkScore
type IssueOpsBenchmarkRunRequest = benchmark.IssueOpsBenchmarkRunRequest
type IssueOpsBenchmarkRunResult = benchmark.IssueOpsBenchmarkRunResult
type IssueOpsBenchmarkCompareResult = benchmark.IssueOpsBenchmarkCompareResult
type IssueOpsAutoresearchCandidate = benchmark.IssueOpsAutoresearchCandidate
type IssueOpsAutoresearchGateRequest = benchmark.IssueOpsAutoresearchGateRequest
type IssueOpsAutoresearchGateResult = benchmark.IssueOpsAutoresearchGateResult
type IssueOpsAgyJudgeRequest = benchmark.IssueOpsAgyJudgeRequest

func LoadIssueOpsBenchmarkFixtures(dir string) ([]IssueOpsBenchmarkFixture, error) {
	return benchmark.LoadIssueOpsBenchmarkFixtures(dir)
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

func RunIssueOpsAgyJudge(req IssueOpsAgyJudgeRequest) (IssueOpsBenchmarkScore, error) {
	return benchmark.RunIssueOpsAgyJudge(req)
}

func BuildIssueOpsAgyJudgePrompt(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) (string, error) {
	return benchmark.BuildIssueOpsAgyJudgePrompt(fixture, artifact)
}
