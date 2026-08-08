package worker

import (
	workercontract "agent-harness/internal/contract/worker"
	"fmt"
	"testing"
	"time"
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

// A2/W1: MaybeDetectStuckWorkerJobs runs the detector once, then skips within
// the interval (stat-only sentinel), amortizing the unbounded scan off the
// session-start hot path while still self-healing crashed jobs.
func TestMaybeDetectStuckWorkerJobsSentinelGates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_WORKER_DIR", dir)

	job, err := EnqueueWorkerJob("stuck", "p")
	if err != nil {
		t.Fatal(err)
	}
	job.Status = workercontract.WorkerStatusRunning
	job.PID = 99999999 // far beyond max pid_t; kill(2) returns ESRCH
	job.UpdatedAt = "2020-01-01T00:00:00Z"
	if err := writeWorkerJob(job); err != nil {
		t.Fatal(err)
	}

	// First run: not gated; detects + fixes the stuck job.
	result, ran, err := MaybeDetectStuckWorkerJobs(time.Hour)
	if err != nil || !ran {
		t.Fatalf("first run should execute: ran=%v err=%v", ran, err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected 1 stuck job fixed, got %+v", result)
	}
	if stored, _ := ReadWorkerJob(job.ID); stored.Status != workercontract.WorkerStatusFailed {
		t.Fatalf("stuck job should be marked failed, got %s", stored.Status)
	}

	// Second run within the interval: gated, skipped.
	if _, ran2, err := MaybeDetectStuckWorkerJobs(time.Hour); err != nil || ran2 {
		t.Fatalf("second run within interval should be skipped: ran=%v err=%v", ran2, err)
	}

	// Zero interval always runs (sentinel age is always >= 0).
	if _, ran3, err := MaybeDetectStuckWorkerJobs(0); err != nil || !ran3 {
		t.Fatalf("zero interval should always run: ran=%v err=%v", ran3, err)
	}
}
