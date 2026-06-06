package core

import (
	"agent-harness/internal/core/policy"
	coreworker "agent-harness/internal/core/worker"
)

const (
	WorkerStatusQueued    = coreworker.WorkerStatusQueued
	WorkerStatusRunning   = coreworker.WorkerStatusRunning
	WorkerStatusSucceeded = coreworker.WorkerStatusSucceeded
	WorkerStatusFailed    = coreworker.WorkerStatusFailed
	WorkerStatusCancelled = coreworker.WorkerStatusCancelled
)

type WorkerJob = coreworker.WorkerJob
type WorkerListResult = coreworker.WorkerListResult

func EnqueueWorkerJob(kind, payload string) (WorkerJob, error) {
	return coreworker.EnqueueWorkerJob(kind, payload)
}

func CancelWorkerJob(id string) (WorkerJob, error) {
	return coreworker.CancelWorkerJob(id)
}

func ReadWorkerJob(id string) (WorkerJob, error) {
	return coreworker.ReadWorkerJob(id)
}

func ListWorkerJobs() (WorkerListResult, error) {
	return coreworker.ListWorkerJobs()
}

func RunReadOnlyWorkerJob(kind, payload string, req CommandPolicyRequest) (WorkerJob, error) {
	return coreworker.RunReadOnlyWorkerJob(kind, payload, policy.CommandPolicyRequest(req))
}
