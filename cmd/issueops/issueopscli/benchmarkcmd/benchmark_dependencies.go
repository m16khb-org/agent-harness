package benchmarkcmd

import (
	"errors"
	issueopscontract "issueops/internal/contract/issueops"
)

// 벤치마크 실행·저장·판정은 파일시스템과 외부 판정기를 다루는 I/O다. CLI는 그
// 구현을 모르고 composition root가 주입한 함수만 호출한다.
var benchmarkDeps = BenchmarkDeps{
	CompareIssueOpsBenchmarkRuns: func(baseline, candidate issueopscontract.IssueOpsBenchmarkRunResult) issueopscontract.IssueOpsBenchmarkCompareResult {
		return issueopscontract.IssueOpsBenchmarkCompareResult{}
	},
	ComputeReliability: func(issueopscontract.RecordedOutcomes, float64) (issueopscontract.ReliabilityReport, error) {
		return issueopscontract.ReliabilityReport{}, errNotConfigured
	},
	ConsensusJudgeVerdict: func([]issueopscontract.JudgeSample) (issueopscontract.ConsensusVerdict, error) {
		return issueopscontract.ConsensusVerdict{}, errNotConfigured
	},
	DecodeIssueOpsBenchmarkJudgeJSON: func([]byte) (issueopscontract.IssueOpsBenchmarkScore, error) {
		return issueopscontract.IssueOpsBenchmarkScore{}, errNotConfigured
	},
	EvaluateIssueOpsAutoresearchGate: func(issueopscontract.IssueOpsAutoresearchGateRequest) issueopscontract.IssueOpsAutoresearchGateResult {
		return issueopscontract.IssueOpsAutoresearchGateResult{}
	},
	FinalizeIssueOpsBenchmarkRunResult: func(result issueopscontract.IssueOpsBenchmarkRunResult) issueopscontract.IssueOpsBenchmarkRunResult {
		return result
	},
	LoadIssueOpsBenchmarkFixtures: func(string) ([]issueopscontract.IssueOpsBenchmarkFixture, error) {
		return nil, errNotConfigured
	},
	MergeIssueOpsBenchmarkScoreWithJudge: func(deterministic, judge issueopscontract.IssueOpsBenchmarkScore) issueopscontract.IssueOpsBenchmarkScore {
		return deterministic
	},
	ReadIssueOpsBenchmarkRun: func(string, string) (issueopscontract.IssueOpsBenchmarkRunResult, error) {
		return issueopscontract.IssueOpsBenchmarkRunResult{}, errNotConfigured
	},
	RunIssueOpsBenchmark: func(issueopscontract.IssueOpsBenchmarkRunRequest) (issueopscontract.IssueOpsBenchmarkRunResult, error) {
		return issueopscontract.IssueOpsBenchmarkRunResult{}, errNotConfigured
	},
	SaveIssueOpsBenchmarkRun: func(string, issueopscontract.IssueOpsBenchmarkRunResult) error { return errNotConfigured },
	ValidateJudgeProvenance: func(issueopscontract.IssueOpsJudgeMap, string, string) error {
		return errNotConfigured
	},
}

var errNotConfigured = errors.New("issueops benchmark is not configured")

