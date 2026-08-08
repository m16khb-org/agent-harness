package workercli

import (
	policycontract "agent-harness/internal/contract/policy"
	workercontract "agent-harness/internal/contract/worker"
)

// worker job 연산은 composition root가 설치한다. CLI/MCP transport는 job을
// 어디에 어떻게 쌓고 실행하는지 알지 않는다.
var (
	EnqueueWorkerJob      func(kind, payload string) (workercontract.WorkerJob, error)
	CancelWorkerJob       func(id string) (workercontract.WorkerJob, error)
	ReadWorkerJob         func(id string) (workercontract.WorkerJob, error)
	ListWorkerJobs        func() (workercontract.WorkerListResult, error)
	DetectStuckWorkerJobs func() (workercontract.WorkerListResult, error)
	RunReadOnlyWorkerJob  func(kind, payload string, req policycontract.CommandPolicyRequest) (workercontract.WorkerJob, error)
)
