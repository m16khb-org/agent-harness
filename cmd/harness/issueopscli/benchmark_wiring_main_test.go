package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	issueopscore "agent-harness/internal/adapter/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 벤치마크 CLI 계약 테스트는 실제 실행
// 경로를 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
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
	wireCleanupForTests()
	os.Exit(m.Run())
}
