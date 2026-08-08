package statuscli

import (
	workercontract "agent-harness/internal/contract/worker"
)

// worker job 연산은 composition root가 설치한다. CLI/MCP transport는 job을
// 어디에 어떻게 쌓고 실행하는지 알지 않는다.
var (
	ListWorkerJobs func() (workercontract.WorkerListResult, error)
)
