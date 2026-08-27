package worker

import (
	workercontract "agent-harness/internal/contract/worker"
	"fmt"
	"testing"
)

// A2/G6: ListWorkerJobs reports a status histogram with Depth = queued+running
// (the saturation signal) so a backlog is visible before jobs time out, which
// the raw per-status counts alone do not surface.
func TestListWorkerJobsReportsQueueDepth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)

	statuses := []string{
		workercontract.WorkerStatusQueued, workercontract.WorkerStatusQueued,
		workercontract.WorkerStatusRunning,
		workercontract.WorkerStatusSucceeded,
		workercontract.WorkerStatusFailed,
		workercontract.WorkerStatusCancelled,
	}
	for i, s := range statuses {
		job, err := EnqueueWorkerJob("queue-test", fmt.Sprintf("p%d", i))
		if err != nil {
			t.Fatal(err)
		}
		job.Status = s
		if err := writeWorkerJob(job); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ListWorkerJobs()
	if err != nil || result.Queue == nil {
		t.Fatalf("list failed or nil queue: %+v err=%v", result, err)
	}
	q := result.Queue
	if q.Total != 6 || q.Queued != 2 || q.Running != 1 || q.Succeeded != 1 || q.Failed != 1 || q.Cancelled != 1 {
		t.Fatalf("histogram wrong: %+v", q)
	}
	if q.Depth != 3 { // queued(2) + running(1)
		t.Fatalf("depth want 3 (queued+running), got %d", q.Depth)
	}
}
