package worker

import (
	"context"
	"fmt"
	workercontract "issueops/internal/contract/worker"
	"os"
	"strings"
	"time"

	"issueops/internal/domain/policy"
)

func EnqueueWorkerJob(kind, payload string) (workercontract.WorkerJob, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return workercontract.WorkerJob{OK: false}, fmt.Errorf("worker job kind is required")
	}
	if strings.ContainsAny(kind, `/\`) || len(kind) > 80 {
		return workercontract.WorkerJob{OK: false}, fmt.Errorf("invalid worker job kind")
	}
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return workercontract.WorkerJob{OK: false}, err
	}
	now := time.Now().UTC()
	id := makeWorkerJobID(kind, payload, now)
	job := workercontract.WorkerJob{
		OK:           true,
		ID:           id,
		Kind:         kind,
		Status:       workercontract.WorkerStatusQueued,
		Payload:      policy.RedactFreeform(payload),
		CreatedAt:    now.Format(time.RFC3339Nano),
		UpdatedAt:    now.Format(time.RFC3339Nano),
		WorkerDir:    dir,
		NoShell:      true,
		SafetyNotice: "worker MVP records lifecycle state only; it never executes shell commands",
	}
	// W3: serialize the enqueue write under the same per-job lock the other
	// writers use (Enqueue was the lone unlocked writeWorkerJob caller). Enqueue
	// holds no other lock and RunReadOnlyWorkerJob calls it before taking its own,
	// so this does not nest.
	return job, withWorkerJobLock(context.Background(), dir, id, func(context.Context) error { return writeWorkerJob(job) })
}

func CancelWorkerJob(id string) (workercontract.WorkerJob, error) {
	dir, err := workerDir()
	if err != nil {
		return workercontract.WorkerJob{OK: false, ID: id}, err
	}
	var job workercontract.WorkerJob
	err = withWorkerJobLock(context.Background(), dir, id, func(context.Context) error {
		current, reReadErr := ReadWorkerJob(id)
		if reReadErr != nil {
			job = current
			return reReadErr
		}
		job = current
		if current.Status == workercontract.WorkerStatusCancelled {
			return nil
		}
		if current.Status != workercontract.WorkerStatusQueued {
			return fmt.Errorf("worker job %s cannot be cancelled from status %s", id, current.Status)
		}
		current.Status = workercontract.WorkerStatusCancelled
		current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		current.OK = true
		job = current
		return writeWorkerJob(current)
	})
	if err != nil {
		return job, err
	}
	return job, nil
}
