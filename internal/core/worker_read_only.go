package core

import "time"

func RunReadOnlyWorkerJob(kind, payload string, req CommandPolicyRequest) (WorkerJob, error) {
	job, err := EnqueueWorkerJob(kind, payload)
	if err != nil {
		return job, err
	}
	job.Status = WorkerStatusRunning
	job.NoShell = true
	job.Command = append([]string{}, req.Argv...)
	job.SafetyNotice = "worker read-only runner executes only argv commands that pass command policy with write/network/shell disabled"
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeWorkerJob(job); err != nil {
		return job, err
	}
	result := RunReadOnlyCommand(req)
	job.Result = &result
	job.OK = result.OK
	if result.OK {
		job.Status = WorkerStatusSucceeded
	} else {
		job.Status = WorkerStatusFailed
	}
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return job, writeWorkerJob(job)
}
