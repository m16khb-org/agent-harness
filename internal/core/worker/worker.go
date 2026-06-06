package worker

import (
	"fmt"
	"os"
	"strings"
	"time"

	"agent-harness/internal/core/policy"
)

const (
	WorkerStatusQueued    = "queued"
	WorkerStatusRunning   = "running"
	WorkerStatusSucceeded = "succeeded"
	WorkerStatusFailed    = "failed"
	WorkerStatusCancelled = "cancelled"
)

type WorkerJob struct {
	OK           bool                     `json:"ok"`
	ID           string                   `json:"id"`
	Kind         string                   `json:"kind"`
	Status       string                   `json:"status"`
	Payload      string                   `json:"payload,omitempty"`
	CreatedAt    string                   `json:"created_at"`
	UpdatedAt    string                   `json:"updated_at"`
	WorkerDir    string                   `json:"worker_dir"`
	NoShell      bool                     `json:"no_shell"`
	SafetyNotice string                   `json:"safety_notice"`
	Command      []string                 `json:"command,omitempty"`
	Result       *policy.CommandRunResult `json:"result,omitempty"`
}

type WorkerListResult struct {
	OK        bool        `json:"ok"`
	WorkerDir string      `json:"worker_dir"`
	Jobs      []WorkerJob `json:"jobs"`
}

func EnqueueWorkerJob(kind, payload string) (WorkerJob, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return WorkerJob{OK: false}, fmt.Errorf("worker job kind is required")
	}
	if strings.ContainsAny(kind, `/\`) || len(kind) > 80 {
		return WorkerJob{OK: false}, fmt.Errorf("invalid worker job kind")
	}
	dir, err := workerDir()
	if err != nil {
		return WorkerJob{OK: false}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return WorkerJob{OK: false}, err
	}
	now := time.Now().UTC()
	id := makeWorkerJobID(kind, payload, now)
	job := WorkerJob{
		OK:           true,
		ID:           id,
		Kind:         kind,
		Status:       WorkerStatusQueued,
		Payload:      policy.RedactFreeform(payload),
		CreatedAt:    now.Format(time.RFC3339Nano),
		UpdatedAt:    now.Format(time.RFC3339Nano),
		WorkerDir:    dir,
		NoShell:      true,
		SafetyNotice: "worker MVP records lifecycle state only; it never executes shell commands",
	}
	return job, writeWorkerJob(job)
}

func CancelWorkerJob(id string) (WorkerJob, error) {
	job, err := ReadWorkerJob(id)
	if err != nil {
		return job, err
	}
	if job.Status == WorkerStatusCancelled {
		return job, nil
	}
	if job.Status != WorkerStatusQueued {
		return job, fmt.Errorf("worker job %s cannot be cancelled from status %s", id, job.Status)
	}
	job.Status = WorkerStatusCancelled
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	job.OK = true
	return job, writeWorkerJob(job)
}
