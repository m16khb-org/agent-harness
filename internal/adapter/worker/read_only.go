package worker

import (
	workercontract "agent-harness/internal/contract/worker"
	"context"
	"fmt"
	"os"
	"time"

	policydomain "agent-harness/internal/contract/policy"
)

func RunReadOnlyWorkerJob(kind, payload string, req policydomain.CommandPolicyRequest) (workercontract.WorkerJob, error) {
	job, err := EnqueueWorkerJob(kind, payload)
	if err != nil {
		return job, err
	}
	dir, err := workerDir()
	if err != nil {
		return job, err
	}

	// Transition to running under lock so concurrent CancelWorkerJob
	// serializes with this state change.
	if err := withWorkerJobLock(context.Background(), dir, job.ID, func(context.Context) error {
		current, reReadErr := ReadWorkerJob(job.ID)
		if reReadErr != nil {
			return reReadErr
		}
		job = current
		if current.Status != workercontract.WorkerStatusQueued {
			return fmt.Errorf("worker job %s cannot run from status %s", job.ID, current.Status)
		}
		current.Status = workercontract.WorkerStatusRunning
		current.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		current.PID = os.Getpid()
		current.NoShell = true
		current.Command = append([]string{}, req.Argv...)
		current.SafetyNotice = "worker read-only runner executes only argv commands that pass command policy with write/network/shell disabled"
		current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		job = current
		return writeWorkerJob(current)
	}); err != nil {
		return job, err
	}

	result := RunReadOnlyCommand(req)

	// Transition to final status under lock.
	if err := withWorkerJobLock(context.Background(), dir, job.ID, func(context.Context) error {
		current, reReadErr := ReadWorkerJob(job.ID)
		if reReadErr != nil {
			return reReadErr
		}
		job = current
		current.Result = &result
		current.OK = result.OK
		if result.OK {
			current.Status = workercontract.WorkerStatusSucceeded
		} else {
			current.Status = workercontract.WorkerStatusFailed
		}
		current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		job = current
		return writeWorkerJob(current)
	}); err != nil {
		return job, err
	}
	return job, nil
}
