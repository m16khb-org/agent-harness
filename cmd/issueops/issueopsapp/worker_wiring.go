package issueopsapp

import (
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/statuscli"
	"issueops/cmd/issueops/workercli"
	"issueops/internal/adapter/worker"
)

// configureWorkerJobs는 worker job 저장소와 실행기를 설치한다.
//
// job을 어디에 쌓고 어떻게 실행할지는 하나의 구현이고, 그 선택은 composition
// root의 결정이다. CLI와 MCP transport는 job 형식만 안다.
func configureWorkerJobs() {
	workercli.EnqueueWorkerJob = worker.EnqueueWorkerJob
	workercli.CancelWorkerJob = worker.CancelWorkerJob
	workercli.ReadWorkerJob = worker.ReadWorkerJob
	workercli.ListWorkerJobs = worker.ListWorkerJobs
	workercli.DetectStuckWorkerJobs = worker.DetectStuckWorkerJobs
	workercli.RunReadOnlyWorkerJob = worker.RunReadOnlyWorkerJob

	mcpcli.EnqueueWorkerJob = worker.EnqueueWorkerJob
	mcpcli.CancelWorkerJob = worker.CancelWorkerJob
	mcpcli.ReadWorkerJob = worker.ReadWorkerJob
	mcpcli.ListWorkerJobs = worker.ListWorkerJobs
	mcpcli.RunReadOnlyWorkerJob = worker.RunReadOnlyWorkerJob

	statuscli.ListWorkerJobs = worker.ListWorkerJobs
}