// BenchmarkDeps는 composition root가 실제 구현을 꽂는 진입점이다.
type BenchmarkDeps struct {
	CompareIssueOpsBenchmarkRuns         func(baseline, candidate issueopscontract.IssueOpsBenchmarkRunResult) issueopscontract.IssueOpsBenchmarkCompareResult
	ComputeReliability                   func(issueopscontract.RecordedOutcomes, float64) (issueopscontract.ReliabilityReport, error)
	ConsensusJudgeVerdict                func([]issueopscontract.JudgeSample) (issueopscontract.ConsensusVerdict, error)
	DecodeIssueOpsBenchmarkJudgeJSON     func([]byte) (issueopscontract.IssueOpsBenchmarkScore, error)
	EvaluateIssueOpsAutoresearchGate     func(issueopscontract.IssueOpsAutoresearchGateRequest) issueopscontract.IssueOpsAutoresearchGateResult
	FinalizeIssueOpsBenchmarkRunResult   func(issueopscontract.IssueOpsBenchmarkRunResult) issueopscontract.IssueOpsBenchmarkRunResult
	LoadIssueOpsBenchmarkFixtures        func(string) ([]issueopscontract.IssueOpsBenchmarkFixture, error)
	MergeIssueOpsBenchmarkScoreWithJudge func(deterministic, judge issueopscontract.IssueOpsBenchmarkScore) issueopscontract.IssueOpsBenchmarkScore
	ReadIssueOpsBenchmarkRun             func(stateRoot, id string) (issueopscontract.IssueOpsBenchmarkRunResult, error)
	RunIssueOpsBenchmark                 func(issueopscontract.IssueOpsBenchmarkRunRequest) (issueopscontract.IssueOpsBenchmarkRunResult, error)
	SaveIssueOpsBenchmarkRun             func(stateRoot string, result issueopscontract.IssueOpsBenchmarkRunResult) error
	ValidateJudgeProvenance              func(judge issueopscontract.IssueOpsJudgeMap, scoredRunID, stateRoot string) error
}

// ConfigureBenchmark는 composition root가 실제 어댑터를 꽂는 진입점이다.
func ConfigureBenchmark(deps BenchmarkDeps) {
	if deps.CompareIssueOpsBenchmarkRuns != nil {
		benchmarkDeps.CompareIssueOpsBenchmarkRuns = deps.CompareIssueOpsBenchmarkRuns
	}
	if deps.ComputeReliability != nil {
		benchmarkDeps.ComputeReliability = deps.ComputeReliability
	}
	if deps.ConsensusJudgeVerdict != nil {
		benchmarkDeps.ConsensusJudgeVerdict = deps.ConsensusJudgeVerdict
	}
	if deps.DecodeIssueOpsBenchmarkJudgeJSON != nil {
		benchmarkDeps.DecodeIssueOpsBenchmarkJudgeJSON = deps.DecodeIssueOpsBenchmarkJudgeJSON
	}
	if deps.EvaluateIssueOpsAutoresearchGate != nil {
		benchmarkDeps.EvaluateIssueOpsAutoresearchGate = deps.EvaluateIssueOpsAutoresearchGate
	}
	if deps.FinalizeIssueOpsBenchmarkRunResult != nil {
		benchmarkDeps.FinalizeIssueOpsBenchmarkRunResult = deps.FinalizeIssueOpsBenchmarkRunResult
	}
	if deps.LoadIssueOpsBenchmarkFixtures != nil {
		benchmarkDeps.LoadIssueOpsBenchmarkFixtures = deps.LoadIssueOpsBenchmarkFixtures
	}
	if deps.MergeIssueOpsBenchmarkScoreWithJudge != nil {
		benchmarkDeps.MergeIssueOpsBenchmarkScoreWithJudge = deps.MergeIssueOpsBenchmarkScoreWithJudge
	}
	if deps.ReadIssueOpsBenchmarkRun != nil {
		benchmarkDeps.ReadIssueOpsBenchmarkRun = deps.ReadIssueOpsBenchmarkRun
	}
	if deps.RunIssueOpsBenchmark != nil {
		benchmarkDeps.RunIssueOpsBenchmark = deps.RunIssueOpsBenchmark
	}
	if deps.SaveIssueOpsBenchmarkRun != nil {
		benchmarkDeps.SaveIssueOpsBenchmarkRun = deps.SaveIssueOpsBenchmarkRun
	}
	if deps.ValidateJudgeProvenance != nil {
		benchmarkDeps.ValidateJudgeProvenance = deps.ValidateJudgeProvenance
	}
}
