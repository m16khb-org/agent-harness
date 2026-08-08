package benchmark

import issueopscontract "agent-harness/internal/contract/issueops"

// 벤치마크 DTO는 계약이 소유한다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	IssueOpsDimensionScore          = issueopscontract.IssueOpsDimensionScore
	IssueOpsBenchmarkScore          = issueopscontract.IssueOpsBenchmarkScore
	IssueOpsBenchmarkRunRequest     = issueopscontract.IssueOpsBenchmarkRunRequest
	IssueOpsBenchmarkRunResult      = issueopscontract.IssueOpsBenchmarkRunResult
	IssueOpsBenchmarkCompareResult  = issueopscontract.IssueOpsBenchmarkCompareResult
	IssueOpsAutoresearchCandidate   = issueopscontract.IssueOpsAutoresearchCandidate
	IssueOpsAutoresearchGateRequest = issueopscontract.IssueOpsAutoresearchGateRequest
	IssueOpsAutoresearchGateResult  = issueopscontract.IssueOpsAutoresearchGateResult
	IssueOpsLLMJudgeRequest         = issueopscontract.IssueOpsLLMJudgeRequest
	IssueOpsJudgeMap                = issueopscontract.IssueOpsJudgeMap
	RecordedRun                     = issueopscontract.RecordedRun
	RecordedOutcomes                = issueopscontract.RecordedOutcomes
	FixtureReliability              = issueopscontract.FixtureReliability
	PassPowKPoint                   = issueopscontract.PassPowKPoint
	ReliabilityReport               = issueopscontract.ReliabilityReport
	RoutingFidelityResult           = issueopscontract.RoutingFidelityResult
	JudgeSample                     = issueopscontract.JudgeSample
	ConsensusVerdict                = issueopscontract.ConsensusVerdict
)
