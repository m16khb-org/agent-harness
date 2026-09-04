package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	issueopscore "issueops/internal/adapter/issueops"
)

// 벤치마크 CLI는 실행 구현을 알지 않는다. 어댑터를 아는 곳은 composition root
// 하나뿐이다.
func configureIssueOpsBenchmark() {
	benchmarkcmd.ConfigureBenchmark(benchmarkcmd.BenchmarkDeps{
		CompareIssueOpsBenchmarkRuns:         issueopscore.CompareIssueOpsBenchmarkRuns,
		ComputeReliability:                   issueopscore.ComputeReliability,
		ConsensusJudgeVerdict:                issueopscore.ConsensusJudgeVerdict,
		DecodeIssueOpsBenchmarkJudgeJSON:     issueopscore.DecodeIssueOpsBenchmarkJudgeJSON,
		EvaluateIssueOpsAutoresearchGate:     issueopscore.EvaluateIssueOpsAutoresearchGate,
		FinalizeIssueOpsBenchmarkRunResult:   issueopscore.FinalizeIssueOpsBenchmarkRunResult,
		LoadIssueOpsBenchmarkFixtures:        issueopscore.LoadIssueOpsBenchmarkFixtures,
		MergeIssueOpsBenchmarkScoreWithJudge: issueopscore.MergeIssueOpsBenchmarkScoreWithJudge,
		ReadIssueOpsBenchmarkRun:             issueopscore.ReadIssueOpsBenchmarkRun,
		RunIssueOpsBenchmark:                 issueopscore.RunIssueOpsBenchmark,
		SaveIssueOpsBenchmarkRun:             issueopscore.SaveIssueOpsBenchmarkRun,
		ValidateJudgeProvenance:              issueopscore.ValidateJudgeProvenance,
	})
}
