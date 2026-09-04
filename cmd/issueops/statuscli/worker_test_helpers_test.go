package statuscli

import (
	"issueops/internal/adapter/worker"
)

// production wiring과 같은 worker store를 설치한다. fitness graph는 test import를
// 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	ListWorkerJobs = worker.ListWorkerJobs
}
