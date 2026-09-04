package issueopscli

import (
	"issueops/cmd/issueops/hookcli/hookenv"
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	issueopscore "issueops/internal/adapter/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 벤치마크 CLI 계약 테스트는 실제 실행
// 경로를 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	// remote artifact gate smoke는 hook enforcement가 켜져 있음을 전제한다.
	// dogfood 셸의 ISSUEOPS_DISABLE_HOOKS=1이 새어 들어오면 hook이 no-op이 되어
	// 빈 stdout을 JSON으로 파싱하려다 실패한다(#395).
	hookenv.ClearInheritedOperatorSwitches()
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
	wireRemoteForTests()
	wireOrphanAndLoopGateForTests()
	wireExecutionRunnersForTests()
	wireIssueOpsRuntimeForTests()
	os.Exit(m.Run())
}
